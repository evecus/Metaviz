package firewall

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/metaviz/internal/config"
	"github.com/metaviz/internal/ipfilter"
)

var nftConfPath string

func SetNftConfPath(dir string) {
	nftConfPath = filepath.Join(dir, "metaviz-nft.conf")
}

// ── Constants ──────────────────────────────────────────────────────────────

const (
	tunFwMark = "0x41"
	tunFwMask = "0xc1"
	tunTable  = 101
)

// privateRangesV4 covers fib-type addresses and all RFC-reserved IPv4 ranges.
// When fakeip is true, 198.18.0.0/15 is exempted via != condition so fakeip
// traffic falls through to the proxy mark instead of being returned early.
func privateRangesV4(fakeip bool) string {
	if fakeip {
		return "" +
			"        fib daddr type { local, broadcast, anycast, multicast } return\n" +
			"        ip daddr != 198.18.0.0/15 ip daddr { 0.0.0.0/8, 10.0.0.0/8, " +
			"100.64.0.0/10, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, " +
			"192.0.0.0/24, 192.0.2.0/24, 192.88.99.0/24, 192.168.0.0/16, " +
			"198.18.0.0/15, 198.51.100.0/24, 203.0.113.0/24, 224.0.0.0/3 } return\n"
	}
	return "" +
		"        fib daddr type { local, broadcast, anycast, multicast } return\n" +
		"        ip daddr { 0.0.0.0/8, 10.0.0.0/8, 100.64.0.0/10, 127.0.0.0/8, " +
		"169.254.0.0/16, 172.16.0.0/12, 192.0.0.0/24, 192.0.2.0/24, 192.88.99.0/24, " +
		"192.168.0.0/16, 198.18.0.0/15, 198.51.100.0/24, 203.0.113.0/24, 224.0.0.0/3 } return\n"
}

// privateRangesV6 covers all RFC-reserved IPv6 ranges.
// When fakeip is true, fc00::/18 is exempted so fakeip IPv6 traffic reaches
// the proxy instead of being returned early.
func privateRangesV6(fakeip bool) string {
	if fakeip {
		return "        ip6 daddr != fc00::/18 ip6 daddr { ::/127, fc00::/7, fe80::/10, ff00::/8 } return\n"
	}
	return "        ip6 daddr { ::/127, fc00::/7, fe80::/10, ff00::/8 } return\n"
}

// ── Entry point ────────────────────────────────────────────────────────────

func setup(modes config.ProxyModes, ports Ports, lanProxy bool, ipv6 bool, bypassCN bool, tunDevice string, gid uint32, extraGIDs []uint32, ipf ipfilter.Config, fakeIP bool) error {
	conf := buildTable(modes, ports, lanProxy, ipv6, bypassCN, tunDevice, gid, extraGIDs, ipf, fakeIP)
	if err := os.WriteFile(nftConfPath, []byte(conf), 0644); err != nil {
		return fmt.Errorf("write nft conf: %w", err)
	}
	if err := setupRoutes(modes, ipv6, tunDevice, fakeIP); err != nil {
		return err
	}
	if lanProxy {
		if err := enableIPForward(ipv6); err != nil {
			log.Printf("firewall: ip_forward: %v", err)
		}
	}
	if err := runCmd("nft -f " + nftConfPath); err != nil {
		return err
	}
	if bypassCN {
		cnNftPath := filepath.Join(filepath.Dir(nftConfPath), "cn-bypass.nft")
		if _, err := os.Stat(cnNftPath); err == nil {
			if err := runCmd("nft -f " + cnNftPath); err != nil {
				log.Printf("firewall: cn-bypass load: %v", err)
			} else {
				log.Println("firewall: CN bypass rules loaded")
			}
		} else {
			log.Printf("firewall: cn-bypass.nft not found at %s", cnNftPath)
		}
		if ipv6 {
			cn6NftPath := filepath.Join(filepath.Dir(nftConfPath), "cn-bypass6.nft")
			if _, err := os.Stat(cn6NftPath); err == nil {
				if err := runCmd("nft -f " + cn6NftPath); err != nil {
					log.Printf("firewall: cn-bypass6 load: %v", err)
				} else {
					log.Println("firewall: CN bypass IPv6 rules loaded")
				}
			} else {
				log.Printf("firewall: cn-bypass6.nft not found at %s", cn6NftPath)
			}
		}
	}
	return nil
}

// setupRoutes installs ip rule / ip route entries for the active modes.
func setupRoutes(modes config.ProxyModes, ipv6 bool, tunDevice string, fakeIP bool) error {
	if modes.NeedsTProxyInbound() {
		if err := setupTProxyRoutes(ipv6); err != nil {
			return err
		}
	}
	if modes.NeedsTunInbound() {
		if err := setupTunRoutes(ipv6, tunDevice, fakeIP); err != nil {
			return err
		}
	}
	return nil
}

func setupTProxyRoutes(ipv6 bool) error {
	cmds := []string{
		"ip rule add fwmark 0x40/0xc0 table 100",
		"ip route add local 0.0.0.0/0 dev lo table 100",
	}
	if ipv6 {
		cmds = append(cmds,
			"ip -6 rule add fwmark 0x40/0xc0 table 100",
			"ip -6 route add local ::/0 dev lo table 100",
		)
	}
	for _, c := range cmds {
		if err := runCmd(c); err != nil {
			log.Printf("firewall: tproxy route: %v", err)
		}
	}
	return nil
}

func setupTunRoutes(ipv6 bool, tunDevice string, fakeIP bool) error {
	cmds := []string{
		fmt.Sprintf("ip rule add fwmark %s/%s table %d", tunFwMark, tunFwMask, tunTable),
		fmt.Sprintf("ip route add default dev %s table %d", tunDevice, tunTable),
	}
	if fakeIP {
		cmds = append(cmds,
			fmt.Sprintf("ip route add 198.18.0.0/15 dev %s", tunDevice),
		)
	}
	if ipv6 {
		cmds = append(cmds,
			fmt.Sprintf("ip -6 rule add fwmark %s/%s table %d", tunFwMark, tunFwMask, tunTable),
			fmt.Sprintf("ip -6 route add default dev %s table %d", tunDevice, tunTable),
		)
		if fakeIP {
			cmds = append(cmds,
				fmt.Sprintf("ip -6 route add fc00::/18 dev %s", tunDevice),
			)
		}
	}
	for _, c := range cmds {
		if err := runCmd(c); err != nil {
			log.Printf("firewall: tun route: %v", err)
		}
	}
	return nil
}

// ── IP filter set ──────────────────────────────────────────────────────────

func buildIPFilterNft(ipf ipfilter.Config, lanProxy bool) (setDef string, ruleSnippet string) {
	if ipf.Mode == ipfilter.ModeOff || !lanProxy || len(ipf.IPs) == 0 {
		return "", ""
	}
	parts := ipf.IPs
	var elems []string
	for _, p := range parts {
		if strings.Contains(p, "/") {
			ip, _, err := net.ParseCIDR(p)
			if err != nil {
				continue
			}
			if ip.To4() == nil {
				log.Printf("firewall: ip_filter: skipping IPv6 CIDR %s (only IPv4 supported)", p)
				continue
			}
			elems = append(elems, p)
		} else {
			ip := net.ParseIP(p)
			if ip == nil {
				continue
			}
			if ip.To4() == nil {
				log.Printf("firewall: ip_filter: skipping IPv6 address %s (only IPv4 supported)", p)
				continue
			}
			elems = append(elems, p)
		}
	}
	if len(elems) == 0 {
		return "", ""
	}
	setDef = fmt.Sprintf(
		"    set ip_filter {\n        type ipv4_addr\n        flags interval\n        auto-merge\n        elements = { %s }\n    }\n",
		strings.Join(elems, ", "),
	)
	switch ipf.Mode {
	case ipfilter.ModeBlacklist:
		ruleSnippet = "        ip saddr @ip_filter return\n"
	case ipfilter.ModeWhitelist:
		ruleSnippet = "        ip saddr != @ip_filter return\n"
	}
	return setDef, ruleSnippet
}

// ── Main table builder ─────────────────────────────────────────────────────

func buildTable(modes config.ProxyModes, ports Ports, lanProxy bool, ipv6 bool, bypassCN bool, tunDevice string, gid uint32, extraGIDs []uint32, ipf ipfilter.Config, fakeIP bool) string {
	ipfSetDef, ipfRule := buildIPFilterNft(ipf, lanProxy)
	var s strings.Builder

	s.WriteString("table inet metaviz {\n")

	s.WriteString("    set interface {\n        type ipv4_addr\n        flags interval\n        auto-merge\n    }\n")
	if ipv6 {
		s.WriteString("    set interface6 {\n        type ipv6_addr\n        flags interval\n        auto-merge\n    }\n")
	}

	if bypassCN {
		s.WriteString("    set cn_bypass {\n        type ipv4_addr\n        flags interval\n        auto-merge\n    }\n")
		if ipv6 {
			s.WriteString("    set cn_bypass6 {\n        type ipv6_addr\n        flags interval\n        auto-merge\n    }\n")
		}
	}

	if ipfSetDef != "" {
		s.WriteString(ipfSetDef)
	}

	if modes.NeedsTProxyInbound() {
		s.WriteString(`
    chain tp_mark {
        tcp flags & (fin | syn | rst | ack) == syn meta mark set mark | 0x40
        meta l4proto udp ct state new meta mark set mark | 0x40
        ct mark set mark
    }
`)
	}
	if modes.NeedsTunInbound() {
		s.WriteString(fmt.Sprintf(`
    chain tun_mark {
        meta mark set meta mark | %s
        ct mark set meta mark
    }
`, tunFwMark))
	}

	s.WriteString(buildProxyRuleChain(modes, ipfRule, ipv6, bypassCN, fakeIP))
	s.WriteString(buildManglePrerouting(modes, ports, lanProxy, ipv6, tunDevice))
	s.WriteString(buildMangleOutput(modes, ipv6, gid, extraGIDs))

	s.WriteString(`
    chain prerouting_mangle {
        type filter hook prerouting priority mangle - 5; policy accept;
        jump proxy_pre
    }

    chain output_mangle {
        type route hook output priority mangle - 5; policy accept;
        jump proxy_out
    }
`)

	s.WriteString(buildNATChains(modes, ports, ipv6, gid, extraGIDs, fakeIP))
	s.WriteString("}\n")
	return s.String()
}

// ── proxy_rule chain ───────────────────────────────────────────────────────

func buildProxyRuleChain(modes config.ProxyModes, ipfRule string, ipv6 bool, bypassCN bool, fakeIP bool) string {
	var s strings.Builder
	s.WriteString("\n    chain proxy_rule {\n")

	if modes.NeedsTProxyInbound() {
		s.WriteString("        meta mark set ct mark\n")
		s.WriteString("        meta mark & 0xc0 == 0x40 return\n")
	}
	if modes.NeedsTunInbound() {
		s.WriteString("        meta mark set ct mark\n")
		s.WriteString(fmt.Sprintf("        meta mark & %s == %s return\n", tunFwMask, tunFwMark))
	}

	// Use fakeip-aware privateRanges functions: when fakeIP is true, 198.18.0.0/15
	// and fc00::/18 are exempted via != condition so fakeip traffic falls through
	// to the proxy mark rules below instead of being returned early.
	s.WriteString(privateRangesV4(fakeIP))
	if ipv6 {
		s.WriteString(privateRangesV6(fakeIP))
	}
	s.WriteString("        ip daddr @interface return\n")
	if ipv6 {
		s.WriteString("        ip6 daddr @interface6 return\n")
	}
	s.WriteString("        meta l4proto { tcp, udp } th dport 53 return\n")

	if ipfRule != "" {
		s.WriteString(ipfRule)
	}

	if bypassCN {
		s.WriteString("        ip daddr @cn_bypass return\n")
		if ipv6 {
			s.WriteString("        ip6 daddr @cn_bypass6 return\n")
		}
	}

	switch modes.TCP {
	case config.TCPModeTProxy:
		s.WriteString("        meta l4proto tcp jump tp_mark\n")
	case config.TCPModeTun:
		s.WriteString("        meta l4proto tcp jump tun_mark\n")
	}

	switch modes.UDP {
	case config.UDPModeTProxy:
		s.WriteString("        meta l4proto udp jump tp_mark\n")
	case config.UDPModeTun:
		s.WriteString("        meta l4proto udp jump tun_mark\n")
	}

	s.WriteString("    }\n")
	return s.String()
}

// ── Mangle prerouting chain ────────────────────────────────────────────────

func buildManglePrerouting(modes config.ProxyModes, ports Ports, lanProxy bool, ipv6 bool, tunDevice string) string {
	var s strings.Builder
	s.WriteString("\n    chain proxy_pre {\n")

	if modes.NeedsTunInbound() {
		s.WriteString(fmt.Sprintf("        iifname \"%s\" return\n", tunDevice))
	}
	if modes.NeedsTProxyInbound() {
		s.WriteString("        iifname \"lo\" mark & 0xc0 != 0x40 return\n")
	}

	if lanProxy {
		if ipv6 {
			s.WriteString("        meta nfproto { ipv4, ipv6 } meta l4proto { tcp, udp } fib saddr type != local fib daddr type != local jump proxy_rule\n")
		} else {
			s.WriteString("        meta nfproto ipv4 meta l4proto { tcp, udp } fib saddr type != local fib daddr type != local jump proxy_rule\n")
		}
	}

	if modes.NeedsTProxyInbound() {
		s.WriteString(fmt.Sprintf("        meta nfproto ipv4 meta l4proto { tcp, udp } mark & 0xc0 == 0x40 tproxy ip to 127.0.0.1:%d\n", ports.TProxy))
		if ipv6 {
			s.WriteString(fmt.Sprintf("        meta nfproto ipv6 meta l4proto { tcp, udp } mark & 0xc0 == 0x40 tproxy ip6 to [::1]:%d\n", ports.TProxy))
		}
	}

	s.WriteString("    }\n")
	return s.String()
}

// ── Helpers ────────────────────────────────────────────────────────────────

// skgidRule returns a single nftables rule bypassing all given GIDs.
// Single GID: "meta skgid N return"
// Multiple:   "meta skgid { N, M, ... } return"
func skgidRule(gid uint32, extraGIDs []uint32) string {
	all := make([]uint32, 0, 1+len(extraGIDs))
	all = append(all, gid)
	all = append(all, extraGIDs...)
	if len(all) == 1 {
		return fmt.Sprintf("        meta skgid %d return\n", all[0])
	}
	parts := make([]string, len(all))
	for i, g := range all {
		parts[i] = fmt.Sprintf("%d", g)
	}
	return fmt.Sprintf("        meta skgid { %s } return\n", strings.Join(parts, ", "))
}

// ── Mangle output chain ────────────────────────────────────────────────────

func buildMangleOutput(modes config.ProxyModes, ipv6 bool, gid uint32, extraGIDs []uint32) string {
	var s strings.Builder
	s.WriteString("\n    chain proxy_out {\n")
	s.WriteString(skgidRule(gid, extraGIDs))
	nfproto := "meta nfproto ipv4"
	if ipv6 {
		nfproto = "meta nfproto { ipv4, ipv6 }"
	}
	s.WriteString(fmt.Sprintf("        %s meta l4proto { tcp, udp } fib saddr type local fib daddr type != local jump proxy_rule\n", nfproto))
	s.WriteString("    }\n")
	return s.String()
}

// ── NAT chains ─────────────────────────────────────────────────────────────

func buildNATChains(modes config.ProxyModes, ports Ports, ipv6 bool, gid uint32, extraGIDs []uint32, fakeIP bool) string {
	var s strings.Builder

	dnsV4 := fmt.Sprintf("        ip daddr != 127.0.0.1 meta l4proto { tcp, udp } th dport 53 redirect to :%d\n", ports.DNS)
	dnsV6 := ""
	if ipv6 {
		dnsV6 = fmt.Sprintf("        ip6 daddr != ::1 meta l4proto { tcp, udp } th dport 53 redirect to :%d\n", ports.DNS)
	}
	skgid := skgidRule(gid, extraGIDs)
	s.WriteString(fmt.Sprintf(`
    chain dns_redirect {
%s        meta l4proto { tcp, udp } th dport %d return
%s%s    }
`, skgid, ports.DNS, dnsV4, dnsV6))

	if modes.TCP == config.TCPModeRedir {
		nfproto := "meta nfproto ipv4"
		if ipv6 {
			nfproto = "meta nfproto { ipv4, ipv6 }"
		}
		ipv6Ranges := ""
		if ipv6 {
			ipv6Ranges = privateRangesV6(fakeIP)
		}
		s.WriteString(fmt.Sprintf(`
    chain tcp_redirect {
%s%s%s        ip daddr @interface return
        %s meta l4proto tcp redirect to :%d
    }
`, skgid, privateRangesV4(fakeIP), ipv6Ranges, nfproto, ports.Redirect))
	}

	// fakeip ping 劫持链：把发往 fakeip 段的 ICMP echo-request 重定向到本机，
	// 让内核直接回包，避免 ping 测延迟时超时失败。参考 xproxy 实现。
	if fakeIP {
		s.WriteString(fmt.Sprintf(`
    chain fakeip_ping {
        icmp type echo-request ip daddr %s counter redirect
`, "198.18.0.0/15"))
		if ipv6 {
			s.WriteString(fmt.Sprintf("        icmpv6 type echo-request ip6 daddr %s counter redirect\n", "fc00::/18"))
		}
		s.WriteString("    }\n")
	}

	s.WriteString("\n    chain prerouting_nat {\n")
	s.WriteString("        type nat hook prerouting priority dstnat - 5; policy accept;\n")
	s.WriteString("        jump dns_redirect\n")
	if modes.TCP == config.TCPModeRedir {
		s.WriteString("        jump tcp_redirect\n")
	}
	if fakeIP {
		s.WriteString("        jump fakeip_ping\n")
	}
	s.WriteString("    }\n")

	s.WriteString("\n    chain output_nat {\n")
	s.WriteString("        type nat hook output priority -105; policy accept;\n")
	s.WriteString("        jump dns_redirect\n")
	if modes.TCP == config.TCPModeRedir {
		s.WriteString("        jump tcp_redirect\n")
	}
	if fakeIP {
		s.WriteString("        jump fakeip_ping\n")
	}
	s.WriteString("    }\n")

	return s.String()
}

// ── Cleanup ────────────────────────────────────────────────────────────────

func cleanup(modes config.ProxyModes, ipv6 bool, tunDevice string) {
	_ = runCmd("nft delete table inet metaviz")
	// Clean up legacy standalone cn-bypass table left by older versions.
	_ = runCmd("nft delete table inet metaviz_cnbypass")

	if modes.NeedsTProxyInbound() {
		cleanupTProxyRoutes(ipv6)
	}
	if modes.NeedsTunInbound() {
		cleanupTunRoutes(ipv6, tunDevice)
	}

	if nftConfPath != "" {
		_ = os.Remove(nftConfPath)
	}
}

func cleanupTProxyRoutes(ipv6 bool) {
	cmds := []string{
		"ip rule del fwmark 0x40/0xc0 table 100",
		"ip route del local 0.0.0.0/0 dev lo table 100",
	}
	if ipv6 {
		cmds = append(cmds,
			"ip -6 rule del fwmark 0x40/0xc0 table 100",
			"ip -6 route del local ::/0 dev lo table 100",
		)
	}
	for _, c := range cmds {
		_ = runCmd(c)
	}
}

func cleanupTunRoutes(ipv6 bool, tunDevice string) {
	if tunDevice == "" {
		tunDevice = "metaviz"
	}
	cmds := []string{
		fmt.Sprintf("ip rule del fwmark %s/%s table %d", tunFwMark, tunFwMask, tunTable),
		fmt.Sprintf("ip route del default dev %s table %d", tunDevice, tunTable),
		// Always attempt to remove fakeip routes (harmless if not present)
		fmt.Sprintf("ip route del 198.18.0.0/15 dev %s", tunDevice),
	}
	if ipv6 {
		cmds = append(cmds,
			fmt.Sprintf("ip -6 rule del fwmark %s/%s table %d", tunFwMark, tunFwMask, tunTable),
			fmt.Sprintf("ip -6 route del default dev %s table %d", tunDevice, tunTable),
			fmt.Sprintf("ip -6 route del fc00::/18 dev %s", tunDevice),
		)
	}
	for _, c := range cmds {
		_ = runCmd(c)
	}
}

// ── Interface IP management ────────────────────────────────────────────────

func AddInterfaceIP(cidr string) {
	set := "interface"
	if !strings.Contains(cidr, ".") {
		if !activeIPv6 {
			return
		}
		set = "interface6"
	}
	if err := runCmd(fmt.Sprintf("nft add element inet metaviz %s { %s }", set, cidr)); err != nil {
		log.Printf("firewall: add %s: %v", cidr, err)
	}
}

func RemoveInterfaceIP(cidr string) {
	set := "interface"
	if !strings.Contains(cidr, ".") {
		if !activeIPv6 {
			return
		}
		set = "interface6"
	}
	if err := runCmd(fmt.Sprintf("nft delete element inet metaviz %s { %s }", set, cidr)); err != nil {
		log.Printf("firewall: remove %s: %v", cidr, err)
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func enableIPForward(ipv6 bool) error {
	if err := runCmd("sysctl -w net.ipv4.ip_forward=1"); err != nil {
		return err
	}
	if ipv6 {
		return runCmd("sysctl -w net.ipv6.conf.all.forwarding=1")
	}
	return nil
}

func runCmd(command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}
	out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w (output: %s)", command, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func IsIPv6Supported() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.IsLoopback() && ipnet.IP.To4() == nil {
					return true
				}
			}
		}
	}
	return false
}
