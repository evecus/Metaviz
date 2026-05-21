package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/metaviz/internal/builder"
	"github.com/metaviz/internal/config"
	"github.com/metaviz/internal/cronrestart"
	"github.com/metaviz/internal/firewall"
	"github.com/metaviz/internal/ipfilter"
	"github.com/metaviz/internal/node"
	"github.com/metaviz/internal/profile"
	"github.com/metaviz/internal/storage"
	"github.com/metaviz/internal/subscription"
	"github.com/metaviz/internal/sysproxy"
)

const mihomoBin = "/usr/bin/mihomo"
const metavizGroup = "metaviz"
const kindNode = "node"

const (
	settingState        = "state"
	settingIPFilter     = "ipfilter"
	settingProxySettings = "proxy_settings"
	settingMetaSettings = "meta_settings"
)

type State string

const (
	StateStopped State = "stopped"
	StateRunning State = "running"
	StateError   State = "error"
)

type StartParams struct {
	BlockAds       bool              `json:"blockAds"`
	RouteMode      builder.RouteMode `json:"routeMode"`
	NodeID         string            `json:"nodeId"`
	ConfigMode     string            `json:"configMode"` // "upload"|"node"|"subnode"|"subscription"
	SubscriptionID string            `json:"subscriptionId"`
	SubNodeIdx     int               `json:"subNodeIdx"`
	UploadedConfigFile string        `json:"uploadedConfigFile"`
}

type savedState struct {
	Params  StartParams `json:"params"`
	Running bool        `json:"running"`
}

type ProxySettings struct {
	SystemProxy bool           `json:"systemProxy"`
	TCPMode     config.TCPMode `json:"tcpMode"`
	UDPMode     config.UDPMode `json:"udpMode"`
	LanProxy    bool           `json:"lanProxy"`
	IPv6        bool           `json:"ipv6"`
	BypassCN    bool           `json:"bypassCN"`
	// ExtraGIDs is a list of additional GIDs whose traffic is bypassed by the
	// firewall (not intercepted by mihomo). Empty means disabled.
	ExtraGIDs []uint32 `json:"extraGIDs"`
}

func (ps ProxySettings) toProxyModes() config.ProxyModes {
	tcp := ps.TCPMode
	if tcp == "" {
		tcp = config.TCPModeRedir
	}
	udp := ps.UDPMode
	if udp == "" {
		udp = config.UDPModeTProxy
	}
	return config.ProxyModes{TCP: tcp, UDP: udp}
}

func (ps ProxySettings) wantsSystemProxy() bool { return ps.SystemProxy }

type Manager struct {
	mu         sync.Mutex
	dataDir    string
	runDir     string
	mrsDir     string
	configsDir string
	providersDir string

	cmd    *exec.Cmd
	state  State
	errMsg string
	params StartParams
	ports  Ports

	activeProxySettings ProxySettings

	db              *storage.DB
	stateStore      *storage.Store
	ipfilterStore   *storage.Store
	proxyStore      *storage.Store
	metaStore       *storage.Store
	subManager      *subscription.Manager
	profileManager  *profile.Manager

	logMu   sync.RWMutex
	logBuf  []string
	logSubs []chan string

	schedStop chan struct{}
}

type Ports struct {
	DNS      int `json:"dns"`
	Mixed    int `json:"mixed"`
	Redirect int `json:"redirect"`
	TProxy   int `json:"tproxy"`
}

func NewManager(dataDir, runDir, mrsDir string) *Manager {
	configsDir := filepath.Join(dataDir, "configs")
	providersDir := filepath.Join(dataDir, "providers")
	for _, d := range []string{configsDir, providersDir} {
		_ = os.MkdirAll(d, 0755)
	}

	dbPath := filepath.Join(dataDir, "metaviz.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("metaviz: open database: %v", err)
	}

	m := &Manager{
		dataDir:      dataDir,
		runDir:       runDir,
		mrsDir:       mrsDir,
		configsDir:   configsDir,
		providersDir: providersDir,
		state:        StateStopped,
		logBuf:       make([]string, 0, 500),
		db:           db,
		stateStore:   storage.NewStore(db, settingState),
		ipfilterStore: storage.NewStore(db, settingIPFilter),
		proxyStore:   storage.NewStore(db, settingProxySettings),
		metaStore:    storage.NewStore(db, settingMetaSettings),
		subManager:   subscription.NewManager(db),
		profileManager: profile.NewManager(db),
	}

	var ss savedState
	if err := m.stateStore.Load(&ss); err == nil {
		m.params = ss.Params
	}
	return m
}

func (m *Manager) RunConfigPath() string { return filepath.Join(m.runDir, "config.yaml") }
func (m *Manager) ConfigsDir() string    { return m.configsDir }
func (m *Manager) ProvidersDir() string  { return m.providersDir }
func (m *Manager) UploadedConfigPath(filename string) string {
	return filepath.Join(m.configsDir, filename)
}

func (m *Manager) AutoStart() {
	var ss savedState
	if err := m.stateStore.Load(&ss); err != nil || !ss.Running {
		return
	}
	log.Printf("metaviz: last state was running, auto-starting mihomo")
	if err := m.Start(ss.Params); err != nil {
		log.Printf("metaviz: auto-start failed: %v", err)
	}
}

func (m *Manager) saveState(running bool) {
	ss := savedState{Params: m.params, Running: running}
	if err := m.stateStore.Save(&ss); err != nil {
		log.Printf("warn: save state: %v", err)
	}
}

// ── Node management ────────────────────────────────────────────────────────

func (m *Manager) loadNodesFromDB() []*node.Node {
	entities, err := m.db.ListEntities(kindNode)
	if err != nil {
		return []*node.Node{}
	}
	out := make([]*node.Node, 0, len(entities))
	for _, e := range entities {
		var n node.Node
		if err := json.Unmarshal([]byte(e.Data), &n); err != nil {
			continue
		}
		out = append(out, &n)
	}
	return out
}

func (m *Manager) GetNodes() []*node.Node {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadNodesFromDB()
}

func (m *Manager) AddNodes(ns []*node.Node) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range ns {
		if err := m.db.UpsertEntity(kindNode, n.ID, n); err != nil {
			log.Printf("warn: add node %s: %v", n.ID, err)
		}
	}
}

func (m *Manager) DeleteNode(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.db.GetEntity(kindNode, id)
	if err != nil || e == nil {
		return false
	}
	_ = m.db.DeleteEntity(kindNode, id)
	return true
}

func (m *Manager) findNode(id string) *node.Node {
	e, err := m.db.GetEntity(kindNode, id)
	if err != nil || e == nil {
		return nil
	}
	var n node.Node
	if err := json.Unmarshal([]byte(e.Data), &n); err != nil {
		return nil
	}
	return &n
}

// ── Group helpers ──────────────────────────────────────────────────────────

func ensureMetavizGroup() (uint32, error) {
	if g, err := user.LookupGroup(metavizGroup); err == nil {
		gid, err := strconv.ParseUint(g.Gid, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("parse gid %q: %w", g.Gid, err)
		}
		return uint32(gid), nil
	}
	log.Printf("group %q not found, creating", metavizGroup)
	if path, err := exec.LookPath("groupadd"); err == nil {
		out, err := exec.Command(path, "--system", metavizGroup).CombinedOutput()
		if err != nil {
			return 0, fmt.Errorf("groupadd: %w (output: %s)", err, strings.TrimSpace(string(out)))
		}
		g, err := user.LookupGroup(metavizGroup)
		if err != nil {
			return 0, fmt.Errorf("lookup group after create: %w", err)
		}
		gid, err := strconv.ParseUint(g.Gid, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("parse gid: %w", err)
		}
		return uint32(gid), nil
	}
	return writeGroupEntry(metavizGroup)
}

func writeGroupEntry(name string) (uint32, error) {
	const groupFile = "/etc/group"
	data, err := os.ReadFile(groupFile)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", groupFile, err)
	}
	usedGIDs := make(map[uint32]bool)
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		if gid, err := strconv.ParseUint(parts[2], 10, 32); err == nil {
			usedGIDs[uint32(gid)] = true
		}
	}
	var chosen uint32
	for candidate := uint32(500); candidate < 65000; candidate++ {
		if !usedGIDs[candidate] {
			chosen = candidate
			break
		}
	}
	if chosen == 0 {
		return 0, fmt.Errorf("no free GID available")
	}
	entry := fmt.Sprintf("%s:x:%d:\n", name, chosen)
	f, err := os.OpenFile(groupFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", groupFile, err)
	}
	defer f.Close()
	if _, err := f.WriteString(entry); err != nil {
		return 0, fmt.Errorf("write %s: %w", groupFile, err)
	}
	return chosen, nil
}

// ── Start / Stop ───────────────────────────────────────────────────────────

func (m *Manager) Start(p StartParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateRunning {
		return fmt.Errorf("already running")
	}

	ps := m.loadProxySettings()
	modes := ps.toProxyModes()
	ms := m.loadMetaSettings()

	ports := Ports{
		DNS:      ms.Inbound.DNSPort,
		Mixed:    ms.Inbound.MixedPort,
		Redirect: ms.Inbound.RedirectPort,
		TProxy:   ms.Inbound.TProxyPort,
	}
	m.ports = ports

	global := metaSettingsToGlobal(ms, ps)

	switch p.ConfigMode {
	case "upload":
		if err := m.prepareUploadConfig(p, global); err != nil {
			return err
		}
	case "node":
		n := m.findNode(p.NodeID)
		if n == nil {
			return fmt.Errorf("node %q not found", p.NodeID)
		}
		data, err := builder.BuildNodeConfig(p.RouteMode, n, m.mrsDir, p.BlockAds, global)
		if err != nil {
			return fmt.Errorf("build config: %w", err)
		}
		if err := os.WriteFile(m.RunConfigPath(), data, 0644); err != nil {
			return err
		}
	case "subnode":
		if p.SubscriptionID == "" {
			return fmt.Errorf("subscriptionId is required")
		}
		proxies, err := m.subManager.GetProxies(p.SubscriptionID)
		if err != nil {
			return fmt.Errorf("subscription cache: %w", err)
		}
		if p.SubNodeIdx < 0 || p.SubNodeIdx >= len(proxies) {
			return fmt.Errorf("subNodeIdx %d out of range", p.SubNodeIdx)
		}
		raw := proxies[p.SubNodeIdx]
		n, err := node.FromMap(raw)
		if err != nil {
			return fmt.Errorf("parse subscription node: %w", err)
		}
		data, err := builder.BuildSubNodeConfig(p.RouteMode, n, m.mrsDir, p.BlockAds, global)
		if err != nil {
			return fmt.Errorf("build config: %w", err)
		}
		if err := os.WriteFile(m.RunConfigPath(), data, 0644); err != nil {
			return err
		}
	case "subscription":
		if p.SubscriptionID == "" {
			return fmt.Errorf("subscriptionId is required")
		}
		sub := m.subManager.GetByID(p.SubscriptionID)
		if sub == nil {
			return fmt.Errorf("subscription %q not found", p.SubscriptionID)
		}
		data, err := builder.BuildSubscriptionConfig(
			p.RouteMode, sub.ID, sub.Name, sub.URL, m.mrsDir, p.BlockAds, global,
		)
		if err != nil {
			return fmt.Errorf("build config: %w", err)
		}
		if err := os.WriteFile(m.RunConfigPath(), data, 0644); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown configMode %q", p.ConfigMode)
	}

	gid, err := ensureMetavizGroup()
	if err != nil {
		return fmt.Errorf("metaviz group: %w", err)
	}

	var ipf ipfilter.Config
	_ = m.ipfilterStore.Load(&ipf)

	fwPorts := firewall.Ports{
		DNS:      ports.DNS,
		TProxy:   ports.TProxy,
		Redirect: ports.Redirect,
	}
	tunDevice := ms.Tun.Device
	if tunDevice == "" {
		tunDevice = "Meta"
	}
	if err := firewall.Apply(modes, fwPorts, ps.LanProxy, ps.IPv6, ps.BypassCN, tunDevice, m.dataDir, gid, ps.ExtraGIDs, ipf, ps.SystemProxy, ms.Inbound.FakeIP); err != nil {
		return fmt.Errorf("firewall: %w", err)
	}

	// mihomo -d <runDir> (config.yaml must be in runDir)
	cmd := exec.Command(mihomoBin, "-d", m.runDir)
	cmd.Dir = m.runDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         0,
			Gid:         gid,
			Groups:      []uint32{gid},
			NoSetGroups: false,
		},
	}
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		firewall.Stop()
		return fmt.Errorf("start mihomo: %w", err)
	}

	if modes.NeedsTunInbound() {
		go func() {
			dev := tunDevice
			for i := 0; i < 20; i++ {
				time.Sleep(500 * time.Millisecond)
				if _, err := os.Stat("/sys/class/net/" + dev); err == nil {
					firewall.ApplyTunRoutes(ps.IPv6)
					return
				}
			}
			log.Printf("warn: tun device %s did not appear within 10s", dev)
		}()
	}

	m.cmd = cmd
	m.state = StateRunning
	m.errMsg = ""
	m.params = p
	m.activeProxySettings = ps
	m.saveState(true)
	m.startScheduler()

	if ps.wantsSystemProxy() {
		if err := sysproxy.Set(ports.Mixed); err != nil {
			m.appendLog("warn: set system proxy: " + err.Error())
		} else {
			m.appendLog(fmt.Sprintf("system proxy set: http/https -> 127.0.0.1:%d", ports.Mixed))
		}
	}

	go m.streamLog(stdout)
	go m.streamLog(stderr)
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		defer m.mu.Unlock()
		firewall.Stop()
		if m.activeProxySettings.wantsSystemProxy() {
			if err := sysproxy.Clear(); err != nil {
				log.Printf("warn: clear system proxy: %v", err)
			}
		}
		if err != nil {
			m.errMsg = err.Error()
			m.state = StateError
			m.appendLog("mihomo exited: " + err.Error())
		} else {
			m.state = StateStopped
			m.appendLog("mihomo stopped")
		}
		m.saveState(false)
		m.cmd = nil
	}()

	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	proc := m.cmd
	m.mu.Unlock()

	if proc != nil && proc.Process != nil {
		_ = proc.Process.Kill()
		done := make(chan struct{})
		go func() { _ = proc.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	firewall.Stop()
	if m.activeProxySettings.wantsSystemProxy() {
		if err := sysproxy.Clear(); err != nil {
			log.Printf("warn: clear system proxy: %v", err)
		}
	}
	m.state = StateStopped
	m.cmd = nil
	m.saveState(false)
	m.stopScheduler()
}

func cleanRunDir(dir string) {
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// ── Upload config ──────────────────────────────────────────────────────────

func (m *Manager) prepareUploadConfig(p StartParams, global builder.GlobalConfig) error {
	filename := p.UploadedConfigFile
	if filename == "" {
		filename = "config.yaml"
	}
	srcPath := m.UploadedConfigPath(filename)
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("uploaded config %q not found", filename)
	}
	patched, err := builder.PatchUploadConfig(src, global)
	if err != nil {
		return fmt.Errorf("patch config: %w", err)
	}
	return os.WriteFile(m.RunConfigPath(), patched, 0644)
}

// ── Settings ───────────────────────────────────────────────────────────────

func (m *Manager) loadMetaSettings() MetaSettings {
	var ms MetaSettings
	_ = m.metaStore.Load(&ms)
	return ms.Filled()
}

func (m *Manager) GetMetaSettings() MetaSettings { return m.loadMetaSettings() }

func (m *Manager) SaveMetaSettings(ms MetaSettings) error {
	return m.metaStore.Save(&ms)
}

func (m *Manager) loadProxySettings() ProxySettings {
	var ps ProxySettings
	_ = m.proxyStore.Load(&ps)
	if ps.TCPMode == "" {
		ps.TCPMode = config.TCPModeRedir
	}
	if ps.UDPMode == "" {
		ps.UDPMode = config.UDPModeTProxy
	}
	return ps
}

func (m *Manager) GetProxySettings() ProxySettings  { return m.loadProxySettings() }
func (m *Manager) SaveProxySettings(ps ProxySettings) error { return m.proxyStore.Save(&ps) }

func (m *Manager) GetIPFilter() ipfilter.Config {
	var cfg ipfilter.Config
	_ = m.ipfilterStore.Load(&cfg)
	if cfg.Mode == "" {
		cfg.Mode = ipfilter.ModeOff
	}
	return cfg
}

func (m *Manager) SaveIPFilter(cfg ipfilter.Config) error { return m.ipfilterStore.Save(&cfg) }

func (m *Manager) GetSubManager() *subscription.Manager    { return m.subManager }
func (m *Manager) GetProfileManager() *profile.Manager     { return m.profileManager }

// ── Status ──────────────────────────────────────────────────────────────────

func readProcessRSS(pid int) int64 {
	if pid == 0 {
		return 0
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var kb int64
				fmt.Sscan(fields[1], &kb)
				return kb
			}
		}
	}
	return 0
}

func (m *Manager) Status() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	pid := 0
	if m.cmd != nil && m.cmd.Process != nil {
		pid = m.cmd.Process.Pid
	}
	ps := m.activeProxySettings
	return map[string]interface{}{
		"state":      m.state,
		"configMode": m.params.ConfigMode,
		"tcpMode":    ps.TCPMode,
		"udpMode":    ps.UDPMode,
		"lanProxy":   ps.LanProxy,
		"ipv6":       ps.IPv6,
		"routeMode":  m.params.RouteMode,
		"blockAds":   m.params.BlockAds,
		"nodeId":     m.params.NodeID,
		"pid":        pid,
		"rssKB":      readProcessRSS(pid),
		"ports":      m.ports,
		"error":      m.errMsg,
	}
}

// ── Logging ────────────────────────────────────────────────────────────────

func (m *Manager) RecentLogs(n int) []string {
	m.logMu.RLock()
	defer m.logMu.RUnlock()
	if n > len(m.logBuf) {
		n = len(m.logBuf)
	}
	out := make([]string, n)
	copy(out, m.logBuf[len(m.logBuf)-n:])
	return out
}

func (m *Manager) SubscribeLogs() chan string {
	ch := make(chan string, 128)
	m.logMu.Lock()
	m.logSubs = append(m.logSubs, ch)
	m.logMu.Unlock()
	return ch
}

func (m *Manager) UnsubscribeLogs(ch chan string) {
	m.logMu.Lock()
	defer m.logMu.Unlock()
	subs := m.logSubs[:0]
	for _, s := range m.logSubs {
		if s != ch {
			subs = append(subs, s)
		}
	}
	m.logSubs = subs
	close(ch)
}

func (m *Manager) streamLog(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[mihomo] %s", line)
		m.appendLog(line)
	}
}

func (m *Manager) appendLog(line string) {
	m.logMu.Lock()
	defer m.logMu.Unlock()
	if len(m.logBuf) >= 500 {
		m.logBuf = m.logBuf[1:]
	}
	m.logBuf = append(m.logBuf, line)
	for _, ch := range m.logSubs {
		select {
		case ch <- line:
		default:
		}
	}
}

// ── Scheduler ──────────────────────────────────────────────────────────────

func (m *Manager) startScheduler() {
	m.stopScheduler()
	ms := m.loadMetaSettings()
	if !ms.ScheduledRestart.Enabled || ms.ScheduledRestart.Cron == "" {
		return
	}
	entry, err := cronrestart.Parse(ms.ScheduledRestart.Cron)
	if err != nil {
		log.Printf("metaviz: invalid cron %q: %v", ms.ScheduledRestart.Cron, err)
		return
	}
	stop := make(chan struct{})
	m.schedStop = stop
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		lastFired := time.Time{}
		for {
			select {
			case <-stop:
				return
			case t := <-ticker.C:
				rounded := t.Truncate(time.Minute)
				if entry.Matches(rounded) && rounded.After(lastFired) {
					lastFired = rounded
					m.mu.Lock()
					running := m.state == StateRunning
					params := m.params
					m.mu.Unlock()
					if running {
						log.Printf("metaviz: scheduled restart triggered")
						m.Stop()
						if err := m.Start(params); err != nil {
							log.Printf("metaviz: scheduled restart failed: %v", err)
						}
					}
				}
			}
		}
	}()
}

func (m *Manager) stopScheduler() {
	if m.schedStop != nil {
		close(m.schedStop)
		m.schedStop = nil
	}
}

func (m *Manager) RestartSchedulerIfNeeded() {
	m.mu.Lock()
	running := m.state == StateRunning
	m.mu.Unlock()
	if running {
		m.startScheduler()
	} else {
		m.stopScheduler()
	}
}

func (m *Manager) RecoverState() {
	var ss savedState
	if err := m.stateStore.Load(&ss); err != nil || !ss.Running {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd == nil {
		log.Printf("metaviz: stale running=true, correcting to stopped")
		ss.Running = false
		_ = m.stateStore.Save(&ss)
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func metaSettingsToGlobal(ms MetaSettings, ps ProxySettings) builder.GlobalConfig {
	modes := ps.toProxyModes()
	return builder.GlobalConfig{
		MixedPort:       ms.Inbound.MixedPort,
		RedirectPort:    ms.Inbound.RedirectPort,
		TProxyPort:      ms.Inbound.TProxyPort,
		DNSPort:         ms.Inbound.DNSPort,
		AllowLan:        ps.LanProxy,
		IPv6:            ps.IPv6,
		LogLevel:        ms.Log.Level,
		TunEnable:                  !ps.SystemProxy && (ms.Tun.Enable || modes.NeedsTunInbound()),
		TunDevice:                  ms.Tun.Device,
		TunStack:                   ms.Tun.Stack,
		TunMTU:                     ms.Tun.MTU,
		SnifferEnable:              ms.Sniffer.Enable,
		SnifferOverrideDestination: ms.Sniffer.OverrideDestination,
		ClashAPIListen:  ms.ClashAPI.Listen,
		ClashAPISecret:  ms.ClashAPI.Secret,
		ClashAPIUI:      ms.ClashAPI.UI,
		FindProcessMode: ms.Misc.FindProcessMode,
		UnifiedDelay:    ms.Misc.UnifiedDelay,
		TCPConcurrent:   ms.Misc.TCPConcurrent,
		// FakeIP 仅对单节点/订阅模式的生成配置生效；上传配置模式由用户自行控制 DNS。
		// 但防火墙规则在所有模式下均按此开关处理 fakeip 段的路由。
		FakeIP: ms.Inbound.FakeIP,
	}
}
