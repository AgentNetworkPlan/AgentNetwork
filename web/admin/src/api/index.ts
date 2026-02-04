import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  timeout: 10000,
  withCredentials: true
})

// Response interceptor for error handling
client.interceptors.response.use(
  response => {
    console.log('[API] Response interceptor - success:', response.status, response.config?.url)
    console.log('[API] Response data:', response.data)
    return response.data
  },
  error => {
    console.log('[API] Response interceptor - error:', error.response?.status, error.config?.url)
    console.log('[API] Error response:', error.response)
    
    // Don't redirect to login if we're already on the login page or doing login request
    const isLoginRequest = error.config?.url?.includes('/auth/login')
    const isOnLoginPage = window.location.pathname === '/login'
    
    console.log('[API] Error handling - isLoginRequest:', isLoginRequest, 'isOnLoginPage:', isOnLoginPage)
    
    if (error.response?.status === 401) {
      if (isLoginRequest || isOnLoginPage) {
        // For login requests or when already on login page, return error data for handling
        console.log('[API] Returning error response data for login handling:', error.response.data)
        if (error.response?.data) {
          return Promise.resolve(error.response.data)
        }
      } else {
        // For other 401 errors, redirect to login and clear auth state
        console.log('[API] Redirecting to login due to 401')
        localStorage.removeItem('daan_authenticated')
        window.location.href = '/login'
        return Promise.reject(error)
      }
    }
    
    console.log('[API] Rejecting error:', error)
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

// ========== 邻居管理类型 ==========
export interface NeighborInfo {
  node_id: string
  public_key?: string
  type: string
  reputation: number
  trust_score: number
  status: string
  last_seen?: string
  addresses?: string[]
  success_pings: number
  failed_pings: number
}

export interface PingResult {
  node_id: string
  online: boolean
  latency_ms: number
  error?: string
}

// ========== 邮箱类型 ==========
export interface MailSummary {
  id: string
  from: string
  to: string
  subject: string
  timestamp: string
  status: string
  encrypted: boolean
}

export interface MailMessage {
  id: string
  from: string
  to: string
  subject: string
  content: string
  timestamp: string
  status: string
  encrypted: boolean
  read_at?: string
}

export interface MailboxResponse {
  messages: MailSummary[]
  total: number
  offset: number
  limit: number
}

// ========== 留言板类型 ==========
export interface BulletinMessage {
  message_id: string
  author: string
  topic: string
  content: string
  preview?: string
  timestamp: string
  expires_at?: string
  status: string
  tags?: string[]
  reply_to?: string
  reputation: number
}

// ========== 声誉类型 ==========
export interface ReputationInfo {
  node_id: string
  reputation: number
  rank?: number
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

  // ========== 邻居管理 API ==========
  getNeighborList: (): Promise<{ neighbors: NeighborInfo[]; count: number }> =>
    client.get('/neighbor/list'),
  
  getBestNeighbors: (count = 5): Promise<{ neighbors: NeighborInfo[]; count: number }> =>
    client.get(`/neighbor/best?count=${count}`),
  
  addNeighbor: (nodeId: string, addresses: string[]): Promise<{ status: string }> =>
    client.post('/neighbor/add', { node_id: nodeId, addresses }),
  
  removeNeighbor: (nodeId: string): Promise<{ status: string }> =>
    client.post('/neighbor/remove', { node_id: nodeId }),
  
  pingNeighbor: (nodeId: string): Promise<PingResult> =>
    client.post('/neighbor/ping', { node_id: nodeId }),

  // ========== 邮箱 API ==========
  sendMail: (to: string, subject: string, content: string): Promise<{ message_id: string; status: string }> =>
    client.post('/mailbox/send', { to, subject, content }),
  
  getMailboxInbox: (limit = 20, offset = 0): Promise<MailboxResponse> =>
    client.get(`/mailbox/inbox?limit=${limit}&offset=${offset}`),
  
  getMailboxOutbox: (limit = 20, offset = 0): Promise<MailboxResponse> =>
    client.get(`/mailbox/outbox?limit=${limit}&offset=${offset}`),
  
  readMail: (messageId: string): Promise<MailMessage> =>
    client.get(`/mailbox/read/${messageId}`),
  
  markMailRead: (messageId: string): Promise<{ status: string }> =>
    client.post('/mailbox/mark-read', { message_id: messageId }),
  
  deleteMail: (messageId: string): Promise<{ status: string }> =>
    client.post('/mailbox/delete', { message_id: messageId }),

  // ========== 留言板 API ==========
  publishBulletin: (topic: string, content: string, ttl: number): Promise<{ message_id: string; topic: string; status: string }> =>
    client.post('/bulletin/publish', { topic, content, ttl }),
  
  getBulletinByTopic: (topic: string, limit = 20): Promise<{ messages: BulletinMessage[]; count: number; topic: string }> =>
    client.get(`/bulletin/topic/${encodeURIComponent(topic)}?limit=${limit}`),
  
  getBulletinByAuthor: (author: string, limit = 20): Promise<{ messages: BulletinMessage[]; count: number; author: string }> =>
    client.get(`/bulletin/author/${encodeURIComponent(author)}?limit=${limit}`),
  
  searchBulletin: (keyword: string, limit = 20): Promise<{ messages: BulletinMessage[]; count: number; keyword: string }> =>
    client.get(`/bulletin/search?keyword=${encodeURIComponent(keyword)}&limit=${limit}`),
  
  subscribeBulletin: (topic: string): Promise<{ status: string }> =>
    client.post('/bulletin/subscribe', { topic }),
  
  unsubscribeBulletin: (topic: string): Promise<{ status: string }> =>
    client.post('/bulletin/unsubscribe', { topic }),
  
  revokeBulletin: (messageId: string): Promise<{ status: string }> =>
    client.post('/bulletin/revoke', { message_id: messageId }),
  
  getBulletinSubscriptions: (): Promise<{ topics: string[]; count: number }> =>
    client.get('/bulletin/subscriptions'),

  // ========== 声誉 API ==========
  getReputation: (nodeId: string): Promise<ReputationInfo> =>
    client.get(`/reputation/query?node_id=${encodeURIComponent(nodeId)}`),
  
  getReputationRanking: (limit = 10): Promise<{ rankings: ReputationInfo[]; count: number }> =>
    client.get(`/reputation/ranking?limit=${limit}`),

  // ========== 消息 API ==========
  sendDirectMessage: (to: string, type: string, content: string): Promise<{ message_id: string; status: string }> =>
    client.post('/message/send', { to, type, content }),
  
  broadcastMessage: (content: string): Promise<{ message_id: string; reached_count: number }> =>
    client.post('/message/broadcast', { content }),
}

export default api

// WebSocket connections
export function createWebSocket(channel: 'topology' | 'logs' | 'stats'): WebSocket {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return new WebSocket(`${protocol}//${host}/ws/${channel}`)
}
