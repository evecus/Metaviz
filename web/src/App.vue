<template>
  <div class="app">
    <div v-if="sidebarOpen && !isPublicRoute" class="sidebar-overlay" @click="sidebarOpen = false"></div>

    <aside v-if="!isPublicRoute" class="sidebar" :class="{ open: sidebarOpen }">
      <div class="sidebar-brand">
        <div class="brand-logo">M</div>
        <div class="brand-text">
          <span class="brand-name">MetaViz</span>
          <span class="brand-ver">v1.0</span>
        </div>
        <button class="sidebar-close" @click="sidebarOpen = false">✕</button>
      </div>

      <nav class="sidebar-nav">
        <span class="nav-section-label">主要</span>
        <RouterLink to="/dashboard" class="nav-item" active-class="active" @click="sidebarOpen = false">
          <span class="nav-icon">⬡</span><span>仪表盘</span>
        </RouterLink>
        <RouterLink to="/nodes" class="nav-item" active-class="active" @click="sidebarOpen = false">
          <span class="nav-icon">⬢</span><span>节点与配置</span>
        </RouterLink>

        <span class="nav-section-label">系统</span>
        <RouterLink to="/settings" class="nav-item" active-class="active" @click="sidebarOpen = false">
          <span class="nav-icon">⚙</span><span>设置</span>
        </RouterLink>
      </nav>

      <div class="sidebar-status">
        <div class="status-row" :class="'status-' + statusStore.status.state">
          <div class="status-dot"></div>
          <span class="status-label">{{ stateLabel }}</span>
          <span v-if="statusStore.status.pid" class="text-xs text-muted monospace" style="margin-left:auto">
            {{ statusStore.status.pid }}
          </span>
        </div>
        <button v-if="authEnabled" class="nav-item" style="width:100%;margin-top:4px;color:var(--text3);font-size:12px" @click="doLogout">
          <span class="nav-icon" style="font-size:13px">⎋</span><span>退出登录</span>
        </button>
      </div>
    </aside>

    <main class="main" :class="{ 'main-full': isPublicRoute }">
      <button v-if="!isPublicRoute" class="hamburger-fab" @click="sidebarOpen = true" aria-label="打开菜单">
        <span></span><span></span><span></span>
      </button>
      <RouterView />
    </main>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useStatusStore, useNodesStore, useSubsStore, useLogsStore, useAuthStore } from './stores.js'
import { api } from './api.js'

const statusStore = useStatusStore()
const nodesStore  = useNodesStore()
const subsStore   = useSubsStore()
const logsStore   = useLogsStore()
const authStore   = useAuthStore()
const router      = useRouter()
const route       = useRoute()

const isPublicRoute = computed(() => !!route.meta?.public)
const authEnabled = ref(false)
const sidebarOpen = ref(false)

async function checkAuthEnabled() {
  try {
    const s = await fetch('/api/auth/status')
    const d = await s.json()
    authEnabled.value = d.enabled
  } catch {}
}

async function doLogout() {
  try { await api('POST', '/auth/logout') } catch {}
  authStore.setToken('')
  router.push('/login')
}

const stateLabel = computed(() => ({
  running: '运行中', stopped: '已停止', error: '错误',
}[statusStore.status.state] || statusStore.status.state))

let poll = null
onMounted(async () => {
  checkAuthEnabled()
  await statusStore.fetch()
  await Promise.all([nodesStore.load(), subsStore.load()])
  if (statusStore.isRunning) logsStore.startSSE()
  poll = setInterval(statusStore.fetch, 60000)
})
onUnmounted(() => { clearInterval(poll); logsStore.stopSSE() })
</script>
