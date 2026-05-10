<template>
  <div style="display:flex;flex-direction:column;height:100%;overflow:hidden">
    <!-- Topbar -->
    <div class="topbar">
      <span class="topbar-title">仪表盘</span>
      <div style="display:flex;align-items:center;gap:10px;margin-left:auto">
        <span class="text-xs text-muted monospace">{{ now }}</span>
        <button v-if="!isRunning" class="btn btn-primary btn-sm"
          :disabled="!canStart || starting" @click="doStart">
          {{ starting ? '启动中…' : '▶ 启动' }}
        </button>
        <button v-else class="btn btn-danger btn-sm" @click="doStop">⏹ 停止</button>
      </div>
    </div>

    <div class="page">
      <div class="page-inner" style="display:flex;flex-direction:column;gap:16px">

        <!-- Stats row -->
        <div class="grid-4">
          <div class="stat-widget">
            <span class="stat-label">状态</span>
            <span class="stat-value" :style="stateColor">{{ stateLabel }}</span>
            <span class="stat-sub">mihomo core</span>
          </div>
          <div class="stat-widget">
            <span class="stat-label">PID</span>
            <span class="stat-value">{{ status.pid || '—' }}</span>
            <span class="stat-sub">进程 ID</span>
          </div>
          <div class="stat-widget">
            <span class="stat-label">内存</span>
            <span class="stat-value">{{ memStr }}</span>
            <span class="stat-sub">RSS 占用</span>
          </div>
          <div class="stat-widget">
            <span class="stat-label">版本</span>
            <span class="stat-value" style="font-size:13px">{{ mihomoVersion || '未安装' }}</span>
            <span class="stat-sub">mihomo</span>
          </div>
        </div>

        <!-- Main grid -->
        <div class="grid-2" style="align-items:stretch">

          <!-- Config selector -->
          <div class="card" style="position:relative;display:flex;flex-direction:column;gap:14px">
            <div v-if="isRunning" class="card-disabled-overlay"></div>
            <div class="card-title" style="margin-bottom:0">选择配置</div>

            <!-- Mode cards -->
            <div class="mode-grid">
              <div class="mode-card" :class="{on: params.configMode==='node'}" @click="params.configMode='node'">
                <div class="mode-icon">🔗</div>
                <div class="mode-name">单节点</div>
                <div class="mode-desc">从节点列表选择</div>
              </div>
              <div class="mode-card" :class="{on: params.configMode==='subscription'}" @click="params.configMode='subscription'">
                <div class="mode-icon">📡</div>
                <div class="mode-name">订阅模式</div>
                <div class="mode-desc">使用完整订阅</div>
              </div>
              <div class="mode-card" :class="{on: params.configMode==='upload'}" @click="params.configMode='upload'">
                <div class="mode-icon">📄</div>
                <div class="mode-name">上传配置</div>
                <div class="mode-desc">使用上传的 YAML</div>
              </div>
            </div>

            <!-- 单节点：选择节点 -->
            <div v-if="params.configMode==='node'" class="form-group" style="margin:0">
              <label class="form-label">节点</label>
              <button class="select" style="text-align:left;cursor:pointer" @click="showNodePicker=true">
                <span v-if="selectedNodeLabel">{{ selectedNodeLabel }}</span>
                <span v-else style="color:var(--text3)">— 点击选择节点 —</span>
              </button>
            </div>

            <!-- 订阅模式：选择订阅 -->
            <div v-if="params.configMode==='subscription'" class="form-group" style="margin:0">
              <label class="form-label">订阅</label>
              <select class="select" v-model="params.subscriptionId">
                <option value="">— 选择订阅 —</option>
                <option v-for="s in subsStore.subs" :key="s.id" :value="s.id">
                  {{ s.name }} ({{ s.nodeCount || 0 }} 节点)
                </option>
              </select>
            </div>

            <!-- 上传配置：选择文件 -->
            <div v-if="params.configMode==='upload'" class="form-group" style="margin:0">
              <label class="form-label">配置文件</label>
              <select class="select" v-model="params.uploadedConfigFile">
                <option value="">— 选择配置文件 —</option>
                <option v-for="uc in uploadedConfigs" :key="uc.filename" :value="uc.filename">
                  {{ uc.filename }}
                </option>
              </select>
            </div>

            <!-- 路由模式 + 广告拦截（单节点/订阅模式才显示） -->
            <template v-if="params.configMode!=='upload'">
              <div style="border-top:1px solid var(--border);padding-top:12px">
                <label class="form-label" style="margin-bottom:8px">路由模式</label>
                <div class="seg">
                  <button class="seg-btn" :class="{on:params.routeMode==='whitelist'}" @click="params.routeMode='whitelist'">🇨🇳 大陆白名单</button>
                  <button class="seg-btn" :class="{on:params.routeMode==='gfwlist'}" @click="params.routeMode='gfwlist'">📋 GFW列表</button>
                  <button class="seg-btn" :class="{on:params.routeMode==='global'}" @click="params.routeMode='global'">🌍 全局</button>
                </div>
              </div>
              <div class="toggle-row" style="border:none;padding:0">
                <div>
                  <div class="toggle-label">广告拦截</div>
                  <div class="toggle-desc">使用内置 ads.mrs 规则集</div>
                </div>
                <label class="toggle">
                  <input type="checkbox" v-model="params.blockAds">
                  <div class="toggle-track"><div class="toggle-thumb"></div></div>
                </label>
              </div>
            </template>

            <div style="border-top:1px solid var(--border);padding-top:10px;font-size:12px;color:var(--text3)">
              代理模式：<strong style="color:var(--text2)">{{ currentProxyModeLabel }}</strong>
              &ensp;·&ensp;<router-link to="/settings" style="color:var(--accent)">在设置中修改</router-link>
            </div>

            <div v-if="startErr" class="alert alert-error">{{ startErr }}</div>
          </div>

          <!-- Log card -->
          <div class="card" style="padding:0;overflow:hidden;display:flex;flex-direction:column;max-height:420px">
            <div style="padding:14px 16px 12px;display:flex;align-items:center;gap:8px;border-bottom:1px solid var(--border)">
              <span class="card-title" style="margin:0">实时日志</span>
              <div v-if="isRunning" style="display:flex;align-items:center;gap:5px;margin-left:8px">
                <div class="live-dot"></div>
                <span style="font-size:11px;color:var(--green);font-weight:600">LIVE</span>
              </div>
              <button class="btn btn-secondary btn-sm" style="margin-left:auto" @click="logsStore.clear()">清空</button>
            </div>
            <div class="log-box" style="flex:1;border-radius:0;height:300px;max-height:300px;overflow-y:auto;min-height:0" ref="logEl">
              <div v-if="!logsStore.lines.length" class="log-line" style="opacity:.4">等待日志…</div>
              <div v-for="(l,i) in logsStore.lines.slice(-120)" :key="i" class="log-line" :class="logCls(l)">{{ l }}</div>
            </div>
          </div>

        </div>
      </div>
    </div>

    <!-- 节点选择弹窗 -->
    <div v-if="showNodePicker" class="mask" @click.self="showNodePicker=false">
      <div class="modal">
        <div class="modal-head">
          <span>选择节点</span>
          <button class="btn-icon btn btn-secondary" @click="showNodePicker=false">✕</button>
        </div>
        <div class="modal-body">
          <!-- 导入节点 -->
          <div class="np-section-title">导入节点（{{ nodesStore.nodes.length }}）</div>
          <div v-if="!nodesStore.nodes.length" class="empty-state" style="padding:12px 0">暂无导入节点</div>
          <div class="np-grid">
            <button v-for="n in nodesStore.nodes" :key="n.id"
              class="np-item" :class="{on: params.configMode==='node' && params.nodeId===n.id}"
              @click="pickNode(n)">
              <span class="np-type">{{ n.protocol }}</span>
              <span class="np-name">{{ n.name }}</span>
              <span class="np-addr">{{ n.address }}:{{ n.port }}</span>
            </button>
          </div>

          <!-- 订阅节点 -->
          <template v-for="sub in subsStore.subs" :key="sub.id">
            <div class="np-section-title" style="margin-top:16px;display:flex;align-items:center;gap:8px">
              <span>订阅：{{ sub.name }}（{{ sub.nodeCount||0 }}）</span>
              <button class="btn btn-secondary btn-sm" style="font-size:11px;padding:3px 8px"
                @click="toggleSubProxies(sub.id)">
                {{ subProxyCache[sub.id] ? '收起' : '展开节点' }}
              </button>
            </div>
            <div v-if="subProxyCache[sub.id]" class="np-grid">
              <button v-for="(p,idx) in subProxyCache[sub.id]" :key="idx"
                class="np-item"
                :class="{on: params.configMode==='subnode' && params.subscriptionId===sub.id && params.subNodeIdx===idx}"
                @click="pickSubNode(sub.id, idx, p)">
                <span class="np-type">{{ p.type }}</span>
                <span class="np-name">{{ p.name || '节点'+(idx+1) }}</span>
                <span class="np-addr">{{ p.server }}</span>
              </button>
            </div>
          </template>
        </div>
        <div class="modal-foot">
          <button class="btn btn-secondary btn-sm" @click="showNodePicker=false">关闭</button>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { api } from '../api.js'
import { useStatusStore, useNodesStore, useSubsStore, useLogsStore } from '../stores.js'

const statusStore = useStatusStore()
const subsStore   = useSubsStore()
const nodesStore  = useNodesStore()
const logsStore   = useLogsStore()

const status    = computed(() => statusStore.status)
const isRunning = computed(() => statusStore.isRunning)

const mihomoVersion = ref('')
const memStr        = ref('—')
const now           = ref('')
const starting      = ref(false)
const startErr      = ref('')
const logEl         = ref(null)
const uploadedConfigs = ref([])
const showNodePicker  = ref(false)
const subProxyCache   = reactive({})
const proxySettings   = ref({ systemProxy: false, tcpMode: 'redir', udpMode: 'tproxy' })

const stateLabel = computed(() => ({ running:'运行中', stopped:'已停止', error:'错误' }[status.value.state] || status.value.state))
const stateColor = computed(() => ({
  running: 'color:var(--green)',
  stopped: 'color:var(--text3)',
  error:   'color:var(--red)',
}[status.value.state] || ''))

const currentProxyModeLabel = computed(() => {
  const ps = proxySettings.value
  if (ps.systemProxy) return '系统代理'
  if (ps.tcpMode === 'tun' || ps.udpMode === 'tun') return 'TUN'
  if (ps.tcpMode === 'tproxy' || ps.udpMode === 'tproxy') return 'TProxy'
  if (ps.tcpMode === 'redir') return 'Redirect'
  return '—'
})

// Persist params
function loadParams() {
  try { const s = localStorage.getItem('metaviz_params'); if (s) return JSON.parse(s) } catch {}
  return null
}
const _saved = loadParams()
const params = reactive({
  configMode:         _saved?.configMode         || 'node',
  nodeId:             _saved?.nodeId             || '',
  subscriptionId:     _saved?.subscriptionId     || '',
  subNodeIdx:         _saved?.subNodeIdx         ?? -1,
  uploadedConfigFile: _saved?.uploadedConfigFile || '',
  routeMode:          _saved?.routeMode          || 'whitelist',
  blockAds:           _saved?.blockAds           ?? true,
})
watch(() => ({...params}), v => localStorage.setItem('metaviz_params', JSON.stringify(v)), { deep: true })

// selected node label
const selectedNodeLabel = ref(_saved?.selectedNodeLabel || '')

const canStart = computed(() => {
  if (params.configMode === 'node')         return !!params.nodeId
  if (params.configMode === 'subnode')      return !!params.subscriptionId && params.subNodeIdx >= 0
  if (params.configMode === 'subscription') return !!params.subscriptionId
  if (params.configMode === 'upload')       return !!params.uploadedConfigFile
  return false
})

async function loadUploadedConfigs() {
  try { uploadedConfigs.value = await api('GET', '/config/list') } catch {}
}

function pickNode(n) {
  params.configMode = 'node'
  params.nodeId = n.id
  selectedNodeLabel.value = `[${n.protocol}] ${n.name}`
  localStorage.setItem('metaviz_params', JSON.stringify({...params, selectedNodeLabel: selectedNodeLabel.value}))
  showNodePicker.value = false
}

function pickSubNode(subId, idx, p) {
  params.configMode = 'subnode'
  params.subscriptionId = subId
  params.subNodeIdx = idx
  selectedNodeLabel.value = `[${p.type}] ${p.name || '节点'+(idx+1)}`
  localStorage.setItem('metaviz_params', JSON.stringify({...params, selectedNodeLabel: selectedNodeLabel.value}))
  showNodePicker.value = false
}

async function toggleSubProxies(subId) {
  if (subProxyCache[subId]) { delete subProxyCache[subId]; return }
  try { subProxyCache[subId] = await subsStore.getProxies(subId) } catch {}
}

async function doStart() {
  starting.value = true; startErr.value = ''
  try {
    await api('POST', '/start', { ...params })
    await statusStore.fetch()
    logsStore.startSSE()
  } catch (e) { startErr.value = e.message }
  finally { starting.value = false }
}

async function doStop() {
  await api('POST', '/stop')
  await statusStore.fetch()
  logsStore.stopSSE()
}

function logCls(l) {
  const s = l.toLowerCase()
  if (s.includes('error') || s.includes('fatal')) return 'err'
  if (s.includes('warn')) return 'warn'
  return ''
}

watch(() => logsStore.lines.length, () => {
  nextTick(() => { if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight })
})

function updateMem() {
  const kb = status.value.rssKB
  if (!kb) { memStr.value = '—'; return }
  memStr.value = kb < 1024 ? kb + ' KB' : (kb/1024).toFixed(1) + ' MB'
}

let clockTimer = null
onMounted(async () => {
  try { const r = await api('GET', '/mihomo/version'); mihomoVersion.value = (r.version || '').match(/v[\d.]+/)?.[0] || r.version } catch {}
  try { proxySettings.value = await api('GET', '/proxy-settings') } catch {}
  await Promise.all([nodesStore.load(), subsStore.load(), loadUploadedConfigs()])
  clockTimer = setInterval(() => { now.value = new Date().toLocaleTimeString('zh-CN'); updateMem() }, 2000)
  now.value = new Date().toLocaleTimeString('zh-CN')
  updateMem()
})
onUnmounted(() => clearInterval(clockTimer))
</script>

<style scoped>
.grid-4 { display:grid; grid-template-columns:repeat(4,1fr); gap:12px; }
@media(max-width:800px){ .grid-4{ grid-template-columns:repeat(2,1fr); } }
@media(max-width:480px){ .grid-4{ grid-template-columns:repeat(2,1fr); } }

.stat-widget {
  background:var(--surface); border:1px solid var(--border);
  border-radius:var(--radius-lg); padding:16px;
  display:flex; flex-direction:column; gap:4px;
}
.stat-label { font-size:12px; color:var(--text3); font-weight:600; }
.stat-value { font-size:22px; font-weight:700; color:var(--text); font-family:var(--mono); line-height:1.2; }
.stat-sub   { font-size:11px; color:var(--text3); margin-top:2px; }

.mode-grid { display:grid; grid-template-columns:repeat(3,1fr); gap:8px; }
.mode-card {
  border:1.5px solid var(--border2); border-radius:var(--radius);
  padding:10px 8px; cursor:pointer; text-align:center;
  transition:all .12s; background:var(--surface2);
}
.mode-card:hover { border-color:var(--accent-lt); background:var(--accent-bg); }
.mode-card.on { border-color:var(--accent); background:var(--accent-bg); }
.mode-icon { font-size:20px; margin-bottom:4px; }
.mode-name { font-size:12.5px; font-weight:700; color:var(--text); }
.mode-desc { font-size:11px; color:var(--text3); margin-top:2px; }

.seg { display:flex; gap:4px; flex-wrap:wrap; }
.seg-btn {
  padding:6px 12px; border-radius:var(--radius); border:1.5px solid var(--border2);
  background:var(--surface2); color:var(--text2); font-size:12.5px; font-weight:500;
  cursor:pointer; transition:all .12s; white-space:nowrap;
}
.seg-btn:hover { border-color:var(--accent-lt); color:var(--accent); }
.seg-btn.on { background:var(--accent); color:#fff; border-color:var(--accent); }

.live-dot {
  width:7px; height:7px; border-radius:50%; background:var(--green);
  animation:pulse 1.5s infinite;
}
@keyframes pulse { 0%,100%{opacity:1;transform:scale(1)} 50%{opacity:.5;transform:scale(.8)} }

.card-disabled-overlay {
  position:absolute; inset:0; border-radius:inherit; z-index:10; cursor:not-allowed;
  background:rgba(255,245,245,.7);
}

.mask {
  position:fixed; inset:0; background:rgba(26,8,8,.5); z-index:300;
  display:flex; align-items:center; justify-content:center;
  backdrop-filter:blur(2px);
}
.modal {
  background:var(--surface); border-radius:var(--radius-lg);
  box-shadow:var(--shadow-lg); width:90%; max-width:560px;
  max-height:80vh; display:flex; flex-direction:column;
}
.modal-head {
  display:flex; align-items:center; justify-content:space-between;
  padding:14px 18px; border-bottom:1px solid var(--border);
  font-size:15px; font-weight:700; color:var(--text);
}
.modal-body { flex:1; overflow-y:auto; padding:14px 18px; }
.modal-foot { padding:10px 18px; border-top:1px solid var(--border); display:flex; justify-content:flex-end; }

.np-section-title {
  font-size:11.5px; font-weight:700; color:var(--text3);
  text-transform:uppercase; letter-spacing:.05em;
  padding:4px 0 6px; border-bottom:1px solid var(--border); margin-bottom:8px;
}
.np-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(150px,1fr)); gap:6px; margin-bottom:4px; }
.np-item {
  display:flex; flex-direction:column; gap:2px; padding:8px 10px;
  background:var(--surface2); border:1.5px solid var(--border2);
  border-radius:var(--radius); cursor:pointer; text-align:left; transition:all .12s;
}
.np-item:hover { border-color:var(--accent-lt); }
.np-item.on { border-color:var(--accent); background:var(--accent-bg); }
.np-type { font-size:10px; font-weight:700; color:var(--accent); font-family:var(--mono); }
.np-name { font-size:12px; font-weight:600; color:var(--text); white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.np-addr { font-size:10px; color:var(--text3); font-family:var(--mono); white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
</style>
