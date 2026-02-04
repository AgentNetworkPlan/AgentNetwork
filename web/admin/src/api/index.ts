import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  timeout: 10000,
  withCredentials: true
})

// Response interceptor for error handling
client.interceptors.response.use(
  response => response.data,
  error => {
    if (error.response?.status === 401) {
      localStorage.removeItem('daan_authenticated')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export interface LoginResponse {
  success: boolean
  session_id?: string
  expires_at?: string
  error?: string
}

export interface HealthResponse {
  status: string
  timestamp: string
  version: string
}

export interface NodeStatus {
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

export interface PeersResponse {
  count: number
  peers: string[]
}

export interface TopologyNode {
  id: string
  name: string
  type: 'self' | 'peer' | 'supernode' | 'genesis'
  x?: number
  y?: number
  reputation: number
  status: 'online' | 'offline' | 'unknown'
}

export interface TopologyLink {
  source: string
  target: string
  latency: number
  status: 'active' | 'inactive'
}

export interface Topology {
  nodes: TopologyNode[]
  links: TopologyLink[]
  updated_at: string
}

export interface APIEndpoint {
  method: string
  path: string
  description: string
  category: string
}

export interface LogEntry {
  timestamp: string
  level: string
  module: string
  message: string
}

export interface NetworkStats {
  total_peers: number
  active_peers: number
  messages_sent: number
  messages_received: number
  bytes_sent: number
  bytes_received: number
  avg_latency_ms: number
}

const api = {
  // Auth
  login: (token: string): Promise<LoginResponse> => 
    client.post('/auth/login', { token }),
  
  logout: (): Promise<{ success: boolean }> => 
    client.post('/auth/logout'),
  
  refreshToken: (): Promise<{ success: boolean; expires_at: string }> =>
    client.post('/auth/token/refresh'),

  // Health
  getHealth: (): Promise<HealthResponse> => 
    client.get('/health'),

  // Node
  getNodeStatus: (): Promise<NodeStatus> => 
    client.get('/node/status'),
  
  getPeers: (): Promise<PeersResponse> => 
    client.get('/node/peers'),
  
  getConfig: (): Promise<any> => 
    client.get('/node/config'),

  // Topology
  getTopology: (): Promise<Topology> => 
    client.get('/topology'),

  // API Browser
  getEndpoints: (): Promise<{ count: number; endpoints: APIEndpoint[] }> => 
    client.get('/endpoints'),

  // Logs
  getLogs: (limit = 100): Promise<{ count: number; logs: LogEntry[] }> => 
    client.get(`/logs?limit=${limit}`),

  // Stats
  getStats: (): Promise<NetworkStats> => 
    client.get('/stats'),
}

export default api

// WebSocket connections
export function createWebSocket(channel: 'topology' | 'logs' | 'stats'): WebSocket {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return new WebSocket(`${protocol}//${host}/ws/${channel}`)
}
