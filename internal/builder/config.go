package builder

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/metaviz/internal/node"
)

type RouteMode string

const (
	RouteModeWhitelist RouteMode = "whitelist"
	RouteModeGFWList   RouteMode = "gfwlist"
	RouteModeGlobal    RouteMode = "global"
)

// FakeIP 池地址段常量，供 builder 和 firewall 共用。
const (
	FakeIPRange  = "198.18.0.0/15"
	FakeIP6Range = "fc00::/18"
)

// GlobalConfig 是注入到每个生成/上传配置里的全局字段。
type GlobalConfig struct {
	MixedPort    int
	RedirectPort int
	TProxyPort   int
	DNSPort      int
	AllowLan     bool
	IPv6         bool
	LogLevel     string
	TunEnable    bool
	TunDevice    string
	TunStack     string
	TunMTU       int
	SnifferEnable              bool
	SnifferOverrideDestination bool
	ClashAPIListen string
	ClashAPISecret string
	ClashAPIUI     string
	FindProcessMode string
	UnifiedDelay    bool
	TCPConcurrent   bool
	// FakeIP 开启时，生成的 dns 块使用 fake-ip 模式，并注入 profile.store-fake-ip。
	// 仅影响单节点/订阅模式；上传配置模式由用户自行控制。
	FakeIP bool
}

// BuildNodeConfig 生成单节点模式的完整 mihomo YAML。
func BuildNodeConfig(routeMode RouteMode, n *node.Node, mrsDir string, blockAds bool, global GlobalConfig) ([]byte, error) {
	proxy, err := NodeToProxy(n)
	if err != nil {
		return nil, fmt.Errorf("convert node: %w", err)
	}

	cfg := buildBase(global)
	cfg["proxies"] = []interface{}{proxy}
	cfg["proxy-groups"] = []interface{}{}
	cfg["rule-providers"] = buildRuleProviders(routeMode, mrsDir, blockAds, global.FakeIP)
	cfg["rules"] = buildRules(routeMode, n.Name, blockAds)
	cfg["dns"] = buildDNS(routeMode, global.DNSPort, global.IPv6, global.FakeIP)

	return yaml.Marshal(cfg)
}

// BuildSubscriptionConfig 生成订阅模式的完整 mihomo YAML。
func BuildSubscriptionConfig(routeMode RouteMode, subID, subName, subURL, mrsDir string, blockAds bool, global GlobalConfig) ([]byte, error) {
	cfg := buildBase(global)

	providerPath := fmt.Sprintf("./providers/%s.yaml", subID)
	cfg["proxy-providers"] = M{
		subName: M{
			"type":     "http",
			"url":      subURL,
			"interval": 86400,
			"path":     providerPath,
			"health-check": M{
				"enable":   true,
				"interval": 600,
				"url":      "https://www.gstatic.com/generate_204",
			},
		},
	}

	cfg["proxy-groups"] = []interface{}{
		M{
			"name": "节点选择",
			"type": "select",
			"use":  []string{subName},
		},
	}

	cfg["rule-providers"] = buildRuleProviders(routeMode, mrsDir, blockAds, global.FakeIP)
	cfg["rules"] = buildRules(routeMode, "节点选择", blockAds)
	cfg["dns"] = buildDNS(routeMode, global.DNSPort, global.IPv6, global.FakeIP)

	return yaml.Marshal(cfg)
}

// BuildSubNodeConfig 生成订阅里选单个节点的配置（subnode 模式）。
func BuildSubNodeConfig(routeMode RouteMode, n *node.Node, mrsDir string, blockAds bool, global GlobalConfig) ([]byte, error) {
	return BuildNodeConfig(routeMode, n, mrsDir, blockAds, global)
}

// PatchUploadConfig 把全局设置字段覆盖合并进上传的配置 YAML。
// 注意：上传配置模式下不注入 fakeip 相关字段，DNS 由用户自行控制。
func PatchUploadConfig(src []byte, global GlobalConfig) ([]byte, error) {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(src, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	// 覆盖端口与基础字段
	cfg["mixed-port"] = global.MixedPort
	cfg["redir-port"] = global.RedirectPort
	cfg["tproxy-port"] = global.TProxyPort
	cfg["allow-lan"] = global.AllowLan
	cfg["ipv6"] = global.IPv6
	cfg["log-level"] = global.LogLevel
	cfg["unified-delay"] = global.UnifiedDelay
	cfg["tcp-concurrent"] = global.TCPConcurrent
	if global.FindProcessMode != "" {
		cfg["find-process-mode"] = global.FindProcessMode
	}
	cfg["geodata-mode"] = false

	// Clash API
	if global.ClashAPIListen != "" {
		cfg["external-controller"] = global.ClashAPIListen
	}
	if global.ClashAPISecret != "" {
		cfg["secret"] = global.ClashAPISecret
	}
	if global.ClashAPIUI != "" {
		cfg["external-ui"] = global.ClashAPIUI
	}

	// TUN（整块覆盖）
	cfg["tun"] = buildTun(global)

	// Sniffer（整块覆盖）
	cfg["sniffer"] = buildSniffer(global.SnifferEnable, global.SnifferOverrideDestination)

	// DNS：只覆盖 listen 端口，其余由用户控制
	if dns, ok := cfg["dns"].(map[string]interface{}); ok {
		dns["listen"] = fmt.Sprintf("0.0.0.0:%d", global.DNSPort)
	}

	return yaml.Marshal(cfg)
}

// ── 基础配置 ────────────────────────────────────────────────────────────────

func buildBase(g GlobalConfig) map[string]interface{} {
	cfg := map[string]interface{}{
		"mixed-port":        g.MixedPort,
		"redir-port":        g.RedirectPort,
		"tproxy-port":       g.TProxyPort,
		"allow-lan":         g.AllowLan,
		"ipv6":              g.IPv6,
		"log-level":         g.LogLevel,
		"unified-delay":     g.UnifiedDelay,
		"tcp-concurrent":    g.TCPConcurrent,
		"find-process-mode": g.FindProcessMode,
		"geodata-mode":      false,
		"tun":               buildTun(g),
		"sniffer":           buildSniffer(g.SnifferEnable, g.SnifferOverrideDestination),
	}
	if g.ClashAPIListen != "" {
		cfg["external-controller"] = g.ClashAPIListen
	}
	if g.ClashAPISecret != "" {
		cfg["secret"] = g.ClashAPISecret
	}
	if g.ClashAPIUI != "" {
		cfg["external-ui"] = g.ClashAPIUI
	}
	// fake-ip 模式需要 profile.store-fake-ip 持久化 IP 映射
	if g.FakeIP {
		cfg["profile"] = M{"store-fake-ip": true}
	}
	return cfg
}

func buildTun(g GlobalConfig) M {
	return M{
		"enable":                g.TunEnable,
		"device":                g.TunDevice,
		"stack":                 g.TunStack,
		"mtu":                   g.TunMTU,
		"auto-route":            false,
		"strict-route":          false,
		"auto-detect-interface": false,
	}
}

func buildSniffer(enable, overrideDestination bool) M {
	return M{
		"enable": enable,
		"sniff": M{
			"HTTP": M{
				"ports":                []int{80, 8080, 8880, 2052, 2082, 2086, 2095},
				"override-destination": overrideDestination,
			},
			"TLS": M{
				"ports":                []int{443, 8443, 2053, 2083, 2087, 2096},
				"override-destination": overrideDestination,
			},
			"QUIC": M{
				"ports":                []int{443, 8443},
				"override-destination": overrideDestination,
			},
		},
	}
}

// ── DNS ─────────────────────────────────────────────────────────────────────

func buildDNS(mode RouteMode, dnsPort int, ipv6 bool, fakeIP bool) M {
	listen := fmt.Sprintf("0.0.0.0:%d", dnsPort)
	base := M{
		"enable":          true,
		"cache-algorithm": "arc",
		"listen":          listen,
		"ipv6":            ipv6,
		"default-nameserver":       []string{"223.5.5.5"},
		"proxy-server-nameserver":  []string{"223.5.5.5"},
	}

	if fakeIP {
		return buildDNSFakeIP(mode, base)
	}
	return buildDNSRedirHost(mode, base)
}

// buildDNSRedirHost 生成 redir-host 模式的 DNS 配置（原有逻辑）。
func buildDNSRedirHost(mode RouteMode, base M) M {
	base["enhanced-mode"] = "redir-host"

	switch mode {
	case RouteModeGFWList:
		base["respect-rules"] = true
		base["nameserver"] = []string{"223.5.5.5", "119.29.29.29"}
		base["nameserver-policy"] = M{
			"rule-set:geosite-gfw,geosite-geolocation-!cn": []string{
				"tls://1.1.1.1",
				"tls://8.8.8.8",
			},
		}

	case RouteModeWhitelist:
		base["respect-rules"] = true
		base["nameserver"] = []string{"tls://1.1.1.1", "tls://8.8.8.8"}
		base["nameserver-policy"] = M{
			"rule-set:geosite-cn": []string{"223.5.5.5", "119.29.29.29"},
		}

	case RouteModeGlobal:
		base["nameserver"] = []string{"tls://1.1.1.1", "tls://8.8.8.8"}
	}

	return base
}

// buildDNSFakeIP 生成 fake-ip 模式的 DNS 配置。
func buildDNSFakeIP(mode RouteMode, base M) M {
	base["enhanced-mode"] = "fake-ip"
	base["fake-ip-range"] = FakeIPRange
	base["fake-ip-range6"] = FakeIP6Range

	switch mode {
	case RouteModeWhitelist:
		// 大陆白名单：国内域名直连解析，其余走代理并分配 fakeip
		base["fake-ip-filter-mode"] = "blacklist"
		base["fake-ip-filter"] = []string{"rule-set:fakeipfilter,geosite-cn"}
		base["respect-rules"] = true
		base["nameserver"] = []string{"tls://1.1.1.1", "tls://8.8.8.8"}
		base["nameserver-policy"] = M{
			"rule-set:geosite-cn": []string{"223.5.5.5", "119.29.29.29"},
		}

	case RouteModeGFWList:
		// GFW 列表：被墙域名分配 fakeip，其余直连解析
		base["fake-ip-filter-mode"] = "whitelist"
		base["fake-ip-filter"] = []string{"rule-set:geosite-gfw,geosite-geolocation-!cn"}
		base["nameserver"] = []string{"223.5.5.5", "119.29.29.29"}

	case RouteModeGlobal:
		// 全局：所有域名分配 fakeip
		base["fake-ip-filter-mode"] = "blacklist"
		// fake-ip-filter 留空 = 全部域名都走 fakeip
		base["nameserver"] = []string{"tls://1.1.1.1", "tls://8.8.8.8"}
	}

	return base
}

// ── Rule Providers ──────────────────────────────────────────────────────────

func buildRuleProviders(mode RouteMode, mrsDir string, blockAds bool, fakeIP bool) M {
	rp := M{}

	switch mode {
	case RouteModeWhitelist:
		rp["geosite-cn"] = localRuleProvider(mrsDir, "geosite-cn", "domain")
		rp["geoip-cn"] = localRuleProvider(mrsDir, "geoip-cn", "ipcidr")
		// 大陆白名单 + fakeip 需要 fakeipfilter 规则集过滤不能 fakeip 的域名
		if fakeIP {
			rp["fakeipfilter"] = localRuleProvider(mrsDir, "fakeipfilter", "domain")
		}

	case RouteModeGFWList:
		rp["geosite-gfw"] = localRuleProvider(mrsDir, "geosite-gfw", "domain")
		rp["geosite-geolocation-!cn"] = localRuleProvider(mrsDir, "geosite-geolocation-!cn", "domain")
		rp["geoip-telegram"] = localRuleProvider(mrsDir, "geoip-telegram", "ipcidr")
	}

	if blockAds {
		rp["ads"] = localRuleProvider(mrsDir, "ads", "domain")
	}

	return rp
}

func localRuleProvider(mrsDir, name, behavior string) M {
	return M{
		"type":     "file",
		"behavior": behavior,
		"format":   "mrs",
		"path":     fmt.Sprintf("./mrs/%s.mrs", name),
	}
}

// ── Rules ───────────────────────────────────────────────────────────────────

func buildRules(mode RouteMode, proxyName string, blockAds bool) []string {
	var rules []string

	switch mode {
	case RouteModeWhitelist:
		if blockAds {
			rules = append(rules, "RULE-SET,ads,REJECT")
		}
		rules = append(rules,
			"RULE-SET,geosite-cn,DIRECT",
			"RULE-SET,geoip-cn,DIRECT",
			fmt.Sprintf("MATCH,%s", proxyName),
		)

	case RouteModeGFWList:
		rules = append(rules,
			fmt.Sprintf("IP-CIDR,1.1.1.1/32,%s,no-resolve", proxyName),
			fmt.Sprintf("IP-CIDR,8.8.8.8/32,%s,no-resolve", proxyName),
		)
		if blockAds {
			rules = append(rules, "RULE-SET,ads,REJECT")
		}
		rules = append(rules,
			fmt.Sprintf("RULE-SET,geosite-gfw,%s", proxyName),
			fmt.Sprintf("RULE-SET,geosite-geolocation-!cn,%s", proxyName),
			fmt.Sprintf("RULE-SET,geoip-telegram,%s,no-resolve", proxyName),
			"MATCH,DIRECT",
		)

	case RouteModeGlobal:
		if blockAds {
			rules = append(rules, "RULE-SET,ads,REJECT")
		}
		rules = append(rules, fmt.Sprintf("MATCH,%s", proxyName))
	}

	return rules
}

// ValidateYAML checks if data is valid YAML with at least one recognizable field.
func ValidateYAML(data []byte) error {
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	if len(m) == 0 {
		return fmt.Errorf("empty config")
	}
	return nil
}

// SummarizeInbounds extracts inbound-like info from a mihomo YAML for display.
func SummarizeInbounds(data []byte) []map[string]interface{} {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	var result []map[string]interface{}
	for _, key := range []string{"mixed-port", "redir-port", "tproxy-port", "socks-port", "port"} {
		if v, ok := cfg[key]; ok {
			result = append(result, map[string]interface{}{"type": key, "port": v})
		}
	}
	if tun, ok := cfg["tun"].(map[string]interface{}); ok {
		if enabled, _ := tun["enable"].(bool); enabled {
			result = append(result, map[string]interface{}{"type": "tun", "device": tun["device"]})
		}
	}
	return result
}

var _ = strings.Join // keep import
