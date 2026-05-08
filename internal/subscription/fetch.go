package subscription

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/metaviz/internal/node"
)

// Fetch downloads a subscription URL and returns a list of mihomo proxy maps.
// Supports:
//   - mihomo YAML (proxies: [...])
//   - base64-encoded share links
//   - plain share links (one per line)
func Fetch(url string) ([]map[string]any, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return parse(strings.TrimSpace(string(body)))
}

func parse(body string) ([]map[string]any, error) {
	// 1. mihomo YAML: contains "proxies:" key
	if strings.Contains(body, "proxies:") {
		var cfg struct {
			Proxies []map[string]any `yaml:"proxies"`
		}
		if err := yaml.Unmarshal([]byte(body), &cfg); err == nil && len(cfg.Proxies) > 0 {
			return filterProxies(cfg.Proxies), nil
		}
	}

	// 2. Base64-encoded share links
	if decoded, err := tryBase64(body); err == nil {
		return parseLinks(decoded)
	}

	// 3. Raw share links
	return parseLinks(body)
}

func tryBase64(s string) (string, error) {
	pad := (4 - len(s)%4) % 4
	s = s + strings.Repeat("=", pad)
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(s)
	}
	if err != nil {
		return "", err
	}
	decoded := string(b)
	if !strings.Contains(decoded, "://") && !strings.Contains(decoded, "proxies:") {
		return "", fmt.Errorf("not share-link base64")
	}
	return decoded, nil
}

func parseLinks(text string) ([]map[string]any, error) {
	nodes, errs := node.ParseLinks(text)
	if len(nodes) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("no valid nodes: %s", strings.Join(errs, "; "))
	}
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		m := nodeToMap(n)
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no supported nodes found")
	}
	return out, nil
}

// nodeToMap converts a Node to a simple map for storage in the cache.
func nodeToMap(n *node.Node) map[string]any {
	m := map[string]any{
		"name":   n.Name,
		"type":   string(n.Protocol),
		"server": n.Address,
		"port":   n.Port,
	}
	if n.UUID != "" {
		m["uuid"] = n.UUID
	}
	if n.Password != "" {
		m["password"] = n.Password
	}
	if n.Method != "" {
		m["cipher"] = n.Method
	}
	if n.AlterID != 0 {
		m["alterId"] = n.AlterID
	}
	if n.Flow != "" {
		m["flow"] = n.Flow
	}
	if n.Network != "" {
		m["network"] = n.Network
	}
	if n.TLS != "" {
		m["tls"] = true
	}
	if n.SNI != "" {
		m["servername"] = n.SNI
	}
	if n.Insecure {
		m["skip-cert-verify"] = true
	}
	return m
}

var nonProxyTypes = map[string]bool{
	"direct": true, "block": true, "dns": true,
	"selector": true, "url-test": true, "fallback": true, "load-balance": true,
}

func filterProxies(obs []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(obs))
	for _, ob := range obs {
		t, _ := ob["type"].(string)
		if t == "" || nonProxyTypes[t] {
			continue
		}
		out = append(out, ob)
	}
	return out
}
