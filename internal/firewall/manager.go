package firewall

import (
	"fmt"
	"log"
	"sync"

	"github.com/metaviz/internal/config"
	"github.com/metaviz/internal/ipfilter"
)

var mu sync.Mutex

// activeTunDevice remembers the tun interface name used in the last Apply()
// so that Stop() / cleanup() can remove the correct ip route/rule entries.
var activeTunDevice string

// activeModes remembers which proxy modes were active so cleanup only tears
// down the routes that were actually installed, avoiding spurious errors when
// running ip rule/route del on entries that were never added.
var activeModes config.ProxyModes

// activeIPv6 remembers whether IPv6 routes were installed.
var activeIPv6 bool

// Ports holds the listen ports that nftables needs to know about.
type Ports struct {
	DNS      int
	TProxy   int
	Redirect int
}

// Apply sets up nftables rules for the chosen TCP/UDP proxy modes.
// tunDevice is the TUN interface name configured by the user (e.g. "metaviz",
// "tun0"). It is used in both the nft iifname match and the ip route rules.
func Apply(modes config.ProxyModes, ports Ports, lanProxy bool, ipv6 bool, bypassCN bool, tunDevice string, dataDir string, gid uint32, ipf ipfilter.Config) error {
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

	if modes.IsSystemProxyOnly() {
		log.Println("firewall: system_proxy only — no nftables rules")
		return nil
	}

	if err := setup(modes, ports, lanProxy, ipv6, bypassCN, tunDevice, gid, ipf); err != nil {
		return fmt.Errorf("nft setup: %w", err)
	}

	SyncLocalIPs()
	return nil
}

// ApplyTunRoutes re-adds the ip rule/route entries for TUN mode.
// Called after mihomo has started and created the TUN device.
func ApplyTunRoutes(ipv6 bool) {
	mu.Lock()
	defer mu.Unlock()
	if activeTunDevice == "" {
		return
	}
	setupTunRoutes(ipv6, activeTunDevice)
}

// Stop tears down nftables rules and ip routes for the last active modes.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	cleanup(activeModes, activeIPv6, activeTunDevice)
	activeTunDevice = ""
	activeModes = config.ProxyModes{}
	activeIPv6 = false
}
