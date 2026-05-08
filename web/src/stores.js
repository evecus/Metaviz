import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from './api.js'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('metaviz_token') || '')
  function setToken(t) {
    token.value = t
    if (t) localStorage.setItem('metaviz_token', t)
    else localStorage.removeItem('metaviz_token')
  }
  return { token, setToken }
})

export const useStatusStore = defineStore('status', () => {
  const status = ref({ state: 'stopped' })
  const isRunning = computed(() => status.value.state === 'running')
  async function fetch() {
    try { status.value = await api('GET', '/status') } catch {}
  }
  return { status, isRunning, fetch }
})

export const useNodesStore = defineStore('nodes', () => {
  const nodes = ref([])
  async function load() {
    try { nodes.value = await api('GET', '/nodes') } catch {}
  }
  async function importNodes(text) {
    const r = await api('POST', '/nodes/import', { text })
    await load()
    return r
  }
  async function del(id) {
    await api('DELETE', '/nodes/' + id)
    await load()
  }
  return { nodes, load, importNodes, del }
})

export const useSubsStore = defineStore('subs', () => {
  const subs = ref([])
  async function load() {
    try { subs.value = await api('GET', '/subscriptions') } catch {}
  }
  async function add(name, url) {
    const r = await api('POST', '/subscriptions', { name, url })
    await load()
    return r
  }
  async function del(id) {
    await api('DELETE', '/subscriptions/' + id)
    await load()
  }
  async function update(id) {
    const r = await api('POST', '/subscriptions/' + id + '/update')
    await load()
    return r
  }
  async function getProxies(id) {
    return api('GET', '/subscriptions/' + id + '/proxies')
  }
  return { subs, load, add, del, update, getProxies }
})

export const useLogsStore = defineStore('logs', () => {
  const lines = ref([])
  let es = null
  function startSSE() {
    if (es) return
    const token = localStorage.getItem('metaviz_token') || ''
    es = new EventSource('/api/logs' + (token ? '?token=' + token : ''))
    es.onmessage = (e) => {
      lines.value.push(e.data)
      if (lines.value.length > 500) lines.value.shift()
    }
    es.onerror = () => { stopSSE() }
  }
  function stopSSE() {
    if (es) { es.close(); es = null }
  }
  function clear() { lines.value = [] }
  return { lines, startSSE, stopSSE, clear }
})
