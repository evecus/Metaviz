package api

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/metaviz/internal/auth"
	"github.com/metaviz/internal/builder"
	"github.com/metaviz/internal/config"
	"github.com/metaviz/internal/core"
	"github.com/metaviz/internal/ipfilter"
	"github.com/metaviz/internal/mihomo"
	"github.com/metaviz/internal/node"
	"github.com/metaviz/internal/updater"
)

var errorOnlyFormatter gin.LogFormatter = func(param gin.LogFormatterParams) string {
	if param.StatusCode < 400 {
		return ""
	}
	return fmt.Sprintf("[GIN] %s | %d | %s | %s | %s %s\n",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		param.StatusCode, param.Latency, param.ClientIP,
		param.Method, param.Path,
	)
}

type Server struct {
	manager       *core.Manager
	dataDir       string
	mrsDir        string
	webFS         embed.FS
	sessionMu     sync.RWMutex
	sessionTokens map[string]bool
}

func NewServer(m *core.Manager, dataDir, mrsDir string, webFS embed.FS) *Server {
	return &Server{
		manager:       m,
		dataDir:       dataDir,
		mrsDir:        mrsDir,
		webFS:         webFS,
		sessionTokens: map[string]bool{},
	}
}

func (s *Server) Run(addr string) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{Formatter: errorOnlyFormatter}), gin.Recovery(), cors.Default())

	a := r.Group("/api")
	{
		a.POST("/auth/login", s.authLogin)
		a.POST("/auth/logout", s.authLogout)
		a.GET("/auth/status", s.authStatus)
		a.POST("/auth/setup", s.authSetup)

		p := a.Group("", s.authMiddleware)
		{
			// Config upload (yaml)
			p.POST("/config", s.uploadConfig)
			p.GET("/config/list", s.listUploadedConfigs)
			p.DELETE("/config/:filename", s.deleteUploadedConfig)
			p.GET("/config/raw/:filename", s.rawUploadedConfig)
			p.PUT("/config/raw/:filename", s.updateUploadedConfig)

			// Nodes
			p.GET("/nodes", s.listNodes)
			p.POST("/nodes/import", s.importNodes)
			p.DELETE("/nodes/:id", s.deleteNode)

			// Start / Stop / Status / Logs
			p.POST("/start", s.start)
			p.POST("/stop", s.stop)
			p.GET("/status", s.status)
			p.GET("/logs", s.streamLogs)

			// Subscriptions
			p.GET("/subscriptions", s.listSubscriptions)
			p.POST("/subscriptions", s.addSubscription)
			p.DELETE("/subscriptions/:id", s.deleteSubscription)
			p.PATCH("/subscriptions/:id", s.updateSubscriptionMeta)
			p.POST("/subscriptions/:id/update", s.updateSubscription)
			p.GET("/subscriptions/:id/proxies", s.getSubscriptionProxies)
			p.DELETE("/subscriptions/:id/proxies/:idx", s.deleteSubscriptionProxy)

			// Settings
			p.GET("/mihomo/version", s.mihomoVersion)
			p.POST("/mihomo/install", s.mihomoInstall)
			p.GET("/system-info", s.systemInfo)
			p.GET("/ip-filter", s.getIPFilter)
			p.POST("/ip-filter", s.saveIPFilter)
			p.GET("/proxy-settings", s.getProxySettings)
			p.POST("/proxy-settings", s.saveProxySettings)
			p.GET("/meta-settings", s.getMetaSettings)
			p.POST("/meta-settings", s.saveMetaSettingsWithAuth)

			// Rulesets (mrs)
			p.GET("/rulesets", s.listRulesets)
			p.DELETE("/rulesets/:file", s.deleteRuleset)
			p.POST("/update-rules", s.updateRules)
		}
	}

	dist, err := fs.Sub(s.webFS, "web/dist")
	if err != nil {
		return fmt.Errorf("embed web/dist: %w", err)
	}
	r.NoRoute(func(c *gin.Context) { serveDistFile(c, dist, c.Request.URL.Path) })
	return r.Run(addr)
}

func serveDistFile(c *gin.Context, dist fs.FS, path string) {
	p := strings.TrimPrefix(path, "/")
	f, err := dist.Open(p)
	if err == nil {
		defer f.Close()
		fi, _ := f.Stat()
		if !fi.IsDir() {
			if strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			switch {
			case strings.HasSuffix(p, ".js"):
				c.Header("Content-Type", "application/javascript; charset=utf-8")
			case strings.HasSuffix(p, ".css"):
				c.Header("Content-Type", "text/css; charset=utf-8")
			case strings.HasSuffix(p, ".svg"):
				c.Header("Content-Type", "image/svg+xml")
			case strings.HasSuffix(p, ".png"):
				c.Header("Content-Type", "image/png")
			case strings.HasSuffix(p, ".ico"):
				c.Header("Content-Type", "image/x-icon")
			case strings.HasSuffix(p, ".webmanifest"):
				c.Header("Content-Type", "application/manifest+json")
			}
			http.ServeContent(c.Writer, c.Request, fi.Name(), fi.ModTime(), f.(io.ReadSeeker))
			return
		}
	}
	if strings.HasPrefix(p, "assets/") {
		c.Status(404)
		return
	}
	idx, err := dist.Open("index.html")
	if err != nil {
		c.Status(404)
		return
	}
	defer idx.Close()
	fi, _ := idx.Stat()
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(c.Writer, c.Request, "index.html", fi.ModTime(), idx.(io.ReadSeeker))
}

// ── Config upload (YAML) ────────────────────────────────────────────────────

func resolveUploadFilename(configsDir, name string) string {
	if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
		name = name + ".yaml"
	}
	dst := filepath.Join(configsDir, name)
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return name
	}
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
	for i := 1; i <= 9999; i++ {
		candidate := fmt.Sprintf("%s_%d.yaml", base, i)
		if _, err := os.Stat(filepath.Join(configsDir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
	return name
}

func (s *Server) uploadConfig(c *gin.Context) {
	file, err := c.FormFile("config")
	if err != nil {
		c.JSON(400, gin.H{"error": "no config file"})
		return
	}
	origName := filepath.Base(file.Filename)
	if !strings.HasSuffix(origName, ".yaml") && !strings.HasSuffix(origName, ".yml") {
		c.JSON(400, gin.H{"error": "only .yaml / .yml files are allowed"})
		return
	}
	configsDir := s.manager.ConfigsDir()
	savedName := resolveUploadFilename(configsDir, origName)
	dst := filepath.Join(configsDir, savedName)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		os.Remove(dst)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if err := builder.ValidateYAML(data); err != nil {
		os.Remove(dst)
		c.JSON(400, gin.H{"error": "invalid YAML: " + err.Error()})
		return
	}
	inbounds := builder.SummarizeInbounds(data)
	c.JSON(200, gin.H{"ok": true, "filename": savedName, "inbounds": inbounds})
}

func (s *Server) listUploadedConfigs(c *gin.Context) {
	type entry struct {
		Filename  string                   `json:"filename"`
		Size      int64                    `json:"size"`
		UpdatedAt time.Time                `json:"updatedAt"`
		Inbounds  []map[string]interface{} `json:"inbounds"`
	}
	configsDir := s.manager.ConfigsDir()
	entries, err := os.ReadDir(configsDir)
	if err != nil {
		c.JSON(200, []entry{})
		return
	}
	var items []entry
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		fi, err := de.Info()
		if err != nil {
			continue
		}
		var inbounds []map[string]interface{}
		if data, err := os.ReadFile(filepath.Join(configsDir, name)); err == nil {
			inbounds = builder.SummarizeInbounds(data)
		}
		items = append(items, entry{Filename: name, Size: fi.Size(), UpdatedAt: fi.ModTime(), Inbounds: inbounds})
	}
	if items == nil {
		items = []entry{}
	}
	c.JSON(200, items)
}

func (s *Server) deleteUploadedConfig(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" || filename[0] == '.' || strings.ContainsAny(filename, "/\\") {
		c.JSON(400, gin.H{"error": "invalid filename"})
		return
	}
	if err := os.Remove(filepath.Join(s.manager.ConfigsDir(), filename)); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (s *Server) rawUploadedConfig(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" || filename[0] == '.' || strings.ContainsAny(filename, "/\\") {
		c.JSON(400, gin.H{"error": "invalid filename"})
		return
	}
	data, err := os.ReadFile(filepath.Join(s.manager.ConfigsDir(), filename))
	if err != nil {
		c.JSON(404, gin.H{"error": "config not found"})
		return
	}
	c.Data(200, "text/plain; charset=utf-8", data)
}

func (s *Server) updateUploadedConfig(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" || filename[0] == '.' || strings.ContainsAny(filename, "/\\") {
		c.JSON(400, gin.H{"error": "invalid filename"})
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "read body: " + err.Error()})
		return
	}
	if err := builder.ValidateYAML(body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(filepath.Join(s.manager.ConfigsDir(), filename), body, 0644); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ── Nodes ───────────────────────────────────────────────────────────────────

func (s *Server) listNodes(c *gin.Context) { c.JSON(200, s.manager.GetNodes()) }

func (s *Server) importNodes(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Text == "" {
		c.JSON(400, gin.H{"error": "missing text"})
		return
	}
	nodes, errs := node.ParseLinks(req.Text)
	if len(nodes) > 0 {
		s.manager.AddNodes(nodes)
	}
	c.JSON(200, gin.H{"imported": len(nodes), "errors": errs, "nodes": nodes})
}

func (s *Server) deleteNode(c *gin.Context) {
	if !s.manager.DeleteNode(c.Param("id")) {
		c.JSON(404, gin.H{"error": "node not found"})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ── Start / Stop ────────────────────────────────────────────────────────────

func (s *Server) start(c *gin.Context) {
	var p core.StartParams
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	switch p.ConfigMode {
	case "upload", "node", "subnode", "subscription":
	default:
		c.JSON(400, gin.H{"error": fmt.Sprintf("unknown configMode %q", p.ConfigMode)})
		return
	}
	if p.ConfigMode == "node" || p.ConfigMode == "subnode" || p.ConfigMode == "subscription" {
		switch p.RouteMode {
		case builder.RouteModeWhitelist, builder.RouteModeGFWList, builder.RouteModeGlobal:
		default:
			c.JSON(400, gin.H{"error": fmt.Sprintf("unknown routeMode %q", p.RouteMode)})
			return
		}
	}
	if err := s.manager.Start(p); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (s *Server) stop(c *gin.Context)   { s.manager.Stop(); c.JSON(200, gin.H{"ok": true}) }
func (s *Server) status(c *gin.Context) { c.JSON(200, s.manager.Status()) }

// ── SSE Logs ────────────────────────────────────────────────────────────────

func (s *Server) streamLogs(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(500)
		return
	}
	for _, line := range s.manager.RecentLogs(100) {
		fmt.Fprintf(c.Writer, "data: %s\n\n", sseEscape(line))
	}
	flusher.Flush()
	ch := s.manager.SubscribeLogs()
	defer s.manager.UnsubscribeLogs(ch)
	notify := c.Request.Context().Done()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", sseEscape(line))
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

func sseEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// ── Update rules (mrs) ──────────────────────────────────────────────────────

func (s *Server) updateRules(c *gin.Context) {
	var req struct {
		Proxy string `json:"proxy"`
	}
	_ = c.ShouldBindJSON(&req)
	results := updater.UpdateAll(s.mrsDir, req.Proxy)
	failed := 0
	for _, r := range results {
		if r.Error != "" {
			failed++
		}
	}
	status := http.StatusOK
	if failed == len(results) {
		status = http.StatusBadGateway
	}
	c.JSON(status, gin.H{"results": results, "failed": failed, "total": len(results)})
}

// ── Rulesets list / delete ──────────────────────────────────────────────────

func (s *Server) listRulesets(c *gin.Context) {
	type entry struct {
		File      string    `json:"file"`
		Size      int64     `json:"size"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	var items []entry
	dirEntries, err := os.ReadDir(s.mrsDir)
	if err != nil {
		c.JSON(200, []entry{})
		return
	}
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".mrs") {
			continue
		}
		fi, err := de.Info()
		if err != nil {
			continue
		}
		items = append(items, entry{File: de.Name(), Size: fi.Size(), UpdatedAt: fi.ModTime()})
	}
	if items == nil {
		items = []entry{}
	}
	c.JSON(200, items)
}

func (s *Server) deleteRuleset(c *gin.Context) {
	name := c.Param("file")
	if name == "" || name[0] == '.' || strings.Contains(name, "/") {
		c.JSON(400, gin.H{"error": "invalid file name"})
		return
	}
	if err := os.Remove(filepath.Join(s.mrsDir, name)); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ── Mihomo install / version ─────────────────────────────────────────────────

func (s *Server) mihomoVersion(c *gin.Context) {
	ver := mihomo.Version()
	sys := mihomo.DetectSystem()
	c.JSON(200, gin.H{"version": ver, "arch": sys.Arch, "libc": sys.LibC, "osName": sys.OSName})
}

func (s *Server) mihomoInstall(c *gin.Context) {
	var req struct {
		Proxy   string `json:"proxy"`
		Version string `json:"version"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Version == "" {
		req.Version = "latest"
	}
	ver, err := mihomo.Install(req.Proxy, req.Version)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "version": ver})
}

func (s *Server) systemInfo(c *gin.Context) { c.JSON(200, mihomo.DetectSystem()) }

// ── IP Filter ───────────────────────────────────────────────────────────────

func (s *Server) getIPFilter(c *gin.Context) { c.JSON(200, s.manager.GetIPFilter()) }

func (s *Server) saveIPFilter(c *gin.Context) {
	var cfg ipfilter.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	switch cfg.Mode {
	case ipfilter.ModeOff, ipfilter.ModeBlacklist, ipfilter.ModeWhitelist:
	default:
		c.JSON(400, gin.H{"error": "invalid mode"})
		return
	}
	if err := s.manager.SaveIPFilter(cfg); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ── Proxy Settings ───────────────────────────────────────────────────────────

func (s *Server) getProxySettings(c *gin.Context) { c.JSON(200, s.manager.GetProxySettings()) }

func (s *Server) saveProxySettings(c *gin.Context) {
	var ps core.ProxySettings
	if err := c.ShouldBindJSON(&ps); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	switch ps.TCPMode {
	case config.TCPModeOff, config.TCPModeRedir, config.TCPModeTProxy, config.TCPModeTun:
	default:
		c.JSON(400, gin.H{"error": "invalid tcpMode"})
		return
	}
	switch ps.UDPMode {
	case config.UDPModeOff, config.UDPModeTProxy, config.UDPModeTun:
	default:
		c.JSON(400, gin.H{"error": "invalid udpMode"})
		return
	}
	if err := s.manager.SaveProxySettings(ps); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ── Meta Settings ────────────────────────────────────────────────────────────

func (s *Server) getMetaSettings(c *gin.Context) { c.JSON(200, s.manager.GetMetaSettings()) }

func (s *Server) saveMetaSettingsWithAuth(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	current := s.manager.GetMetaSettings()
	data, _ := json.Marshal(raw)
	var ms core.MetaSettings
	_ = json.Unmarshal(data, &ms)

	// Handle password hash
	if authMap, ok := raw["auth"].(map[string]interface{}); ok {
		if pw, ok := authMap["newPassword"].(string); ok && pw != "" {
			hash, err := auth.HashPassword(pw)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			ms.Auth.PasswordHash = hash
		} else {
			ms.Auth.PasswordHash = current.Auth.PasswordHash
		}
	} else {
		ms.Auth.PasswordHash = current.Auth.PasswordHash
	}

	if err := s.manager.SaveMetaSettings(ms); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	s.manager.RestartSchedulerIfNeeded()
	c.JSON(200, gin.H{"ok": true})
}

// ── Subscriptions ────────────────────────────────────────────────────────────

func (s *Server) listSubscriptions(c *gin.Context) {
	c.JSON(200, s.manager.GetSubManager().List())
}

func (s *Server) addSubscription(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.URL == "" {
		c.JSON(400, gin.H{"error": "url is required"})
		return
	}
	if req.Name == "" {
		req.Name = req.URL
	}
	sub, err := s.manager.GetSubManager().Add(req.Name, req.URL, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, sub)
}

func (s *Server) deleteSubscription(c *gin.Context) {
	if err := s.manager.GetSubManager().Delete(c.Param("id")); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (s *Server) updateSubscription(c *gin.Context) {
	sub, err := s.manager.GetSubManager().Update(c.Param("id"))
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error(), "sub": sub})
		return
	}
	c.JSON(200, sub)
}

func (s *Server) getSubscriptionProxies(c *gin.Context) {
	proxies, err := s.manager.GetSubManager().GetProxies(c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, proxies)
}

func (s *Server) deleteSubscriptionProxy(c *gin.Context) {
	idx, err := strconv.Atoi(c.Param("idx"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid index"})
		return
	}
	if err := s.manager.GetSubManager().DeleteProxy(c.Param("id"), idx); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (s *Server) updateSubscriptionMeta(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	sub, err := s.manager.GetSubManager().UpdateMeta(c.Param("id"), req.Name, req.URL, nil)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, sub)
}

// ── Auth ─────────────────────────────────────────────────────────────────────

func (s *Server) authMiddleware(c *gin.Context) {
	ms := s.manager.GetMetaSettings()
	if !ms.Auth.Enabled {
		c.Next()
		return
	}
	token := c.GetHeader("X-Auth-Token")
	if token == "" {
		token, _ = c.Cookie("metaviz_token")
	}
	s.sessionMu.RLock()
	ok := s.sessionTokens[token]
	s.sessionMu.RUnlock()
	if !ok {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
		return
	}
	c.Next()
}

func (s *Server) authStatus(c *gin.Context) {
	ms := s.manager.GetMetaSettings()
	needsSetup := ms.Auth.Enabled && ms.Auth.PasswordHash == ""
	c.JSON(200, gin.H{"enabled": ms.Auth.Enabled, "needsSetup": needsSetup})
}

func (s *Server) authSetup(c *gin.Context) {
	ms := s.manager.GetMetaSettings()
	if ms.Auth.PasswordHash != "" {
		c.JSON(400, gin.H{"error": "account already configured"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		c.JSON(400, gin.H{"error": "username and password required"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	ms.Auth.Username = req.Username
	ms.Auth.PasswordHash = hash
	ms.Auth.Enabled = true
	if err := s.manager.SaveMetaSettings(ms); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	token := auth.GenerateToken()
	s.sessionMu.Lock()
	s.sessionTokens[token] = true
	s.sessionMu.Unlock()
	c.JSON(200, gin.H{"ok": true, "token": token})
}

func (s *Server) authLogin(c *gin.Context) {
	ms := s.manager.GetMetaSettings()
	if !ms.Auth.Enabled {
		c.JSON(200, gin.H{"ok": true, "token": "noauth"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Username != ms.Auth.Username || !auth.CheckPassword(ms.Auth.PasswordHash, req.Password) {
		c.JSON(401, gin.H{"error": "invalid username or password"})
		return
	}
	token := auth.GenerateToken()
	s.sessionMu.Lock()
	s.sessionTokens[token] = true
	s.sessionMu.Unlock()
	c.JSON(200, gin.H{"ok": true, "token": token})
}

func (s *Server) authLogout(c *gin.Context) {
	token := c.GetHeader("X-Auth-Token")
	if token == "" {
		token, _ = c.Cookie("metaviz_token")
	}
	s.sessionMu.Lock()
	delete(s.sessionTokens, token)
	s.sessionMu.Unlock()
	c.JSON(200, gin.H{"ok": true})
}
