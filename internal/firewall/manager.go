package firewall

import (
	"fmt"
	"log"
	"sync"

	"github.com/metaviz/internal/config"
	"github.com/metaviz/internal/ipfilter"
)

var mu sync.Mutex

// activeTunDevice remembers the tun interface name used in the last Apply().
var activeTunDevice string

// activeModes remembers which proxy modes were active so cleanup only tears
// down the routes that were actually installed.
var activeModes config.ProxyModes

// activeIPv6 remembers whether IPv6 routes were installed.
var activeIPv6 bool

// activeFakeIP remembers whether fakeip mode was active.
var activeFakeIP bool

// Ports holds the listen ports that nftables needs to know about.
type Ports struct {
	DNS      int
	TProxy   int
	Redirect int
}

// Apply sets up nftables rules for the chosen TCP/UDP proxy modes.
// fakeIP controls whether fakeip-specific nft rules are generated:
//   - proxy_rule chain 中提前 accept fakeip 段（198.18.0.0/16, fc00::/18）
//   - NAT 链中添加 ICMP ping 劫持，避免 ping fakeip 地址超时
//
// 上传配置模式下也应传入 fakeIP=true（若用户配置了 fakeip），
// 防火墙规则与配置模式无关，只要 fakeip 池地址需要被代理就需要开启。
func Apply(modes config.ProxyModes, ports Ports, lanProxy bool, ipv6 bool, bypassCN bool, tunDevice string, dataDir string, gid uint32, extraGIDs []uint32, ipf ipfilter.Config, systemProxy bool, fakeIP bool) error {
	mu.Lock()
	defer mu.Unlock()

	SetNftConfPath(dataDir)
	cleanup(activeModes, activeIPv6, activeTunDevice)

	if tunDevice == "" {
		tunDevice = "metaviz"
	}
	activeTunDevice = tunDevice
	activeModes = modes
	activeIPv6 = ipv6
	activeFakeIP = fakeIP

	if systemProxy {
		log.Println("firewall: system_proxy enabled — skipping nftables rules and routes")
		return nil
	}

	if modes.IsSystemProxyOnly() {
		log.Println("firewall: system_proxy only — no nftables rules")
		return nil
	}

	if err := setup(modes, ports, lanProxy, ipv6, bypassCN, tunDevice, gid, extraGIDs, ipf, fakeIP); err != nil {
		return fmt.Errorf("nft setup: %w", err)
	}

	SyncLocalIPs()
	return nil
}

// ApplyTunRoutes re-adds the ip rule/route entries for TUN mode.
func ApplyTunRoutes(ipv6 bool) {
	mu.Lock()
	defer mu.Unlock()
	if activeTunDevice == "" {
		return
	}
	setupTunRoutes(ipv6, activeTunDevice, activeFakeIP)
}

// Stop tears down nftables rules and ip routes for the last active modes.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	cleanup(activeModes, activeIPv6, activeTunDevice)
	activeTunDevice = ""
	activeModes = config.ProxyModes{}
	activeIPv6 = false
	activeFakeIP = false
}
