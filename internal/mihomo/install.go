package mihomo

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const MihomoInstallPath = "/usr/bin/mihomo"

type SystemInfo struct {
	Arch   string `json:"arch"`
	LibC   string `json:"libc"`
	OSName string `json:"osName"`
}

func DetectSystem() SystemInfo {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	case "arm":
		arch = "armv7"
	case "386":
		arch = "386"
	}
	libc := "musl"
	if _, err := exec.LookPath("ldd"); err == nil {
		out, _ := exec.Command("ldd", "--version").CombinedOutput()
		if strings.Contains(strings.ToLower(string(out)), "gnu") {
			libc = "gnu"
		}
	}
	osName := "linux"
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				osName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
				break
			}
		}
	}
	return SystemInfo{Arch: arch, LibC: libc, OSName: osName}
}

func Version() string {
	out, err := exec.Command(MihomoInstallPath, "-v").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Split(string(out), "\n")[0])
}

func Install(proxy, version string) (string, error) {
	sys := DetectSystem()
	tag := version
	if tag == "" || tag == "latest" {
		var err error
		tag, err = fetchLatestTag(proxy)
		if err != nil {
			return "", fmt.Errorf("fetch latest version: %w", err)
		}
	} else if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	assetName := fmt.Sprintf("mihomo-linux-%s-%s.gz", sys.Arch, tag)
	url := fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/%s", tag, assetName)

	tmp, err := downloadFile(url, proxy)
	if err != nil {
		// fallback: try tar.gz
		assetName = fmt.Sprintf("mihomo-linux-%s-%s.tar.gz", sys.Arch, tag)
		url = fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/%s", tag, assetName)
		tmp, err = downloadFile(url, proxy)
		if err != nil {
			return "", fmt.Errorf("download: %w", err)
		}
	}
	defer os.Remove(tmp)

	bin, err := extractBinary(tmp, assetName)
	if err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}
	defer os.Remove(bin)

	if err := os.Chmod(bin, 0755); err != nil {
		return "", err
	}
	if err := os.Rename(bin, MihomoInstallPath); err != nil {
		if err2 := copyExec(bin, MihomoInstallPath); err2 != nil {
			return "", fmt.Errorf("install binary: %w", err)
		}
	}
	return Version(), nil
}

func fetchLatestTag(proxy string) (string, error) {
	client := makeHTTPClient(proxy)
	resp, err := client.Get("https://api.github.com/repos/MetaCubeX/mihomo/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	idx := strings.Index(s, `"tag_name"`)
	if idx < 0 {
		return "", fmt.Errorf("tag_name not found")
	}
	s = s[idx+len(`"tag_name"`):]
	s = s[strings.Index(s, `"`)+1:]
	return s[:strings.Index(s, `"`)], nil
}

func downloadFile(url, proxy string) (string, error) {
	client := makeHTTPClient(proxy)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.CreateTemp("", "mihomo-dl-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func extractBinary(src, assetName string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()
	out, err := os.CreateTemp("", "mihomo-bin-*")
	if err != nil {
		return "", err
	}
	out.Close()

	if strings.HasSuffix(assetName, ".tar.gz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return "", err
		}
		tr := tar.NewReader(gr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}
			if strings.Contains(hdr.Name, "mihomo") && !strings.HasSuffix(hdr.Name, "/") {
				outf, _ := os.Create(out.Name())
				io.Copy(outf, tr)
				outf.Close()
				return out.Name(), nil
			}
		}
		return "", fmt.Errorf("binary not found in tar")
	}
	// .gz
	f.Seek(0, 0)
	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gr.Close()
	outf, _ := os.Create(out.Name())
	defer outf.Close()
	io.Copy(outf, gr)
	return out.Name(), nil
}

func copyExec(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func makeHTTPClient(proxy string) *http.Client {
	return &http.Client{Timeout: 120 * time.Second}
}
