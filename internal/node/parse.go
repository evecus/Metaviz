package node

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseLinks parses a multi-line block of share links.
func ParseLinks(text string) ([]*Node, []string) {
	var nodes []*Node
	var errs []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		n, err := ParseLink(line)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s  →  %v", trunc(line, 48), err))
			continue
		}
		n.ID = NewID()
		nodes = append(nodes, n)
	}
	return nodes, errs
}

// ParseLink parses a single share link.
func ParseLink(s string) (*Node, error) {
	low := strings.ToLower(s)
	switch {
	case strings.HasPrefix(low, "vmess://"):
		return parseVMess(s)
	case strings.HasPrefix(low, "vless://"):
		return parseVLESS(s)
	case strings.HasPrefix(low, "trojan://"):
		return parseTrojan(s)
	case strings.HasPrefix(low, "ss://"):
		return parseSS(s)
	case strings.HasPrefix(low, "tuic://"):
		return parseTUIC(s)
	case strings.HasPrefix(low, "hy2://"), strings.HasPrefix(low, "hysteria2://"):
		return parseHysteria2(s)
	case strings.HasPrefix(low, "wireguard://"), strings.HasPrefix(low, "wg://"):
		return parseWireGuard(s)
	case strings.HasPrefix(low, "socks5://"), strings.HasPrefix(low, "socks://"):
		return parseSOCKS5(s)
	case strings.HasPrefix(low, "https://"):
		return parseHTTPProxy(s, true)
	case strings.HasPrefix(low, "http://"):
		return parseHTTPProxy(s, false)
	default:
		return nil, fmt.Errorf("unsupported protocol")
	}
}

// ── VMess ──────────────────────────────────────────────────────────────────

type vmessJSON struct {
	PS   string      `json:"ps"`
	Add  string      `json:"add"`
	Port interface{} `json:"port"`
	ID   string      `json:"id"`
	Aid  interface{} `json:"aid"`
	Scy  string      `json:"scy"`
	Net  string      `json:"net"`
	Type string      `json:"type"`
	Host string      `json:"host"`
	Path string      `json:"path"`
	TLS  string      `json:"tls"`
	SNI  string      `json:"sni"`
	ALPN string      `json:"alpn"`
	FP   string      `json:"fp"`
}

func parseVMess(s string) (*Node, error) {
	raw := s[len("vmess://"):]

	// Standard URI: vmess://uuid@host:port?params#name
	if strings.Contains(raw, "@") {
		u, err := url.Parse(s)
		if err != nil {
			return nil, err
		}
		n := &Node{
			Protocol: ProtoVMess,
			Address:  u.Hostname(),
			Port:     atoiSafe(u.Port()),
			UUID:     u.User.Username(),
			Name:     decodeFragment(u.Fragment),
			Security: "auto",
		}
		q := u.Query()
		fillTransport(n, q)
		fillTLS(n, q)
		return n, nil
	}

	// Legacy base64 JSON
	b, err := b64Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("vmess base64: %w", err)
	}
	var v vmessJSON
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("vmess json: %w", err)
	}
	n := &Node{
		Protocol:    ProtoVMess,
		Name:        v.PS,
		Address:     v.Add,
		Port:        anyInt(v.Port),
		UUID:        v.ID,
		AlterID:     anyInt(v.Aid),
		Security:    orDefault(v.Scy, "auto"),
		Network:     normalNet(v.Net),
		Host:        v.Host,
		Path:        v.Path,
		TLS:         v.TLS,
		SNI:         v.SNI,
		ALPN:        v.ALPN,
		Fingerprint: v.FP,
	}
	return n, nil
}

// ── VLESS ──────────────────────────────────────────────────────────────────

func parseVLESS(s string) (*Node, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Protocol:   ProtoVLESS,
		Address:    u.Hostname(),
		Port:       atoiSafe(u.Port()),
		UUID:       u.User.Username(),
		Name:       decodeFragment(u.Fragment),
		Encryption: "none",
	}
	q := u.Query()
	n.Flow = q.Get("flow")
	if enc := q.Get("encryption"); enc != "" {
		n.Encryption = enc
	}
	fillTransport(n, q)
	fillTLS(n, q)
	n.PublicKey = qVal(q, "pbk")
	n.ShortID   = qVal(q, "sid")
	n.SpiderX   = qVal(q, "spx")
	return n, nil
}

// ── Trojan ─────────────────────────────────────────────────────────────────

func parseTrojan(s string) (*Node, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Protocol: ProtoTrojan,
		Address:  u.Hostname(),
		Port:     atoiSafe(u.Port()),
		Password: userDecode(u),
		Name:     decodeFragment(u.Fragment),
		TLS:      "tls",
	}
	q := u.Query()
	n.Flow = q.Get("flow")
	fillTransport(n, q)
	fillTLS(n, q)
	if n.TLS == "" {
		n.TLS = "tls"
	}
	return n, nil
}

// ── Shadowsocks ────────────────────────────────────────────────────────────

func parseSS(s string) (*Node, error) {
	raw := s[len("ss://"):]
	name := ""
	if idx := strings.Index(raw, "#"); idx >= 0 {
		name, _ = url.PathUnescape(raw[idx+1:])
		raw = raw[:idx]
	}
	// ss://BASE64(method:pass)@host:port
	if idx := strings.LastIndex(raw, "@"); idx >= 0 {
		userPart := raw[:idx]
		hostPart := raw[idx+1:]
		// userPart might be base64 or plain method:pass
		plain := userPart
		if dec, err := b64Decode(userPart); err == nil {
			plain = string(dec)
		}
		parts := strings.SplitN(plain, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("ss: bad userinfo")
		}
		u, err := url.Parse("ss://" + hostPart)
		if err != nil {
			return nil, err
		}
		return &Node{
			Protocol: ProtoSS,
			Name:     name,
			Address:  u.Hostname(),
			Port:     atoiSafe(u.Port()),
			Method:   parts[0],
			Password: parts[1],
		}, nil
	}
	// Legacy full-base64
	dec, err := b64Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("ss base64: %w", err)
	}
	u, err := url.Parse("ss://" + string(dec))
	if err != nil {
		return nil, err
	}
	pass, _ := u.User.Password()
	return &Node{
		Protocol: ProtoSS,
		Name:     name,
		Address:  u.Hostname(),
		Port:     atoiSafe(u.Port()),
		Method:   u.User.Username(),
		Password: pass,
	}, nil
}

// ── TUIC ───────────────────────────────────────────────────────────────────

func parseTUIC(s string) (*Node, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Protocol: ProtoTUIC,
		Address:  u.Hostname(),
		Port:     atoiSafe(u.Port()),
		Name:     decodeFragment(u.Fragment),
		TLS:      "tls",
	}
	// user:pass encodes uuid:password
	info, _ := url.PathUnescape(u.User.String())
	if idx := strings.Index(info, ":"); idx >= 0 {
		n.UUID     = info[:idx]
		n.Password = info[idx+1:]
	} else {
		n.UUID = info
	}
	q := u.Query()
	n.CongestionControl = q.Get("congestion_control")
	fillTLS(n, q)
	if n.TLS == "" {
		n.TLS = "tls"
	}
	return n, nil
}

// ── Hysteria2 ──────────────────────────────────────────────────────────────

func parseHysteria2(s string) (*Node, error) {
	if strings.HasPrefix(strings.ToLower(s), "hy2://") {
		s = "hysteria2://" + s[6:]
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Protocol: ProtoHysteria2,
		Address:  u.Hostname(),
		Port:     atoiSafe(u.Port()),
		Password: userDecode(u),
		Name:     decodeFragment(u.Fragment),
		TLS:      "tls",
	}
	q := u.Query()
	fillTLS(n, q)
	if n.TLS == "" {
		n.TLS = "tls"
	}
	if v := qVal(q, "pinSHA256"); v != "" {
		n.PinSHA256 = v
	}
	if obfs := q.Get("obfs"); obfs != "" {
		n.ObfsType     = obfs
		n.ObfsPassword = qVal(q, "obfs-password")
	}
	if mp := qVal(q, "mport"); mp != "" {
		n.Ports = strings.ReplaceAll(mp, "-", ":")
	}
	return n, nil
}

// ── Shared helpers ─────────────────────────────────────────────────────────

func fillTransport(n *Node, q url.Values) {
	n.Network = normalNet(q.Get("type"))
	switch n.Network {
	case "ws", "httpupgrade":
		n.Host = qVal(q, "host")
		n.Path = qVal(q, "path")
	case "grpc":
		n.GrpcSvc  = qVal(q, "serviceName")
		// n.GrpcMode = q.Get("mode")
		n.Host     = qVal(q, "authority")
	case "http":
		n.Host = qVal(q, "host")
		n.Path = qVal(q, "path")
	case "xhttp":
		n.Host = qVal(q, "host")
		n.Path = qVal(q, "path")
	}
}

func fillTLS(n *Node, q url.Values) {
	sec := q.Get("security")
	if sec == "tls" || sec == "reality" {
		n.TLS = sec
	}
	if v := qVal(q, "sni"); v != "" {
		n.SNI = v
	}
	if v := qVal(q, "fp"); v != "" {
		n.Fingerprint = v
	}
	if v := qVal(q, "alpn"); v != "" {
		n.ALPN = v
	}
	if q.Get("allowInsecure") == "1" || q.Get("insecure") == "1" {
		n.Insecure = true
	}
}

func normalNet(s string) string {
	switch strings.ToLower(s) {
	case "tcp", "raw", "":
		return "tcp"
	case "ws":
		return "ws"
	case "grpc":
		return "grpc"
	case "h2", "http":
		return "http"
	case "httpupgrade":
		return "httpupgrade"
	case "xhttp", "splithttp":
		return "xhttp"
	default:
		return s
	}
}

func b64Decode(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func anyInt(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	case int:
		return x
	}
	return 0
}

func userDecode(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	s, _ := url.PathUnescape(u.User.Username())
	return s
}

func qVal(q url.Values, key string) string {
	v := q.Get(key)
	if v == "" {
		return ""
	}
	dec, err := url.QueryUnescape(v)
	if err != nil {
		return v
	}
	return dec
}

func decodeFragment(s string) string {
	dec, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return dec
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── WireGuard ──────────────────────────────────────────────────────────────
// wireguard://privateKey@server:port?publicKey=xxx&ip=10.0.0.2/32&mtu=1420&presharedKey=xxx&reserved=0,0,0#name

func parseWireGuard(s string) (*Node, error) {
	// normalise scheme so url.Parse works
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "wg://") {
		s = "wireguard://" + s[5:]
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Protocol:     ProtoWireGuard,
		Address:      u.Hostname(),
		Port:         atoiSafe(u.Port()),
		Name:         decodeFragment(u.Fragment),
		WGPrivateKey: u.User.Username(),
	}
	if n.Name == "" {
		n.Name = n.Address
	}
	q := u.Query()
	n.WGPublicKey = q.Get("publicKey")
	if n.WGPublicKey == "" {
		n.WGPublicKey = q.Get("pub")
	}
	if ip := q.Get("ip"); ip != "" {
		for _, part := range strings.Split(ip, ",") {
			if t := strings.TrimSpace(part); t != "" {
				n.WGIP = append(n.WGIP, t)
			}
		}
	}
	if mtu := q.Get("mtu"); mtu != "" {
		n.WGMTU = atoiSafe(mtu)
	}
	n.WGPresharedKey = q.Get("presharedKey")
	if reserved := q.Get("reserved"); reserved != "" {
		for _, part := range strings.Split(reserved, ",") {
			n.WGReserved = append(n.WGReserved, atoiSafe(strings.TrimSpace(part)))
		}
	}
	return n, nil
}

// ── SOCKS5 ────────────────────────────────────────────────────────────────
// socks5://user:pass@host:port#name  or  socks://...

func parseSOCKS5(s string) (*Node, error) {
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "socks://") {
		s = "socks5://" + s[8:]
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Protocol: ProtoSOCKS5,
		Address:  u.Hostname(),
		Port:     atoiSafe(u.Port()),
		Name:     decodeFragment(u.Fragment),
	}
	if n.Name == "" {
		n.Name = n.Address
	}
	if u.User != nil {
		n.Username, _ = url.PathUnescape(u.User.Username())
		n.Password, _ = u.User.Password()
	}
	q := u.Query()
	if q.Get("tls") == "1" || q.Get("security") == "tls" {
		n.TLS = "tls"
		n.SNI = q.Get("sni")
		if q.Get("allowInsecure") == "1" || q.Get("insecure") == "1" {
			n.Insecure = true
		}
	}
	return n, nil
}

// ── HTTP / HTTPS ──────────────────────────────────────────────────────────
// http://user:pass@host:port#name   https://user:pass@host:port#name

func parseHTTPProxy(s string, https bool) (*Node, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Protocol: ProtoHTTP,
		Address:  u.Hostname(),
		Port:     atoiSafe(u.Port()),
		Name:     decodeFragment(u.Fragment),
		HTTPS:    https,
	}
	if n.Name == "" {
		n.Name = n.Address
	}
	if u.User != nil {
		n.Username, _ = url.PathUnescape(u.User.Username())
		n.Password, _ = u.User.Password()
	}
	if https {
		n.TLS = "tls"
		q := u.Query()
		n.SNI = q.Get("sni")
		if q.Get("allowInsecure") == "1" || q.Get("insecure") == "1" {
			n.Insecure = true
		}
	}
	return n, nil
}
