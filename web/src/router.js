import { createRouter, createWebHashHistory } from 'vue-router'
import DashboardView from './views/DashboardView.vue'
import NodesView     from './views/NodesView.vue'
import SettingsView  from './views/SettingsView.vue'
import LoginView     from './views/LoginView.vue'

const routes = [
  { path: '/',          redirect: '/dashboard' },
  { path: '/login',     component: LoginView, meta: { public: true } },
  { path: '/dashboard', component: DashboardView },
  { path: '/nodes',     component: NodesView },
  { path: '/settings',  component: SettingsView },
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  try {
    const res = await fetch('/api/auth/status')
    const status = await res.json()
    if (!status.enabled) return true
    const token = localStorage.getItem('metaviz_token')
    if (!token) return '/login'
    if (status.needsSetup) return '/login'
    return true
  } catch {
    return true
  }
})
