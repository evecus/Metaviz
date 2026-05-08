<template>
  <div style="display:flex;flex-direction:column;height:100%;overflow:hidden">
    <!-- Topbar -->
    <div class="topbar">
      <span class="topbar-title">节点与配置</span>
      <div class="topbar-tabs" style="margin-left:auto">
        <button class="tab-btn" :class="{active: tab==='nodes'}" @click="tab='nodes'">节点</button>
        <button class="tab-btn" :class="{active: tab==='configs'}" @click="tab='configs'">上传配置</button>
      </div>
    </div>

    <!-- ═══════════════ 节点 Tab ═══════════════ -->
    <div v-if="tab==='nodes'" class="page">
      <div class="page-inner">
        <div class="card-row" style="gap:16px;align-items:flex-start">

          <!-- Left: Import + Add subscription -->
          <div style="flex:1;display:flex;flex-direction:column;gap:16px;min-width:0">

            <!-- Import nodes -->
            <div class="card">
              <div class="card-title">导入节点</div>
              <div class="form-group" style="margin:0">
                <label class="form-label">粘贴分享链接（每行一个）</label>
                <textarea class="input" v-model="importText" rows="6"
                  placeholder="vmess://...&#10;vless://...&#10;trojan://...&#10;ss://...&#10;tuic://...&#10;hy2://..."></textarea>
                <div class="form-hint">支持 vmess / vless / trojan / ss / tuic / hysteria2</div>
              </div>
              <div style="display:flex;gap:8px;margin-top:10px">
                <button class="btn btn-primary" :disabled="!importText.trim() || importing" @click="doImport">
                  {{ importing ? '导入中…' : '+ 导入' }}
                </button>
                <button class="btn btn-secondary" @click="importText=''">清空</button>
              </div>
              <div v-if="importResult" class="alert mt-8" :class="importResult.ok ? 'alert-success' : 'alert-error'">
                {{ importResult.msg }}
              </div>
            </div>

            <!-- Add subscription -->
            <div class="card">
              <div class="card-title">添加订阅</div>
              <div class="form-group">
                <label class="form-label">订阅名称</label>
                <input class="input" v-model="subForm.name" placeholder="我的订阅">
              </div>
              <div class="form-group" style="margin-bottom:10px">
                <label class="form-label">订阅 URL</label>
                <input class="input" v-model="subForm.url" placeholder="https://...">
              </div>
              <button class="btn btn-primary" :disabled="!subForm.url || addingSub" @click="doAddSub">
                {{ addingSub ? '添加中…' : '+ 添加订阅' }}
              </button>
              <div v-if="subError" class="alert alert-error mt-8">{{ subError }}</div>
            </div>

          </div>

          <!-- Right: Node list + Subscription list -->
          <div style="flex:1;display:flex;flex-direction:column;gap:16px;min-width:0">

            <!-- Node list -->
            <div class="card">
              <div style="display:flex;align-items:center;margin-bottom:12px">
                <span class="card-title" style="margin:0">节点列表（{{ nodesStore.nodes.length }}）</span>
                <button v-if="nodesStore.nodes.length" class="btn btn-secondary btn-sm" style="margin-left:auto" @click="clearAllNodes">清空全部</button>
              </div>
              <div v-if="!nodesStore.nodes.length" class="empty-state">暂无导入节点</div>
              <div v-else style="display:flex;flex-direction:column;gap:6px;max-height:320px;overflow-y:auto">
                <div v-for="n in nodesStore.nodes" :key="n.id" class="item-card">
                  <span class="proto-badge" :class="'proto-'+n.protocol">{{ n.protocol }}</span>
                  <div style="flex:1;min-width:0">
                    <div class="item-title">{{ n.name }}</div>
                    <div class="item-sub">{{ n.address }}:{{ n.port }}</div>
                  </div>
                  <div class="item-actions">
                    <button class="del-btn" @click.stop="nodesStore.del(n.id)" title="删除">✕</button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Subscription list -->
            <div class="card">
              <div style="display:flex;align-items:center;margin-bottom:12px">
                <span class="card-title" style="margin:0">订阅列表（{{ subsStore.subs.length }}）</span>
              </div>
              <div v-if="!subsStore.subs.length" class="empty-state">暂无订阅</div>
              <div v-else style="display:flex;flex-direction:column;gap:8px;max-height:400px;overflow-y:auto">
                <div v-for="sub in subsStore.subs" :key="sub.id" class="sub-card">
                  <div style="display:flex;align-items:center;gap:8px">
                    <span class="proto-badge proto-sub">订阅</span>
                    <div style="flex:1;min-width:0">
                      <div class="item-title">{{ sub.name }}</div>
                      <div class="item-sub" style="font-size:11px">{{ sub.url }}</div>
                    </div>
                    <div class="item-actions">
                      <button class="del-btn" @click="subsStore.del(sub.id)" title="删除">✕</button>
                    </div>
                  </div>
                  <div style="display:flex;align-items:center;gap:10px;margin-top:8px;padding-top:8px;border-top:1px solid var(--border)">
                    <span style="font-size:11.5px;color:var(--text3)">
                      {{ sub.nodeCount || 0 }} 节点
                      <span v-if="sub.updatedAt"> · {{ fmtDate(sub.updatedAt) }}</span>
                    </span>
                    <span v-if="sub.error" class="text-xs" style="color:var(--red)">{{ sub.error }}</span>
                    <button class="btn btn-secondary btn-sm" style="margin-left:auto"
                      :disabled="updatingSub===sub.id" @click="doUpdateSub(sub.id)">
                      {{ updatingSub===sub.id ? '更新中…' : '↻ 更新' }}
                    </button>
                    <button class="btn btn-secondary btn-sm" @click="toggleSubDetail(sub.id)">
                      {{ expandedSub===sub.id ? '收起' : '查看节点' }}
                    </button>
                  </div>
                  <!-- Proxy list -->
                  <div v-if="expandedSub===sub.id && subProxies[sub.id]" style="margin-top:8px">
                    <div v-if="!subProxies[sub.id].length" class="empty-state" style="padding:8px 0">暂无节点，请先更新</div>
                    <div v-else style="display:flex;flex-direction:column;gap:4px">
                      <div v-for="(p,idx) in subProxies[sub.id]" :key="idx"
                        style="display:flex;align-items:center;gap:6px;padding:6px 8px;background:var(--surface2);border-radius:6px">
                        <span class="proto-badge" :class="'proto-'+(p.type||'')">{{ p.type }}</span>
                        <span style="flex:1;font-size:12px;font-weight:500;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ p.name }}</span>
                        <span style="font-size:11px;color:var(--text3);font-family:var(--mono)">{{ p.server }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

          </div>
        </div>
      </div>
    </div>

    <!-- ═══════════════ 上传配置 Tab ═══════════════ -->
    <div v-if="tab==='configs'" class="page">
      <div class="page-inner">
        <div class="card-row" style="gap:16px;align-items:flex-start">

          <!-- Upload area -->
          <div class="card" style="flex:1">
            <div class="card-title">上传配置文件</div>
            <div class="upload-area"
              :class="{dragging}"
              @dragover.prevent="dragging=true"
              @dragleave="dragging=false"
              @drop.prevent="handleDrop">
              <input type="file" ref="fileInput" accept=".yaml,.yml" style="display:none" @change="handleFileChange">
              <div style="font-size:28px;margin-bottom:8px">📂</div>
              <div style="font-weight:600;color:var(--text2);margin-bottom:4px">拖放 YAML 配置文件</div>
              <div style="font-size:12px;color:var(--text3);margin-bottom:12px">或点击选择文件 (.yaml / .yml)</div>
              <button class="btn btn-primary btn-sm" @click="fileInput.click()">选择文件</button>
            </div>
            <div v-if="uploading" class="alert alert-info mt-8">上传中…</div>
            <div v-if="uploadResult" class="alert mt-8" :class="uploadResult.ok ? 'alert-success':'alert-error'">
              {{ uploadResult.msg }}
            </div>
          </div>

          <!-- Config list -->
          <div class="card" style="flex:1">
            <div style="display:flex;align-items:center;margin-bottom:12px">
              <span class="card-title" style="margin:0">已上传配置（{{ configs.length }}）</span>
              <button class="btn btn-secondary btn-sm" style="margin-left:auto" @click="loadConfigs">刷新</button>
            </div>
            <div v-if="!configs.length" class="empty-state">暂无上传配置</div>
            <div v-else style="display:flex;flex-direction:column;gap:8px">
              <div v-for="c in configs" :key="c.filename" class="config-item">
                <div style="display:flex;align-items:center;gap:8px">
                  <span style="font-size:18px">📄</span>
                  <div style="flex:1;min-width:0">
                    <div style="font-size:13px;font-weight:600;font-family:var(--mono);word-break:break-all">{{ c.filename }}</div>
                    <div style="font-size:11.5px;color:var(--text3);margin-top:2px">
                      {{ fmtSize(c.size) }} · {{ fmtDate(c.updatedAt) }}
                    </div>
                  </div>
                  <button class="del-btn" @click="deleteConfig(c.filename)" title="删除">✕</button>
                </div>
                <div v-if="c.inbounds && c.inbounds.length" style="display:flex;flex-wrap:wrap;gap:4px;margin-top:6px">
                  <span v-for="ib in c.inbounds" :key="ib.type"
                    style="font-size:11px;padding:2px 7px;background:var(--accent-bg);color:var(--accent);border-radius:4px;font-family:var(--mono)">
                    {{ ib.type }}{{ ib.port ? ':'+ib.port : '' }}
                  </span>
                </div>
                <div style="margin-top:8px;display:flex;gap:6px">
                  <button class="btn btn-secondary btn-sm" @click="viewConfig(c.filename)">查看</button>
                  <button class="btn btn-secondary btn-sm" @click="editConfig(c.filename)">编辑</button>
                </div>
              </div>
            </div>
          </div>

        </div>
      </div>
    </div>

    <!-- 查看/编辑配置弹窗 -->
    <div v-if="editingConfig" class="mask" @click.self="editingConfig=null">
      <div class="modal" style="max-width:700px">
        <div class="modal-head">
          <span>{{ editingConfig.filename }}</span>
          <div style="display:flex;gap:8px;align-items:center">
            <button class="btn btn-primary btn-sm" @click="saveConfig" :disabled="savingConfig">
              {{ savingConfig ? '保存中…' : '保存' }}
            </button>
            <button class="btn btn-secondary btn-sm" @click="editingConfig=null">关闭</button>
          </div>
        </div>
        <div style="padding:14px 18px;flex:1;overflow:auto">
          <textarea class="input" v-model="editingConfig.content" rows="22"
            style="font-family:var(--mono);font-size:12.5px;resize:vertical"></textarea>
        </div>
        <div v-if="saveError" class="alert alert-error" style="margin:0 18px 12px">{{ saveError }}</div>
      </div>
    </div>

  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { api } from '../api.js'
import { useNodesStore, useSubsStore } from '../stores.js'

const nodesStore = useNodesStore()
const subsStore  = useSubsStore()

const tab = ref('nodes')

// ── Import nodes ───────────────────────────────────────────────────────────
const importText   = ref('')
const importing    = ref(false)
const importResult = ref(null)

async function doImport() {
  importing.value = true; importResult.value = null
  try {
    const r = await nodesStore.importNodes(importText.value)
    importResult.value = { ok: true, msg: `成功导入 ${r.imported} 个节点${r.errors?.length ? '，' + r.errors.length + ' 个失败' : ''}` }
    importText.value = ''
    setTimeout(() => importResult.value = null, 4000)
  } catch (e) {
    importResult.value = { ok: false, msg: e.message }
  }
  importing.value = false
}

async function clearAllNodes() {
  if (!confirm('确认清空所有节点？')) return
  for (const n of [...nodesStore.nodes]) await nodesStore.del(n.id)
}

// ── Subscriptions ──────────────────────────────────────────────────────────
const subForm    = reactive({ name: '', url: '' })
const addingSub  = ref(false)
const subError   = ref('')
const updatingSub = ref('')
const expandedSub = ref('')
const subProxies  = reactive({})

async function doAddSub() {
  addingSub.value = true; subError.value = ''
  try {
    await subsStore.add(subForm.name || subForm.url, subForm.url)
    subForm.name = ''; subForm.url = ''
  } catch (e) { subError.value = e.message }
  addingSub.value = false
}

async function doUpdateSub(id) {
  updatingSub.value = id
  try {
    await subsStore.update(id)
    if (expandedSub.value === id) {
      subProxies[id] = await subsStore.getProxies(id)
    }
  } catch {}
  updatingSub.value = ''
}

async function toggleSubDetail(id) {
  if (expandedSub.value === id) { expandedSub.value = ''; return }
  expandedSub.value = id
  if (!subProxies[id]) {
    try { subProxies[id] = await subsStore.getProxies(id) } catch { subProxies[id] = [] }
  }
}

// ── Upload configs ─────────────────────────────────────────────────────────
const configs      = ref([])
const fileInput    = ref(null)
const dragging     = ref(false)
const uploading    = ref(false)
const uploadResult = ref(null)
const editingConfig = ref(null)
const savingConfig  = ref(false)
const saveError     = ref('')

async function loadConfigs() {
  try { configs.value = await api('GET', '/config/list') } catch {}
}

async function uploadFile(file) {
  if (!file) return
  if (!file.name.endsWith('.yaml') && !file.name.endsWith('.yml')) {
    uploadResult.value = { ok: false, msg: '只支持 .yaml / .yml 文件' }
    return
  }
  uploading.value = true; uploadResult.value = null
  const fd = new FormData(); fd.append('config', file)
  try {
    const res = await fetch('/api/config', {
      method: 'POST',
      headers: { 'X-Auth-Token': localStorage.getItem('metaviz_token') || '' },
      body: fd,
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '上传失败')
    uploadResult.value = { ok: true, msg: `已上传：${data.filename}` }
    await loadConfigs()
    setTimeout(() => uploadResult.value = null, 4000)
  } catch (e) { uploadResult.value = { ok: false, msg: e.message } }
  uploading.value = false
}

function handleFileChange(e) { uploadFile(e.target.files[0]); e.target.value = '' }
function handleDrop(e) { dragging.value = false; uploadFile(e.dataTransfer.files[0]) }

async function deleteConfig(filename) {
  if (!confirm(`确认删除 ${filename}？`)) return
  try { await api('DELETE', '/config/' + filename); await loadConfigs() } catch (e) { alert(e.message) }
}

async function viewConfig(filename) {
  try {
    const res = await fetch('/api/config/raw/' + filename, {
      headers: { 'X-Auth-Token': localStorage.getItem('metaviz_token') || '' }
    })
    const content = await res.text()
    editingConfig.value = { filename, content }
    saveError.value = ''
  } catch (e) { alert(e.message) }
}

async function editConfig(filename) { await viewConfig(filename) }

async function saveConfig() {
  if (!editingConfig.value) return
  savingConfig.value = true; saveError.value = ''
  try {
    await fetch('/api/config/raw/' + editingConfig.value.filename, {
      method: 'PUT',
      headers: {
        'Content-Type': 'text/plain',
        'X-Auth-Token': localStorage.getItem('metaviz_token') || ''
      },
      body: editingConfig.value.content
    }).then(async r => {
      if (!r.ok) { const d = await r.json(); throw new Error(d.error) }
    })
    await loadConfigs()
    editingConfig.value = null
  } catch (e) { saveError.value = e.message }
  savingConfig.value = false
}

// ── Utils ──────────────────────────────────────────────────────────────────
function fmtDate(d) {
  if (!d) return ''
  return new Date(d).toLocaleString('zh-CN', { month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit' })
}
function fmtSize(b) {
  if (!b) return '0 B'
  if (b < 1024) return b + ' B'
  if (b < 1048576) return (b/1024).toFixed(1) + ' KB'
  return (b/1048576).toFixed(1) + ' MB'
}

onMounted(async () => {
  await Promise.all([nodesStore.load(), subsStore.load(), loadConfigs()])
})
</script>

<style scoped>
.sub-card {
  background:var(--surface2); border:1px solid var(--border);
  border-radius:var(--radius); padding:12px 14px;
}
.config-item {
  background:var(--surface2); border:1px solid var(--border);
  border-radius:var(--radius); padding:12px 14px;
}
.upload-area {
  border:2px dashed var(--border2); border-radius:var(--radius-lg);
  padding:32px 20px; text-align:center; cursor:pointer;
  transition:all .15s; background:var(--surface2);
}
.upload-area:hover, .upload-area.dragging {
  border-color:var(--accent); background:var(--accent-bg);
}

.mask {
  position:fixed; inset:0; background:rgba(26,8,8,.5); z-index:300;
  display:flex; align-items:center; justify-content:center; backdrop-filter:blur(2px);
}
.modal {
  background:var(--surface); border-radius:var(--radius-lg);
  box-shadow:var(--shadow-lg); width:90%; max-width:560px;
  max-height:88vh; display:flex; flex-direction:column;
}
.modal-head {
  display:flex; align-items:center; justify-content:space-between;
  padding:14px 18px; border-bottom:1px solid var(--border);
  font-size:15px; font-weight:700; color:var(--text);
  flex-shrink:0;
}
</style>
