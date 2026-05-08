.PHONY: build web go clean

# 最终产物：单个可执行文件
build: web go

web:
	cd web && npm install && npm run build

go:
	GOPROXY=direct GONOSUMDB="* " GOFLAGS="-mod=mod" \
	go build -ldflags="-s -w" -o metaviz .

clean:
	rm -f metaviz
	rm -rf web/dist web/node_modules

# 安装到 /usr/local/bin（需要 root）
install: build
	install -m 755 metaviz /usr/local/bin/metaviz
	mkdir -p /etc/metaviz
	@echo "MetaViz installed. Run: metaviz --dir /var/lib/metaviz --port 7777"
