<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-brand">
        <div class="login-logo">M</div>
        <div>
          <div class="login-title">MetaViz</div>
          <div class="login-sub">mihomo 代理管理面板</div>
        </div>
      </div>

      <!-- Setup mode -->
      <template v-if="needsSetup">
        <div class="alert alert-info" style="margin-bottom:16px">首次使用，请设置管理员账号</div>
        <div class="form-group">
          <label class="form-label">用户名</label>
          <input class="input" v-model="form.username" placeholder="admin" @keydown.enter="doSetup">
        </div>
        <div class="form-group">
          <label class="form-label">密码</label>
          <input class="input" type="password" v-model="form.password" placeholder="设置密码" @keydown.enter="doSetup">
        </div>
        <button class="btn btn-primary w-full" :disabled="loading" @click="doSetup">
          {{ loading ? '设置中…' : '完成设置' }}
        </button>
      </template>

      <!-- Login mode -->
      <template v-else>
        <div class="form-group">
          <label class="form-label">用户名</label>
          <input class="input" v-model="form.username" placeholder="用户名" autofocus @keydown.enter="doLogin">
        </div>
        <div class="form-group">
          <label class="form-label">密码</label>
          <input class="input" type="password" v-model="form.password" placeholder="密码" @keydown.enter="doLogin">
        </div>
        <button class="btn btn-primary w-full" :disabled="loading" @click="doLogin">
          {{ loading ? '登录中…' : '登录' }}
        </button>
      </template>

      <div v-if="errMsg" class="alert alert-error" style="margin-top:12px">{{ errMsg }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores.js'

const router    = useRouter()
const authStore = useAuthStore()
const form      = ref({ username: '', password: '' })
const loading   = ref(false)
const errMsg    = ref('')
const needsSetup = ref(false)

onMounted(async () => {
  try {
    const res = await fetch('/api/auth/status')
    const d   = await res.json()
    if (!d.enabled) { router.replace('/dashboard'); return }
    needsSetup.value = d.needsSetup
  } catch {}
})

async function doLogin() {
  errMsg.value = ''; loading.value = true
  try {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value),
    })
    const d = await res.json()
    if (!res.ok) throw new Error(d.error || '登录失败')
    authStore.setToken(d.token)
    router.replace('/dashboard')
  } catch (e) { errMsg.value = e.message }
  loading.value = false
}

async function doSetup() {
  if (!form.value.username || !form.value.password) {
    errMsg.value = '用户名和密码不能为空'; return
  }
  errMsg.value = ''; loading.value = true
  try {
    const res = await fetch('/api/auth/setup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value),
    })
    const d = await res.json()
    if (!res.ok) throw new Error(d.error || '设置失败')
    authStore.setToken(d.token)
    router.replace('/dashboard')
  } catch (e) { errMsg.value = e.message }
  loading.value = false
}
</script>

<style scoped>
.login-page {
  min-height: 100vh; display: flex; align-items: center; justify-content: center;
  background: var(--bg); padding: 24px;
}
.login-card {
  background: var(--surface); border: 1px solid var(--border);
  border-radius: var(--radius-lg); padding: 32px 28px;
  width: 100%; max-width: 380px; box-shadow: var(--shadow-lg);
}
.login-brand {
  display: flex; align-items: center; gap: 14px; margin-bottom: 28px;
}
.login-logo {
  width: 44px; height: 44px; border-radius: 12px;
  background: var(--accent); color: #fff;
  display: flex; align-items: center; justify-content: center;
  font-weight: 800; font-size: 22px; flex-shrink: 0;
}
.login-title { font-size: 20px; font-weight: 800; color: var(--text); line-height: 1.2; }
.login-sub   { font-size: 12px; color: var(--text3); margin-top: 2px; }
</style>
