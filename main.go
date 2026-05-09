package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/metaviz/internal/api"
	"github.com/metaviz/internal/core"
)

//go:embed web/dist
var webFS embed.FS

//go:embed assets/mrs
var mrsFS embed.FS

//go:embed assets/cn-bypass.nft
var cnBypassNft []byte

//go:embed assets/cn-bypass6.nft
var cnBypass6Nft []byte

func main() {
	var (
		dirFlag  string
		portFlag int
	)
	flag.StringVar(&dirFlag, "dir", "", "data directory (default: <exe-dir>/data)")
	flag.IntVar(&portFlag, "port", 8080, "web UI port")
	flag.Parse()

	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("executable path: %v", err)
	}
	baseDir := filepath.Dir(exe)

	dataDir := filepath.Join(baseDir, "data")
	if dirFlag != "" {
		abs, err := filepath.Abs(dirFlag)
		if err != nil {
			log.Fatalf("invalid --dir: %v", err)
		}
		dataDir = abs
	}

	listen := fmt.Sprintf(":%d", portFlag)

	runDir      := filepath.Join(dataDir, "run")
	mrsDir      := filepath.Join(runDir, "mrs")
	configsDir  := filepath.Join(dataDir, "configs")
	providersDir := filepath.Join(dataDir, "providers")

	for _, d := range []string{dataDir, runDir, mrsDir, configsDir, providersDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			log.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// 提取内嵌的 .mrs 规则集到 run/mrs/（已存在则跳过）
	if err := extractMRS(mrsFS, mrsDir); err != nil {
		log.Printf("warn: extract mrs: %v", err)
	}

	// 提取 cn-bypass.nft 和 cn-bypass6.nft（已存在则跳过）
	cnNftDst := filepath.Join(dataDir, "cn-bypass.nft")
	if _, statErr := os.Stat(cnNftDst); os.IsNotExist(statErr) {
		if len(cnBypassNft) > 0 {
			if err := os.WriteFile(cnNftDst, cnBypassNft, 0644); err != nil {
				log.Printf("warn: extract cn-bypass.nft: %v", err)
			} else {
				log.Printf("metaviz: extracted cn-bypass.nft -> %s", cnNftDst)
			}
		}
	}
	cn6NftDst := filepath.Join(dataDir, "cn-bypass6.nft")
	if _, statErr := os.Stat(cn6NftDst); os.IsNotExist(statErr) {
		if len(cnBypass6Nft) > 0 {
			if err := os.WriteFile(cn6NftDst, cnBypass6Nft, 0644); err != nil {
				log.Printf("warn: extract cn-bypass6.nft: %v", err)
			} else {
				log.Printf("metaviz: extracted cn-bypass6.nft -> %s", cn6NftDst)
			}
		}
	}

	manager := core.NewManager(dataDir, runDir, mrsDir)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		sig := <-sigCh
		log.Printf("metaviz: signal %v — shutting down", sig)
		manager.Stop()
		os.Exit(0)
	}()

	manager.RecoverState()
	manager.AutoStart()

	srv := api.NewServer(manager, dataDir, mrsDir, webFS)
	log.Printf("metaviz: listening on %s  data=%s", listen, dataDir)
	if err := srv.Run(listen); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// extractMRS 只提取 .mrs 文件，跳过其他文件（README 等）
func extractMRS(efs embed.FS, dst string) error {
	entries, err := efs.ReadDir("assets/mrs")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mrs") {
			continue
		}
		target := filepath.Join(dst, e.Name())
		if _, err := os.Stat(target); err == nil {
			continue // 已存在，跳过
		}
		data, err := efs.ReadFile("assets/mrs/" + e.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
		log.Printf("metaviz: extracted mrs/%s", e.Name())
	}
	return nil
}
