package builder

import (
	"fmt"
	"strings"

	"github.com/metaviz/internal/node"
)

// NodeToProxy converts a Node to a mihomo proxy map (YAML-serializable).
func NodeToProxy(n *node.Node) (map[string]interface{}, error) {
	switch n.Protocol {
	case node.ProtoVMess:
		return vmessProxy(n)
	case node.ProtoVLESS:
		return vlessProxy(n)
	case node.ProtoTrojan:
		return trojanProxy(n)
	case node.ProtoSS:
		return ssProxy(n)
	case node.ProtoTUIC:
		return tuicProxy(n)
	case node.ProtoHysteria2:
		return hy2Proxy(n)
	case node.ProtoWireGuard:
		return wireguardProxy(n)
	case node.ProtoSOCKS5:
		return socks5Proxy(n)
	case node.ProtoHTTP:
		return httpProxy(n)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", n.Protocol)
	}
}

func vmessProxy(n *node.Node) (map[string]interface{}, error) {
	m := M{
		"name":      n.Name,
		"type":      "vmess",
		"server":    n.Address,
		"port":      n.Port,
		"uuid":      n.UUID,
		"alterId":   n.AlterID,
		"cipher":    nonEmpty(n.Security, "auto"),
		"udp":       true,
	}
	setTransport(m, n)
	setTLS(m, n)
	return m, nil
}

func vlessProxy(n *node.Node) (map[string]interface{}, error) {
	m := M{
		"name":   n.Name,
		"type":   "vless",
		"server": n.Address,
		"port":   n.Port,
		"uuid":   n.UUID,
		"udp":    true,
	}
	if n.Flow != "" {
		m["flow"] = n.Flow
	}
	setTransport(m, n)
	setTLS(m, n)
	return m, nil
}

func trojanProxy(n *node.Node) (map[string]interface{}, error) {
	m := M{
		"name":     n.Name,
		"type":     "trojan",
		"server":   n.Address,
		"port":     n.Port,
		"password": n.Password,
		"udp":      true,
	}
	if n.Flow != "" {
		m["flow"] = n.Flow
	}
	setTransport(m, n)
	setTLS(m, n)
	return m, nil
}

func ssProxy(n *node.Node) (map[string]interface{}, error) {
	return M{
		"name":     n.Name,
		"type":     "ss",
		"server":   n.Address,
		"port":     n.Port,
		"cipher":   n.Method,
		"password": n.Password,
		"udp":      true,
	}, nil
}

func tuicProxy(n *node.Node) (map[string]interface{}, error) {
	m := M{
		"name":               n.Name,
		"type":               "tuic",
		"server":             n.Address,
		"port":               n.Port,
		"uuid":               n.UUID,
		"password":           n.Password,
		"congestion-controller": nonEmpty(n.CongestionControl, "bbr"),
		"udp-relay-mode":     "native",
		"alpn":               []string{"h3"},
	}
	setTLS(m, n)
	return m, nil
}

func hy2Proxy(n *node.Node) (map[string]interface{}, error) {
	m := M{
		"name":             n.Name,
		"type":             "hysteria2",
		"server":           n.Address,
		"port":             n.Port,
		"password":         n.Password,
		"sni":              nonEmpty(n.SNI, n.Address),
		"skip-cert-verify": n.Insecure,
		"tfo":              false,
	}
	if n.Ports != "" {
		m["ports"] = n.Ports
	}
	if n.ObfsType != "" {
		m["obfs"] = n.ObfsType
		m["obfs-password"] = n.ObfsPassword
	}
	if n.Fingerprint != "" {
		m["fingerprint"] = n.Fingerprint
	}
	if n.ALPN != "" {
		m["alpn"] = strings.Split(n.ALPN, ",")
	}
	return m, nil
}

// ── TLS ────────────────────────────────────────────────────────────────────

func setTLS(m M, n *node.Node) {
	if n.TLS == "" {
		return
	}
	m["tls"] = true
	if n.SNI != "" {
		m["servername"] = n.SNI
	}
	if n.Insecure {
		m["skip-cert-verify"] = true
	}
	if n.Fingerprint != "" {
		m["fingerprint"] = n.Fingerprint
	}
	if n.ALPN != "" {
		m["alpn"] = strings.Split(n.ALPN, ",")
	}
	if n.TLS == "reality" {
		m["reality-opts"] = M{
			"public-key": n.PublicKey,
			"short-id":   n.ShortID,
		}
		delete(m, "skip-cert-verify")
	}
}

// ── Transport ──────────────────────────────────────────────────────────────

func setTransport(m M, n *node.Node) {
	if n.Network == "" || n.Network == "tcp" {
		return
	}
	m["network"] = n.Network
	switch n.Network {
	case "ws":
		opts := M{}
		if n.Path != "" {
			opts["path"] = n.Path
		}
		if n.Host != "" {
			opts["headers"] = M{"Host": n.Host}
		}
		m["ws-opts"] = opts
	case "grpc":
		opts := M{}
		if n.GrpcSvc != "" {
			opts["grpc-service-name"] = n.GrpcSvc
		}
		m["grpc-opts"] = opts
	case "http":
		opts := M{}
		if n.Host != "" {
			opts["host"] = []string{n.Host}
		}
		if n.Path != "" {
			opts["path"] = n.Path
		}
		m["http-opts"] = opts
	case "httpupgrade":
		opts := M{}
		if n.Host != "" {
			opts["host"] = n.Host
		}
		if n.Path != "" {
			opts["path"] = n.Path
		}
		m["httpupgrade-opts"] = opts
	case "xhttp", "splithttp":
		m["network"] = "xhttp"
		opts := M{}
		if n.Host != "" {
			opts["host"] = n.Host
		}
		if n.Path != "" {
			opts["path"] = n.Path
		}
		m["xhttp-opts"] = opts
	}
}

type M = map[string]interface{}

func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ── WireGuard ──────────────────────────────────────────────────────────────

func wireguardProxy(n *node.Node) (map[string]interface{}, error) {
	m := M{
		"name":        n.Name,
		"type":        "wireguard",
		"server":      n.Address,
		"port":        n.Port,
		"private-key": n.WGPrivateKey,
		"public-key":  n.WGPublicKey,
		"udp":         true,
	}
	if len(n.WGIP) > 0 {
		m["ip"] = strings.Join(n.WGIP, ", ")
	}
	if n.WGMTU > 0 {
		m["mtu"] = n.WGMTU
	}
	if n.WGPresharedKey != "" {
		m["preshared-key"] = n.WGPresharedKey
	}
	if len(n.WGReserved) > 0 {
		m["reserved"] = n.WGReserved
	}
	return m, nil
}

// ── SOCKS5 ────────────────────────────────────────────────────────────────

func socks5Proxy(n *node.Node) (map[string]interface{}, error) {
	m := M{
		"name":   n.Name,
		"type":   "socks5",
		"server": n.Address,
		"port":   n.Port,
		"udp":    true,
	}
	if n.Username != "" {
		m["username"] = n.Username
		m["password"] = n.Password
	}
	if n.TLS == "tls" {
		m["tls"] = true
		if n.SNI != "" {
			m["sni"] = n.SNI
		}
		if n.Insecure {
			m["skip-cert-verify"] = true
		}
	}
	return m, nil
}

// ── HTTP / HTTPS ──────────────────────────────────────────────────────────

func httpProxy(n *node.Node) (map[string]interface{}, error) {
	m := M{
		"name":   n.Name,
		"type":   "http",
		"server": n.Address,
		"port":   n.Port,
	}
	if n.Username != "" {
		m["username"] = n.Username
		m["password"] = n.Password
	}
	if n.HTTPS {
		m["tls"] = true
		if n.SNI != "" {
			m["sni"] = n.SNI
		}
		if n.Insecure {
			m["skip-cert-verify"] = true
		}
	}
	return m, nil
}
