<template>
  <div style="display:flex;flex-direction:column;height:100%;overflow:hidden">
    <div class="topbar">
      <span class="topbar-title">设置</span>
      <div style="margin-left:auto;display:flex;gap:8px">
        <button class="btn btn-secondary btn-sm" @click="loadAll">↺ 重置</button>
        <button class="btn btn-primary btn-sm" :disabled="saving" @click="saveAll">
          {{ saving ? '保存中…' : '保存设置' }}
        </button>
      </div>
    </div>

    <div class="page">
      <div class="page-inner" style="display:flex;flex-direction:column;gap:16px">

        <div v-if="saveOk" class="alert alert-success">设置已保存</div>
        <div v-if="saveErr" class="alert alert-error">{{ saveErr }}</div>

        <!-- ── 代理配置 ───────────────────────────────────── -->
        <div class="card">
          <div class="card-title">代理配置</div>
          <div class="grid-2">
            <div>
              <div class="form-label" style="margin-bottom:8px">TCP 代理模式</div>
              <div style="display:flex;flex-direction:column;gap:6px">
                <label v-for="opt in tcpModes" :key="opt.v" class="radio-row">
                  <input type="radio" :value="opt.v" v-model="ps.tcpMode">
                  <span class="radio-label">{{ opt.label }}</span>
                  <span class="radio-desc">{{ opt.desc }}</span>
                </label>
              </div>
            </div>
            <div>
              <div class="form-label" style="margin-bottom:8px">UDP 代理模式</div>
              <div style="display:flex;flex-direction:column;gap:6px">
                <label v-for="opt in udpModes" :key="opt.v" class="radio-row">
                  <input type="radio" :value="opt.v" v-model="ps.udpMode">
                  <span class="radio-label">{{ opt.label }}</span>
                  <span class="radio-desc">{{ opt.desc }}</span>
                </label>
              </div>
            </div>
          </div>

          <div style="border-top:1px solid var(--border);margin-top:14px;padding-top:14px;display:flex;flex-direction:column;gap:0">
            <div class="toggle-row">
              <div><div class="toggle-label">系统代理</div><div class="toggle-desc">设置 HTTP/HTTPS 系统代理（不启用透明代理）</div></div>
              <label class="toggle"><input type="checkbox" v-model="ps.systemProxy"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
            </div>
            <div class="toggle-row">
              <div><div class="toggle-label">局域网代理</div><div class="toggle-desc">对局域网内的设备也进行代理</div></div>
              <label class="toggle"><input type="checkbox" v-model="ps.lanProxy"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
            </div>
            <div class="toggle-row">
              <div><div class="toggle-label">IPv6</div><div class="toggle-desc">启用 IPv6 透明代理</div></div>
              <label class="toggle"><input type="checkbox" v-model="ps.ipv6"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
            </div>
            <div class="toggle-row">
              <div><div class="toggle-label">绕过中国大陆 IP</div><div class="toggle-desc">防火墙规则层面绕过 CN IP（需要 cn-bypass.nft）</div></div>
              <label class="toggle"><input type="checkbox" v-model="ps.bypassCN"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
            </div>
            <!-- Extra GID bypass -->
            <div class="toggle-row" style="align-items:flex-start;flex-wrap:wrap;gap:8px">
              <div style="flex:0 0 auto">
                <div class="toggle-label">防火墙绕过 GID</div>
                <div class="toggle-desc">多个 GID 用空格分隔，这些 GID 的流量将绕过代理直连</div>
              </div>
              <div style="flex:1;display:flex;align-items:center;gap:8px;min-width:200px">
                <input
                  v-model="extraGIDRaw"
                  @blur="onExtraGIDBlur"
                  placeholder="例：1000 1001 65534"
                  style="flex:1;font-size:13px;font-family:monospace;padding:5px 10px;border-radius:5px;border:1px solid var(--border);background:var(--bg2,var(--color-bg));color:var(--text1,var(--color-text));outline:none"
                  :style="extraGIDError ? 'border-color:#e05555' : ''"
                />
                <span v-if="extraGIDError" style="font-size:11px;color:#e05555;white-space:nowrap">{{ extraGIDError }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- ── DNS 模式 ──────────────────────────────────── -->
        <div class="card">
          <div class="card-title">DNS 模式</div>
          <div class="toggle-row" style="border:none;padding:0">
            <div>
              <div class="toggle-label">启用 Fake-IP 模式</div>
              <div class="toggle-desc">
                使用虚假 IP 进行透明代理，减少 DNS 泄露，提升连接速度。
                启用后防火墙会自动处理 fakeip 池（198.18.0.0/15、fc00::/18）的路由。
              </div>
            </div>
            <label class="toggle"><input type="checkbox" v-model="ms.inbound.fakeIP"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
          </div>

          <!-- 开启 fakeip 时的说明 -->
          <div v-if="ms.inbound.fakeIP" class="fakeip-info">
            <div class="fakeip-info-title">📌 Fake-IP 说明</div>
            <div class="fakeip-info-body">
              <p><strong>单节点 / 订阅模式</strong>：面板自动生成 fake-ip DNS 配置，无需手动操作。</p>
              <p>
                <strong>上传配置模式</strong>：面板不修改上传配置的 DNS 块，
                请在您的配置文件中手动设置 fake-ip DNS，fakeip 地址池为：
              </p>
              <div class="fakeip-pool">
                <code>fake-ip-range: 198.18.0.0/15</code>
                <code>fake-ip-range6: fc00::/18</code>
              </div>
              <p style="margin-top:6px;color:var(--text3);font-size:12px">
                ⚠️ 防火墙规则（包括 fakeip 路由和 ICMP 劫持）在所有模式下均按此开关生效。
              </p>
            </div>
          </div>
        </div>

        <!-- ── 端口配置 ───────────────────────────────────── -->
        <div class="card">
          <div class="card-title">端口配置</div>
          <div class="grid-3">
            <div class="form-group" style="margin:0">
              <label class="form-label">Mixed Port</label>
              <input class="input" type="number" v-model.number="ms.inbound.mixedPort" min="1" max="65535">
              <div class="form-hint">HTTP + SOCKS5 混合入站</div>
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">Redirect Port</label>
              <input class="input" type="number" v-model.number="ms.inbound.redirectPort" min="1" max="65535">
              <div class="form-hint">TCP 透明代理 (redir)</div>
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">TProxy Port</label>
              <input class="input" type="number" v-model.number="ms.inbound.tproxyPort" min="1" max="65535">
              <div class="form-hint">TCP+UDP 透明代理</div>
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">DNS Port</label>
              <input class="input" type="number" v-model.number="ms.inbound.dnsPort" min="1" max="65535">
              <div class="form-hint">DNS 监听端口</div>
            </div>
          </div>
        </div>

        <!-- ── TUN 配置 ────────────────────────────────────── -->
        <div class="card">
          <div class="card-title">TUN 配置</div>
          <div class="form-hint" style="margin-bottom:12px">TUN 在 TCP 或 UDP 代理模式选择「TUN」时自动启用；以下参数在 TUN 启用时生效。</div>
          <div class="grid-3">
            <div class="form-group" style="margin:0">
              <label class="form-label">设备名称</label>
              <input class="input" v-model="ms.tun.device" placeholder="Meta">
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">协议栈</label>
              <select class="select" v-model="ms.tun.stack">
                <option value="system">system</option>
                <option value="gvisor">gvisor</option>
                <option value="mixed">mixed</option>
              </select>
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">MTU</label>
              <input class="input" type="number" v-model.number="ms.tun.mtu" min="576" max="9000">
            </div>
          </div>
        </div>

        <!-- ── Sniffer ────────────────────────────────────── -->
        <div class="card">
          <div class="card-title">域名嗅探 (Sniffer)</div>
          <div class="toggle-row" style="border:none;padding:0;margin-bottom:8px">
            <div>
              <div class="toggle-label">启用域名嗅探</div>
              <div class="toggle-desc">对 HTTP / TLS / QUIC 流量自动嗅探域名</div>
            </div>
            <label class="toggle"><input type="checkbox" v-model="ms.sniffer.enable"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
          </div>
          <div v-if="ms.sniffer.enable" class="toggle-row" style="border:none;padding:0">
            <div>
              <div class="toggle-label">覆盖 IP 目标</div>
              <div class="toggle-desc">嗅探到域名后用域名覆盖原始 IP 目标（override-destination）</div>
            </div>
            <label class="toggle"><input type="checkbox" v-model="ms.sniffer.overrideDestination"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
          </div>
        </div>

        <!-- ── 日志 ──────────────────────────────────────── -->
        <div class="card">
          <div class="card-title">日志</div>
          <div class="form-group" style="margin:0">
            <label class="form-label">日志级别</label>
            <select class="select" v-model="ms.log.level" style="max-width:200px">
              <option value="silent">silent（禁用）</option>
              <option value="error">error</option>
              <option value="warning">warning</option>
              <option value="info">info</option>
              <option value="debug">debug</option>
            </select>
          </div>
        </div>

        <!-- ── Clash API ──────────────────────────────────── -->
        <div class="card">
          <div class="card-title">Clash API（External Controller）</div>
          <div class="grid-2">
            <div class="form-group" style="margin:0">
              <label class="form-label">监听地址</label>
              <input class="input" v-model="ms.clashAPI.listen" placeholder="0.0.0.0:9090">
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">密钥 (Secret)</label>
              <input class="input" v-model="ms.clashAPI.secret" type="password" placeholder="留空不设置">
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">UI 目录 (external-ui)</label>
              <input class="input" v-model="ms.clashAPI.ui" placeholder="留空不设置">
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">UI 下载 URL</label>
              <input class="input" v-model="ms.clashAPI.uiURL" placeholder="可选">
            </div>
          </div>
        </div>

        <!-- ── 杂项 ──────────────────────────────────────── -->
        <div class="card">
          <div class="card-title">杂项</div>
          <div class="toggle-row">
            <div><div class="toggle-label">Unified Delay</div><div class="toggle-desc">统一延迟计算方式</div></div>
            <label class="toggle"><input type="checkbox" v-model="ms.misc.unifiedDelay"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
          </div>
          <div class="toggle-row">
            <div><div class="toggle-label">TCP Concurrent</div><div class="toggle-desc">TCP 并发连接</div></div>
            <label class="toggle"><input type="checkbox" v-model="ms.misc.tcpConcurrent"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
          </div>
          <div class="form-group" style="margin-top:12px;margin-bottom:0">
            <label class="form-label">进程匹配模式 (find-process-mode)</label>
            <select class="select" v-model="ms.misc.findProcessMode" style="max-width:200px">
              <option value="off">off</option>
              <option value="strict">strict</option>
              <option value="always">always</option>
            </select>
          </div>
        </div>

        <!-- ── IP 过滤 ────────────────────────────────────── -->
        <div class="card">
          <div class="card-title">IP 过滤（黑/白名单）</div>
          <div class="form-group" style="margin-bottom:12px">
            <label class="form-label">过滤模式</label>
            <select class="select" v-model="ipf.mode" style="max-width:200px">
              <option value="off">关闭</option>
              <option value="blacklist">黑名单（阻断这些 IP）</option>
              <option value="whitelist">白名单（只允许这些 IP）</option>
            </select>
          </div>
          <div v-if="ipf.mode !== 'off'" class="form-group" style="margin:0">
            <label class="form-label">IP 列表（每行一个，支持 CIDR）</label>
            <textarea class="input" v-model="ipf.ipsText" rows="5" placeholder="192.168.1.0/24&#10;10.0.0.1"></textarea>
          </div>
        </div>

        <!-- ── 定时重启 ───────────────────────────────────── -->
        <div class="card">
          <div class="card-title">定时重启</div>
          <div class="toggle-row" style="margin-bottom:12px">
            <div><div class="toggle-label">启用定时重启</div></div>
            <label class="toggle"><input type="checkbox" v-model="ms.scheduledRestart.enabled"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
          </div>
          <div v-if="ms.scheduledRestart.enabled" class="form-group" style="margin:0">
            <label class="form-label">Cron 表达式</label>
            <input class="input" v-model="ms.scheduledRestart.cron" placeholder="15 3 * * *" style="max-width:280px">
            <div class="form-hint">示例：15 3 * * * = 每天 03:15 重启</div>
          </div>
        </div>

        <!-- ── mihomo 安装 ────────────────────────────────── -->
        <div class="card">
          <div class="card-title">mihomo 内核</div>
          <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
            <div>
              <div style="font-size:13px;color:var(--text2)">当前版本</div>
              <div style="font-size:15px;font-weight:700;font-family:var(--mono);color:var(--text);margin-top:2px">
                {{ mihomoVer || '未安装' }}
              </div>
              <div style="font-size:12px;color:var(--text3);margin-top:2px">{{ sysInfo.osName }}</div>
            </div>
            <div style="margin-left:auto;display:flex;gap:8px;flex-wrap:wrap">
              <input class="input" v-model="installProxy" placeholder="代理地址（可选）" style="width:180px">
              <button class="btn btn-primary" :disabled="installing" @click="doInstall">
                {{ installing ? '安装中…' : '⬇ 安装/更新 mihomo' }}
              </button>
            </div>
          </div>
          <div v-if="installResult" class="alert mt-8" :class="installResult.ok ? 'alert-success':'alert-error'">
            {{ installResult.msg }}
          </div>
        </div>

        <!-- ── 规则集更新 ─────────────────────────────────── -->
        <div class="card">
          <div class="card-title">内置规则集 (.mrs)</div>
          <div style="display:flex;align-items:center;gap:10px;margin-bottom:14px;flex-wrap:wrap">
            <span style="font-size:13px;color:var(--text2)">共 {{ rulesets.length }} 个规则集文件</span>
            <button class="btn btn-primary" style="margin-left:auto" :disabled="updatingRules" @click="doUpdateRules">
              {{ updatingRules ? '更新中…' : '⬇ 一键更新规则集' }}
            </button>
          </div>
          <div v-if="updateRulesResult" class="alert mb-8" :class="updateRulesResult.failed===0?'alert-success':'alert-error'">
            更新完成：{{ updateRulesResult.total - updateRulesResult.failed }} 成功，{{ updateRulesResult.failed }} 失败
          </div>
          <table class="tbl">
            <thead><tr><th>文件</th><th>大小</th><th>更新时间</th><th></th></tr></thead>
            <tbody>
              <tr v-if="!rulesets.length"><td colspan="4" class="empty-state">暂无规则集文件</td></tr>
              <tr v-for="r in rulesets" :key="r.file">
                <td class="monospace" style="font-size:12px">{{ r.file }}</td>
                <td class="text-muted text-xs">{{ fmtSize(r.size) }}</td>
                <td class="text-muted text-xs">{{ fmtDate(r.updatedAt) }}</td>
                <td><button class="del-btn" @click="deleteRuleset(r.file)" title="删除">✕</button></td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- ── 账号 ──────────────────────────────────────── -->
        <div class="card">
          <div class="card-title">账号 & 认证</div>
          <div class="toggle-row">
            <div><div class="toggle-label">启用登录认证</div></div>
            <label class="toggle"><input type="checkbox" v-model="ms.auth.enabled"><div class="toggle-track"><div class="toggle-thumb"></div></div></label>
          </div>
          <div v-if="ms.auth.enabled" style="margin-top:12px;display:flex;flex-direction:column;gap:12px">
            <div class="form-group" style="margin:0">
              <label class="form-label">用户名</label>
              <input class="input" v-model="ms.auth.username" style="max-width:280px">
            </div>
            <div class="form-group" style="margin:0">
              <label class="form-label">新密码（留空不修改）</label>
              <input class="input" type="password" v-model="newPassword" style="max-width:280px">
            </div>
          </div>
        </div>

      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { api } from '../api.js'

const saving  = ref(false)
const saveOk  = ref(false)
const saveErr = ref('')

// Proxy settings
const ps = reactive({ systemProxy: false, tcpMode: 'redir', udpMode: 'tproxy', lanProxy: false, ipv6: false, bypassCN: false, extraGIDs: [] })

// Extra GID bypass — space-separated list, e.g. "1000 1001 65534"
const extraGIDRaw   = ref('')
const extraGIDError = ref('')

function parseGIDs(raw) {
  const parts = raw.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return []
  const result = []
  for (const p of parts) {
    if (!/^\d+$/.test(p)) return null
    const n = parseInt(p, 10)
    if (n < 0 || n > 65535) return null
    result.push(n)
  }
  return result
}

function onExtraGIDBlur() {
  if (!extraGIDRaw.value.trim()) { extraGIDError.value = ''; ps.extraGIDs = []; return }
  const gids = parseGIDs(extraGIDRaw.value)
  if (gids === null) { extraGIDError.value = '只能输入数字（0-65535），多个用空格分隔'; return }
  extraGIDError.value = ''
  extraGIDRaw.value = gids.join(' ')
  ps.extraGIDs = gids
}

const tcpModes = [
  { v: 'off',    label: '关闭',     desc: '不启用 TCP 透明代理' },
  { v: 'redir',  label: 'Redirect', desc: 'NAT 重定向（TCP only）' },
  { v: 'tproxy', label: 'TProxy',   desc: '透明代理（TCP+UDP）' },
  { v: 'tun',    label: 'TUN',      desc: '虚拟网卡' },
]
const udpModes = [
  { v: 'off',    label: '关闭',     desc: '不代理 UDP' },
  { v: 'tproxy', label: 'TProxy',   desc: 'UDP TProxy' },
  { v: 'tun',    label: 'TUN',      desc: '虚拟网卡' },
]

// Meta settings（含 inbound.fakeIP）
const ms = reactive({
  inbound: { mixedPort: 7890, redirectPort: 7892, tproxyPort: 7893, dnsPort: 1053, fakeIP: false },
  tun: { device: 'Meta', stack: 'mixed', mtu: 1500 },
  sniffer: { enable: true, overrideDestination: true },
  log: { level: 'warning' },
  clashAPI: { listen: '0.0.0.0:9090', secret: '', ui: '', uiURL: '' },
  misc: { findProcessMode: 'off', unifiedDelay: true, tcpConcurrent: true },
  auth: { enabled: true, username: '' },
  scheduledRestart: { enabled: false, cron: '15 3 * * *' },
})
const newPassword = ref('')

// IP filter
const ipf = reactive({ mode: 'off', ipsText: '' })

// mihomo
const mihomoVer    = ref('')
const sysInfo      = ref({ osName: '' })
const installing   = ref(false)
const installProxy  = ref('')
const installResult = ref(null)

// Rulesets
const rulesets         = ref([])
const updatingRules    = ref(false)
const updateRulesResult = ref(null)

async function loadAll() {
  try {
    const [psData, msData, ipfData, ver, sys, rs] = await Promise.all([
      api('GET', '/proxy-settings'),
      api('GET', '/meta-settings'),
      api('GET', '/ip-filter'),
      api('GET', '/mihomo/version'),
      api('GET', '/system-info'),
      api('GET', '/rulesets'),
    ])
    Object.assign(ps, psData)
    const gids = Array.isArray(psData.extraGIDs) ? psData.extraGIDs.filter(Boolean) : []
    ps.extraGIDs = gids
    extraGIDRaw.value = gids.join(' ')
    // Deep merge ms
    for (const k of Object.keys(msData)) {
      if (typeof msData[k] === 'object' && msData[k] !== null && !Array.isArray(msData[k])) {
        Object.assign(ms[k] || {}, msData[k])
      } else {
        ms[k] = msData[k]
      }
    }
    Object.assign(ipf, { mode: ipfData.mode || 'off', ipsText: (ipfData.ips || []).join('\n') })
    mihomoVer.value = ver.version
    sysInfo.value   = sys
    rulesets.value  = rs
  } catch (e) { saveErr.value = '加载失败: ' + e.message }
}

async function saveAll() {
  saving.value = true; saveOk.value = false; saveErr.value = ''
  try {
    onExtraGIDBlur()
    if (extraGIDError.value) { saving.value = false; return }
    await api('POST', '/proxy-settings', { ...ps })

    const msPayload = JSON.parse(JSON.stringify(ms))
    if (newPassword.value) msPayload.auth = { ...msPayload.auth, newPassword: newPassword.value }
    await api('POST', '/meta-settings', msPayload)

    const ips = ipf.ipsText.split('\n').map(s => s.trim()).filter(Boolean)
    await api('POST', '/ip-filter', { mode: ipf.mode, ips })

    newPassword.value = ''
    saveOk.value = true
    setTimeout(() => saveOk.value = false, 3000)
  } catch (e) { saveErr.value = e.message }
  saving.value = false
}

async function doInstall() {
  installing.value = true; installResult.value = null
  try {
    const r = await api('POST', '/mihomo/install', { proxy: installProxy.value, version: 'latest' })
    installResult.value = { ok: true, msg: '安装成功：' + r.version }
    mihomoVer.value = r.version
  } catch (e) {
    installResult.value = { ok: false, msg: e.message }
  }
  installing.value = false
}

async function doUpdateRules() {
  updatingRules.value = true; updateRulesResult.value = null
  try {
    const r = await api('POST', '/update-rules', {})
    updateRulesResult.value = r
    rulesets.value = await api('GET', '/rulesets')
    setTimeout(() => updateRulesResult.value = null, 5000)
  } catch (e) { saveErr.value = e.message }
  updatingRules.value = false
}

async function deleteRuleset(file) {
  if (!confirm('确认删除 ' + file + '？')) return
  try { await api('DELETE', '/rulesets/' + file); rulesets.value = await api('GET', '/rulesets') } catch (e) { alert(e.message) }
}

function fmtSize(b) {
  if (!b) return '0 B'
  if (b < 1024) return b + ' B'
  if (b < 1048576) return (b/1024).toFixed(1) + ' KB'
  return (b/1048576).toFixed(2) + ' MB'
}
function fmtDate(d) {
  if (!d) return ''
  return new Date(d).toLocaleString('zh-CN', { month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit' })
}

onMounted(loadAll)
</script>

<style scoped>
.radio-row {
  display:flex; align-items:center; gap:8px; cursor:pointer;
  padding:7px 10px; border-radius:var(--radius);
  border:1.5px solid var(--border2); background:var(--surface2);
  transition:all .12s;
}
.radio-row:has(input:checked) { border-color:var(--accent); background:var(--accent-bg); }
.radio-row input { accent-color:var(--accent); flex-shrink:0; }
.radio-label { font-size:13px; font-weight:600; color:var(--text); }
.radio-desc  { font-size:11.5px; color:var(--text3); margin-left:auto; }

/* fakeip 提示框 */
.fakeip-info {
  margin-top:12px;
  border-radius:var(--radius);
  border:1px solid var(--accent);
  background:var(--accent-bg);
  overflow:hidden;
}
.fakeip-info-title {
  padding:8px 14px;
  font-size:13px;
  font-weight:600;
  color:var(--accent);
  border-bottom:1px solid color-mix(in srgb, var(--accent) 20%, transparent);
}
.fakeip-info-body {
  padding:10px 14px;
  font-size:13px;
  color:var(--text2);
  display:flex;
  flex-direction:column;
  gap:4px;
}
.fakeip-info-body p { margin:0; line-height:1.6; }
.fakeip-pool {
  display:flex;
  flex-wrap:wrap;
  gap:8px;
  margin-top:4px;
}
.fakeip-pool code {
  font-family:var(--mono);
  font-size:12px;
  background:var(--surface2);
  border:1px solid var(--border2);
  border-radius:4px;
  padding:3px 8px;
  color:var(--text);
}
</style>
