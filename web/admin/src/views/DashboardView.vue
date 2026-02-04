<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import api, { createWebSocket, type NetworkStats } from '@/api'
import { Connection, Timer, Upload, Download, Monitor } from '@element-plus/icons-vue'

const authStore = useAuthStore()

const stats = ref<NetworkStats | null>(null)
const peers = ref<string[]>([])
const peerCount = ref(0)
const loading = ref(true)

let statsWs: WebSocket | null = null

onMounted(async () => {
  await fetchData()
  setupWebSocket()
})

onUnmounted(() => {
  if (statsWs) {
    statsWs.close()
  }
})

async function fetchData() {
  loading.value = true
  try {
    const [statsData, peersData] = await Promise.all([
      api.getStats(),
      api.getPeers()
    ])
    stats.value = statsData
    peers.value = peersData.peers
    peerCount.value = peersData.count
    await authStore.fetchNodeStatus()
  } catch (e) {
    console.error('Failed to fetch data:', e)
  }
  loading.value = false
}

function setupWebSocket() {
  statsWs = createWebSocket('stats')
  statsWs.onmessage = (event) => {
    try {
      stats.value = JSON.parse(event.data)
    } catch (e) {
      console.error('Failed to parse stats:', e)
    }
  }
  statsWs.onclose = () => {
    // Reconnect after 5 seconds
    setTimeout(setupWebSocket, 5000)
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function shortenId(id: string): string {
  if (id.length <= 16) return id
  return id.slice(0, 8) + '...' + id.slice(-6)
}
</script>

<template>
  <div class="dashboard" v-loading="loading">
    <!-- Node Info Card -->
    <div class="info-section">
      <el-card class="info-card">
        <template #header>
          <div class="card-header">
            <span>节点信息</span>
            <el-tag 
              :type="authStore.nodeStatus ? 'success' : 'danger'"
              effect="dark"
            >
              {{ authStore.nodeStatus ? '在线' : '离线' }}
            </el-tag>
          </div>
        </template>
        
        <el-descriptions :column="2" border v-if="authStore.nodeStatus">
          <el-descriptions-item label="节点 ID">
            <code>{{ shortenId(authStore.nodeStatus.node_id) }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="版本">
            {{ authStore.nodeStatus.version }}
          </el-descriptions-item>
          <el-descriptions-item label="运行时间">
            {{ authStore.nodeStatus.uptime }}
          </el-descriptions-item>
          <el-descriptions-item label="声誉值">
            <el-progress 
              :percentage="authStore.nodeStatus.reputation * 100" 
              :color="[
                { color: '#f56c6c', percentage: 30 },
                { color: '#e6a23c', percentage: 60 },
                { color: '#67c23a', percentage: 100 }
              ]"
            />
          </el-descriptions-item>
          <el-descriptions-item label="P2P 端口">
            {{ authStore.nodeStatus.p2p_port }}
          </el-descriptions-item>
          <el-descriptions-item label="HTTP 端口">
            {{ authStore.nodeStatus.http_port }}
          </el-descriptions-item>
          <el-descriptions-item label="创世节点">
            <el-tag :type="authStore.nodeStatus.is_genesis ? 'warning' : 'info'" size="small">
              {{ authStore.nodeStatus.is_genesis ? '是' : '否' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="超级节点">
            <el-tag :type="authStore.nodeStatus.is_supernode ? 'warning' : 'info'" size="small">
              {{ authStore.nodeStatus.is_supernode ? '是' : '否' }}
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>
    </div>

    <!-- Stats Cards -->
    <div class="stats-grid">
      <div class="stat-card">
        <el-icon :size="32" color="#4fc3f7"><Connection /></el-icon>
        <div class="stat-value">{{ peerCount }}</div>
        <div class="stat-label">连接节点</div>
      </div>

      <div class="stat-card">
        <el-icon :size="32" color="#67c23a"><Upload /></el-icon>
        <div class="stat-value">{{ stats ? formatBytes(stats.bytes_sent) : '-' }}</div>
        <div class="stat-label">发送流量</div>
      </div>

      <div class="stat-card">
        <el-icon :size="32" color="#e6a23c"><Download /></el-icon>
        <div class="stat-value">{{ stats ? formatBytes(stats.bytes_received) : '-' }}</div>
        <div class="stat-label">接收流量</div>
      </div>

      <div class="stat-card">
        <el-icon :size="32" color="#f56c6c"><Timer /></el-icon>
        <div class="stat-value">{{ stats ? stats.avg_latency_ms.toFixed(1) + ' ms' : '-' }}</div>
        <div class="stat-label">平均延迟</div>
      </div>
    </div>

    <!-- Peers List -->
    <el-card class="peers-card">
      <template #header>
        <div class="card-header">
          <span>连接的节点 ({{ peerCount }})</span>
          <el-button size="small" @click="fetchData">刷新</el-button>
        </div>
      </template>

      <el-table :data="peers.slice(0, 10)" stripe style="width: 100%">
        <el-table-column prop="id" label="节点 ID">
          <template #default="{ row }">
            <code>{{ shortenId(row) }}</code>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default>
            <el-tag type="success" size="small">在线</el-tag>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="peers.length > 10" class="more-peers">
        还有 {{ peers.length - 10 }} 个节点...
        <router-link to="/topology">查看网络拓扑</router-link>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.dashboard {
  max-width: 1200px;
  margin: 0 auto;
}

.info-section {
  margin-bottom: 24px;
}

.info-card {
  background: var(--el-bg-color-overlay);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  padding: 24px;
  text-align: center;
}

.stat-card .stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #fff;
  margin: 12px 0 8px;
}

.stat-card .stat-label {
  font-size: 14px;
  color: #999;
}

.peers-card {
  background: var(--el-bg-color-overlay);
}

.more-peers {
  text-align: center;
  padding: 16px;
  color: #999;
}

.more-peers a {
  color: var(--el-color-primary);
  margin-left: 8px;
}

code {
  font-family: 'Consolas', monospace;
  background: rgba(0, 0, 0, 0.2);
  padding: 2px 6px;
  border-radius: 4px;
}
</style>
