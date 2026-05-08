# singa

基于 sing-box 的透明代理管理面板。nftables 规则移植自 [v2rayA](https://github.com/v2rayA/v2rayA)，路由规则参考 v2rayN sing-box 模板。

## 功能

| 功能 | 说明 |
|------|------|
| **节点模式** | 粘贴分享链接，面板自动生成完整 config |
| **上传模式** | 上传自己写好的 config.json |
| **透明代理** | tproxy / redirect / tun / system_proxy |
| **路由模式** | 绕过大陆 / 仅代理 GFW / 全局代理 |
| **局域网代理** | 自动开启 ip_forward，代理局域网设备 |
| **IPv6** | 可选，下发 ip6 规则 |
| **实时日志** | SSE 日志流，带颜色分级 |

## 支持的节点协议

`vmess` · `vless` · `trojan` · `shadowsocks` · `tuic` · `hysteria2 (hy2)`

含完整 TLS/Reality/transport 字段支持（ws/grpc/http/httpupgrade/xhttp）。

## 使用

### 前置要求

- Linux，root 权限
- `/usr/bin/sing-box` 已安装
- `nft`（nftables）可用

### 运行

```bash
chmod +x singa-linux-amd64
sudo ./singa-linux-amd64
# 面板: http://localhost:8080
```

### 目录

```
singa           ← 二进制
data/
  nodes.json    ← 持久化节点
  config.json   ← 上传模式的用户配置
  run/
    config.json ← 运行时配置（自动生成/复制）
  srs/          ← .srs 规则集文件（首次运行自动解压）
```

## 构建

```bash
# 1. 前端
cd web && npm install && npm run build && cd ..

# 2. 后端
go mod tidy
go build -trimpath -ldflags="-s -w" -o singa .
```

或推 tag 触发 GitHub Actions（自动下载 .srs 并编译 6 个平台）：

```bash
git tag v1.0.0 && git push origin v1.0.0
```

## 端口说明

每次启动随机分配（10000–59999 范围内的空闲端口）：

| 用途 | 说明 |
|------|------|
| DNS  | sing-box dns-in 监听端口 |
| Mixed | SOCKS5+HTTP 混合入站 |
| TProxy | tproxy 透明代理入站 |
| Redirect | redirect 透明代理入站 |

上传模式下端口从 config.json 的 inbound 解析获取。
