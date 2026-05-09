# MetaViz

基于 [mihomo](https://github.com/MetaCubeX/mihomo) 内核的 Linux 透明代理管理面板，提供 Web UI，支持节点导入、订阅管理、透明代理、路由规则配置。

---

## 系统要求

| 项目 | 要求 |
|------|------|
| 操作系统 | Linux（仅支持 Linux） |
| 架构 | amd64 / arm64 / armv7 / 386 |
| 内核 | 需支持 nftables（Linux 3.13+，推荐 5.4+） |
| 权限 | 需以 **root** 运行（配置 nftables / ip route 规则） |
| 依赖 | `nft`（nftables 命令行工具） |

---

## 安装

### 1. 下载 MetaViz

从 Releases 页面下载对应架构的二进制文件，例如：

```bash
# amd64
wget https://github.com/evecus/metaviz/releases/latest/download/metaviz-linux-amd64 -O metaviz
chmod +x metaviz
```

### 2. 安装 mihomo 内核

首次运行后，在 Web UI **仪表盘 → mihomo 内核** 处点击安装，MetaViz 会自动：

- 检测当前系统架构（amd64 / arm64 / armv7 / 386）
- 检测 libc 类型（glibc 或 musl）
- 从 GitHub Releases 下载对应版本：`mihomo-linux-<arch>-<version>.gz`
- 解压并安装到 `/usr/bin/mihomo`

> 也可以手动将 mihomo 二进制放到 `/usr/bin/mihomo` 并赋予可执行权限，MetaViz 会直接使用。

### 3. 启动

```bash
sudo ./metaviz
# 默认监听 :8080，数据目录 <可执行文件所在目录>/data

# 自定义端口和数据目录
sudo ./metaviz --port 9090 --dir /etc/metaviz/data
```

打开浏览器访问 `http://<IP>:8080`。

---

## 功能说明

### 仪表盘

选择配置模式后点击**启动**，mihomo 开始运行并配置透明代理规则。

| 配置模式 | 说明 |
|----------|------|
| 单节点 | 从导入节点或订阅节点中选择一个节点使用 |
| 订阅模式 | 使用完整订阅，生成 `proxy-providers`，由 mihomo 自动拉取节点 |
| 上传配置 | 使用自定义 YAML 配置文件，MetaViz 只覆盖端口、TUN、Sniffer 等全局字段 |

**路由模式：**

| 模式 | 说明 |
|------|------|
| 大陆白名单 | 国内域名/IP 直连，其余走代理 |
| GFW 列表 | GFW 封锁的域名/IP 走代理，1.1.1.1、8.8.8.8 强制走代理，其余直连 |
| 全局 | 所有流量走代理 |

---

### 节点与配置

**支持的节点协议（导入 / 单节点使用）：**

| 协议 | share link 格式 |
|------|----------------|
| VMess | `vmess://...`（标准 URI 或 base64 JSON） |
| VLESS | `vless://...`（支持 Reality、xhttp/splithttp） |
| Trojan | `trojan://...` |
| Shadowsocks | `ss://...` |
| TUIC v5 | `tuic://...` |
| Hysteria2 | `hysteria2://...` 或 `hy2://...` |
| WireGuard | `wireguard://privateKey@server:port?publicKey=xxx&ip=10.0.0.2/32&mtu=1420#name` |
| SOCKS5 | `socks5://user:pass@host:port#name` 或 `socks://...` |
| HTTP/HTTPS | `http://user:pass@host:port#name` / `https://...` |

**订阅：** 添加订阅 URL 后，MetaViz 自动拉取并解析节点。订阅模式下直接使用 `proxy-providers`，不在配置文件中展开节点列表。

---

### 设置

#### 代理配置

| 选项 | 说明 |
|------|------|
| TCP 代理模式 | 关闭 / Redirect（NAT 重定向，TCP only）/ TProxy（透明代理，TCP+UDP）/ TUN（虚拟网卡） |
| UDP 代理模式 | 关闭 / TProxy / TUN |
| 系统代理 | 开启后仅设置系统 HTTP/SOCKS5 代理地址，**不配置任何 nftables 规则和路由** |
| 局域网代理 | 允许局域网内其他设备通过本机代理 |
| IPv6 | 启用 IPv6 代理支持 |

> TCP 或 UDP 选择 **TUN** 模式时，TUN 虚拟网卡自动启用，无需单独开关。  
> 开启**系统代理**时，无论 TCP/UDP 模式如何设置，均不会配置 nftables 规则。

#### TUN 配置

仅在 TCP 或 UDP 代理模式选择 TUN 时生效，路由由 MetaViz 控制（`auto-route: false`）。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 设备名称 | `Meta` | TUN 网卡名 |
| 协议栈 | `mixed` | `system` / `gvisor` / `mixed` |
| MTU | `1500` | 最大传输单元 |

#### 域名嗅探（Sniffer）

对 HTTP / TLS / QUIC 流量自动嗅探域名，可选是否覆盖 IP 目标（`override-destination`）。

#### 绕过中国大陆 IP

在 nftables 规则层面直接绕过 CN IP，命中的流量不经过 mihomo，性能更优。

- 需要 `cn-bypass.nft`（IPv4 段）存在于数据目录
- 开启 IPv6 时同时加载 `cn-bypass6.nft`（IPv6 段）
- 首次启动时自动释放到数据目录

---

## 数据目录结构

```
data/
├── run/                  # mihomo 运行目录
│   ├── config.yaml       # 生成的 mihomo 配置
│   ├── mrs/              # 规则集文件（.mrs）
│   └── providers/        # 订阅节点缓存
├── configs/              # 上传的自定义 YAML 配置
├── cn-bypass.nft         # CN IPv4 绕过规则（首次启动自动释放）
├── cn-bypass6.nft        # CN IPv6 绕过规则（首次启动自动释放）
├── metaviz.db            # 数据库（节点、订阅、设置）
└── ipfilter.json         # IP 过滤配置
```

---

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--port` | `8080` | Web UI 监听端口 |
| `--dir` | `<exe目录>/data` | 数据目录路径 |

---

## 注意事项

- MetaViz 需要 **root 权限**，用于操作 nftables 和 ip route
- mihomo 以独立进程运行，MetaViz 负责生命周期管理
- 停止 MetaViz 时会自动清理 nftables 规则和路由
- `cn-bypass.nft` 和 `cn-bypass6.nft` 数据目录中已存在时不会覆盖，如需更新请手动删除后重启
