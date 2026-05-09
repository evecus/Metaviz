package node

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Protocol string

const (
	ProtoVMess     Protocol = "vmess"
	ProtoVLESS     Protocol = "vless"
	ProtoTrojan    Protocol = "trojan"
	ProtoSS        Protocol = "ss"
	ProtoTUIC      Protocol = "tuic"
	ProtoHysteria2 Protocol = "hysteria2"
	ProtoWireGuard Protocol = "wireguard"
	ProtoSOCKS5    Protocol = "socks5"
	ProtoHTTP      Protocol = "http"
)

type Node struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Protocol Protocol `json:"protocol"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`

	// Auth
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	Method   string `json:"method,omitempty"`

	// VMess
	AlterID  int    `json:"alterId,omitempty"`
	Security string `json:"security,omitempty"`

	// VLESS / Trojan
	Flow       string `json:"flow,omitempty"`
	Encryption string `json:"encryption,omitempty"`

	// Transport
	Network  string `json:"network,omitempty"`
	Path     string `json:"path,omitempty"`
	Host     string `json:"host,omitempty"`
	GrpcSvc  string `json:"grpcSvc,omitempty"`

	// TLS
	TLS         string `json:"tls,omitempty"`
	SNI         string `json:"sni,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	ALPN        string `json:"alpn,omitempty"`
	Insecure    bool   `json:"insecure,omitempty"`

	// Reality
	PublicKey string `json:"publicKey,omitempty"`
	ShortID   string `json:"shortId,omitempty"`
	SpiderX   string `json:"spiderX,omitempty"`

	// TUIC
	CongestionControl string `json:"congestionControl,omitempty"`

	// Hysteria2
	ObfsType     string `json:"obfsType,omitempty"`
	ObfsPassword string `json:"obfsPassword,omitempty"`
	Ports        string `json:"ports,omitempty"`
	PinSHA256    string `json:"pinSHA256,omitempty"`

	// WireGuard
	WGPrivateKey string   `json:"wgPrivateKey,omitempty"`
	WGPublicKey  string   `json:"wgPublicKey,omitempty"`
	WGIP         []string `json:"wgIp,omitempty"`          // 本机 tunnel IP，如 ["10.0.0.2/32"]
	WGMTU        int      `json:"wgMtu,omitempty"`
	WGPresharedKey string `json:"wgPresharedKey,omitempty"`
	WGReserved   []int    `json:"wgReserved,omitempty"`    // Cloudflare WARP [R,R,R]

	// SOCKS5 / HTTP(S)
	Username string `json:"username,omitempty"` // socks5/http 用户名（区别于 UUID）
	HTTPS    bool   `json:"https,omitempty"`    // http 代理是否启用 TLS（即 HTTPS）
}

func NewID() string { return uuid.New().String() }

// FromMap constructs a Node from a mihomo proxy map (YAML unmarshalled map).
func FromMap(m map[string]any) (*Node, error) {
	str := func(key string) string { v, _ := m[key].(string); return v }
	intVal := func(key string) int {
		switch v := m[key].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
		return 0
	}

	typ := str("type")
	var proto Protocol
	switch typ {
	case "vmess":
		proto = ProtoVMess
	case "vless":
		proto = ProtoVLESS
	case "trojan":
		proto = ProtoTrojan
	case "ss", "shadowsocks":
		proto = ProtoSS
	case "tuic":
		proto = ProtoTUIC
	case "hysteria2":
		proto = ProtoHysteria2
	case "wireguard":
		proto = ProtoWireGuard
	case "socks5":
		proto = ProtoSOCKS5
	case "http":
		proto = ProtoHTTP
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", typ)
	}

	n := &Node{
		ID:       NewID(),
		Name:     str("name"),
		Protocol: proto,
		Address:  str("server"),
		Port:     intVal("port"),
		UUID:     str("uuid"),
		Password: str("password"),
		Method:   str("cipher"),
		AlterID:  intVal("alterId"),
		Security: str("cipher"),
		Flow:     str("flow"),
	}

	// TLS
	if tls, ok := m["tls"].(bool); ok && tls {
		n.TLS = "tls"
		n.SNI, _ = m["servername"].(string)
		n.Insecure, _ = m["skip-cert-verify"].(bool)
		n.Fingerprint, _ = m["fingerprint"].(string)
		if alpn, ok := m["alpn"].([]interface{}); ok {
			parts := make([]string, 0, len(alpn))
			for _, a := range alpn {
				if s, ok := a.(string); ok {
					parts = append(parts, s)
				}
			}
			n.ALPN = strings.Join(parts, ",")
		}
		if realOpts, ok := m["reality-opts"].(map[string]any); ok {
			n.TLS = "reality"
			n.PublicKey, _ = realOpts["public-key"].(string)
			n.ShortID, _ = realOpts["short-id"].(string)
		}
	}

	// Transport (ws-opts / grpc-opts / httpupgrade-opts)
	if net, ok := m["network"].(string); ok && net != "" {
		n.Network = net
		switch net {
		case "ws":
			if opts, ok := m["ws-opts"].(map[string]any); ok {
				n.Path, _ = opts["path"].(string)
				if hdrs, ok := opts["headers"].(map[string]any); ok {
					n.Host, _ = hdrs["Host"].(string)
				}
			}
		case "grpc":
			if opts, ok := m["grpc-opts"].(map[string]any); ok {
				n.GrpcSvc, _ = opts["grpc-service-name"].(string)
			}
		case "httpupgrade", "xhttp":
			if opts, ok := m["httpupgrade-opts"].(map[string]any); ok {
				n.Host, _ = opts["host"].(string)
				n.Path, _ = opts["path"].(string)
			}
			if opts, ok := m["xhttp-opts"].(map[string]any); ok {
				n.Host, _ = opts["host"].(string)
				n.Path, _ = opts["path"].(string)
			}
		}
	}

	// TUIC
	n.CongestionControl, _ = m["congestion-controller"].(string)

	// Hysteria2
	n.ObfsType, _ = m["obfs"].(string)
	n.ObfsPassword, _ = m["obfs-password"].(string)
	n.Ports, _ = m["ports"].(string)

	// WireGuard
	n.WGPrivateKey, _ = m["private-key"].(string)
	n.WGPublicKey, _ = m["public-key"].(string)
	if mtu, ok := m["mtu"].(int); ok {
		n.WGMTU = mtu
	} else if mtuF, ok := m["mtu"].(float64); ok {
		n.WGMTU = int(mtuF)
	}
	n.WGPresharedKey, _ = m["preshared-key"].(string)
	if ipRaw, ok := m["ip"].(string); ok && ipRaw != "" {
		n.WGIP = strings.Split(ipRaw, ",")
		for i, s := range n.WGIP {
			n.WGIP[i] = strings.TrimSpace(s)
		}
	}
	if reserved, ok := m["reserved"].([]interface{}); ok {
		for _, v := range reserved {
			if f, ok := v.(float64); ok {
				n.WGReserved = append(n.WGReserved, int(f))
			}
		}
	}

	// SOCKS5 / HTTP(S)
	n.Username = str("username")
	if tls, ok := m["tls"].(bool); ok {
		n.HTTPS = tls
	}

	return n, nil
}
