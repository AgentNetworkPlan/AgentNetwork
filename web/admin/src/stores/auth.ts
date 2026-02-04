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
      console.log('[AuthStore] Starting login process...')
      console.log('[AuthStore] Token:', token ? token.substring(0, 8) + '...' : 'empty')
      console.log('[AuthStore] Calling API login...')
      const response = await api.login(token)
      console.log('[AuthStore] Raw API response type:', typeof response)
      console.log('[AuthStore] Raw API response:', response)
      
      if (response && response.success) {
        console.log('[AuthStore] Login successful!')
        console.log('[AuthStore] Session ID:', response.session_id)
        console.log('[AuthStore] Expires at:', response.expires_at)
        
        isAuthenticated.value = true
        localStorage.setItem('daan_authenticated', 'true')
        error.value = null
        
        console.log('[AuthStore] Updated authentication state')
        console.log('[AuthStore] Fetching node status...')
        await fetchNodeStatus()
        console.log('[AuthStore] Login process completed successfully')
        return true
      } else {
        error.value = (response && response.error) || '登录失败'
        console.log('[AuthStore] Login failed:', error.value)
        console.log('[AuthStore] Response success field:', response ? response.success : 'undefined')
        return false
      }
    } catch (e: any) {
      console.error('[AuthStore] Login exception:', e)
      console.error('[AuthStore] Exception stack:', e.stack)
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
      console.log('[AuthStore] Fetching node status...')
      nodeStatus.value = await api.getNodeStatus()
      console.log('[AuthStore] Node status fetched successfully:', nodeStatus.value)
    } catch (e) {
      console.error('[AuthStore] Failed to fetch node status:', e)
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
