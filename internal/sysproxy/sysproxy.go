package sysproxy

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const envFile = "/etc/environment"

var proxyKeys = []string{
	"http_proxy", "HTTP_PROXY",
	"https_proxy", "HTTPS_PROXY",
	"all_proxy", "ALL_PROXY",
}

// Set writes proxy environment variables to /etc/environment.
// Existing proxy lines are replaced; all other lines are preserved.
func Set(port int) error {
	http := fmt.Sprintf("http://127.0.0.1:%d", port)
	socks := fmt.Sprintf("socks5://127.0.0.1:%d", port)

	lines, err := readLines()
	if err != nil {
		return err
	}
	lines = removeProxyLines(lines)
	lines = append(lines,
		fmt.Sprintf("http_proxy=%s", http),
		fmt.Sprintf("HTTP_PROXY=%s", http),
		fmt.Sprintf("https_proxy=%s", http),
		fmt.Sprintf("HTTPS_PROXY=%s", http),
		fmt.Sprintf("all_proxy=%s", socks),
		fmt.Sprintf("ALL_PROXY=%s", socks),
	)
	return writeLines(lines)
}

// Clear removes proxy environment variables from /etc/environment.
func Clear() error {
	lines, err := readLines()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines = removeProxyLines(lines)
	return writeLines(lines)
}

func removeProxyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		key := strings.SplitN(l, "=", 2)[0]
		key = strings.TrimSpace(key)
		if !isProxyKey(key) {
			out = append(out, l)
		}
	}
	return out
}

func isProxyKey(key string) bool {
	for _, k := range proxyKeys {
		if strings.EqualFold(key, k) {
			return true
		}
	}
	return false
}

func readLines() ([]string, error) {
	f, err := os.Open(envFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func writeLines(lines []string) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	return os.WriteFile(envFile, []byte(content), 0644)
}
