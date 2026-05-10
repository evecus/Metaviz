package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Result holds the outcome of updating a single file.
type Result struct {
	File  string `json:"file"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// mrsFiles maps local filename → download URL.
var mrsFiles = map[string]string{
	"geosite-cn.mrs":              "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/meta/geo/geosite/cn.mrs",
	"geoip-cn.mrs":                "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/meta/geo/geoip/cn.mrs",
	"geosite-gfw.mrs":             "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/meta/geo/geosite/gfw.mrs",
	"geosite-geolocation-!cn.mrs": "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/meta/geo/geosite/geolocation-!cn.mrs",
	"geoip-telegram.mrs":          "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/meta/geo/geoip/telegram.mrs",
	"ads.mrs":                     "https://raw.githubusercontent.com/evecus/ruleset/refs/heads/master/mihomo/ads.mrs",
	// fakeip 过滤规则集：域名黑名单，这些域名不应分配 fakeip（NTP、STUN、本地服务等）
	"fakeipfilter.mrs": "https://raw.githubusercontent.com/wwqgtxx/clash-rules/release/fakeip-filter.mrs",
}

// UpdateAll downloads all known mrs files to mrsDir.
func UpdateAll(mrsDir, proxy string) []Result {
	results := make([]Result, 0, len(mrsFiles))
	for file, url := range mrsFiles {
		err := downloadTo(url, filepath.Join(mrsDir, file), proxy)
		r := Result{File: file, OK: err == nil}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results
}

// UpdateFiles downloads only the specified files.
type FileSpec struct {
	File string
	URL  string // if empty, falls back to built-in URL
}

func UpdateFiles(mrsDir, proxy string, specs []FileSpec) []Result {
	results := make([]Result, 0, len(specs))
	for _, spec := range specs {
		url := spec.URL
		if url == "" {
			url = mrsFiles[spec.File]
		}
		if url == "" {
			results = append(results, Result{File: spec.File, OK: false, Error: "unknown file, no URL"})
			continue
		}
		err := downloadTo(url, filepath.Join(mrsDir, spec.File), proxy)
		r := Result{File: spec.File, OK: err == nil}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results
}

func downloadTo(url, dst, proxy string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	return os.Rename(tmp, dst)
}

// KnownFiles returns the list of built-in mrs filenames.
func KnownFiles() []string {
	names := make([]string, 0, len(mrsFiles))
	for k := range mrsFiles {
		names = append(names, k)
	}
	return names
}
