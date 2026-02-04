import { defineStore } from 'pinia'
import { ref } from 'vue'
import api from '@/api'

interface NodeStatus {
  node_id: string
  public_key: string
  start_time: string
  uptime: string
  version: string
  p2p_port: number
  http_port: number
  grpc_port: number
  admin_port: number
  is_genesis: boolean
  is_supernode: boolean
  reputation: number
  token_count: number
}

export const useAuthStore = defineStore('auth', () => {
  const isAuthenticated = ref(localStorage.getItem('daan_authenticated') === 'true')
  const nodeStatus = ref<NodeStatus | null>(null)
  const error = ref<string | null>(null)

  async function login(token: string): Promise<boolean> {
    try {
      const response = await api.login(token)
      if (response.success) {
        isAuthenticated.value = true
        localStorage.setItem('daan_authenticated', 'true')
        error.value = null
        // Fetch node status after login
        await fetchNodeStatus()
        return true
      } else {
        error.value = response.error || '登录失败'
        return false
      }
    } catch (e: any) {
      error.value = e.message || '网络错误'
      return false
    }
  }

  async function logout(): Promise<void> {
    try {
      await api.logout()
    } catch {
      // Ignore logout errors
    }
    isAuthenticated.value = false
    nodeStatus.value = null
    localStorage.removeItem('daan_authenticated')
  }

  async function fetchNodeStatus(): Promise<void> {
    try {
      nodeStatus.value = await api.getNodeStatus()
    } catch {
      // Ignore errors
    }
  }

  async function checkAuth(): Promise<boolean> {
    try {
      const health = await api.getHealth()
      if (health.status === 'healthy') {
        await fetchNodeStatus()
        return isAuthenticated.value
      }
    } catch {
      // API not available
    }
    return false
  }

  return {
    isAuthenticated,
    nodeStatus,
    error,
    login,
    logout,
    fetchNodeStatus,
    checkAuth
  }
})
