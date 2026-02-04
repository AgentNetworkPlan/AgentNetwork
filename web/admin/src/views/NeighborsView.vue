<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, RefreshRight, Plus, Delete, Position } from '@element-plus/icons-vue'
import api, { type NeighborInfo } from '@/api'

const neighbors = ref<NeighborInfo[]>([])
const loading = ref(true)
const pingLoading = ref<string | null>(null)

// 添加邻居对话框
const addDialogVisible = ref(false)
const addForm = ref({
  node_id: '',
  addresses: ''
})
const addLoading = ref(false)

onMounted(async () => {
  await fetchNeighbors()
})

async function fetchNeighbors() {
  loading.value = true
  try {
    const data = await api.getNeighborList()
    neighbors.value = data.neighbors || []
  } catch (e) {
    console.error('Failed to fetch neighbors:', e)
    ElMessage.error('获取邻居列表失败')
  }
  loading.value = false
}

async function pingNeighbor(nodeId: string) {
  pingLoading.value = nodeId
  try {
    const result = await api.pingNeighbor(nodeId)
    if (result.online) {
      ElMessage.success(`节点在线，延迟: ${result.latency_ms}ms`)
    } else {
      ElMessage.warning(`节点离线: ${result.error || '无响应'}`)
    }
    // 刷新列表以更新状态
    await fetchNeighbors()
  } catch (e) {
    console.error('Ping failed:', e)
    ElMessage.error('Ping 失败')
  }
  pingLoading.value = null
}

async function removeNeighbor(nodeId: string) {
  try {
    await ElMessageBox.confirm(
      `确定要移除邻居节点 ${shortenId(nodeId)} 吗？`,
      '确认移除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    
    await api.removeNeighbor(nodeId)
    ElMessage.success('邻居已移除')
    await fetchNeighbors()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Remove failed:', e)
      ElMessage.error('移除邻居失败')
    }
  }
}

function showAddDialog() {
  addForm.value = { node_id: '', addresses: '' }
  addDialogVisible.value = true
}

async function addNeighbor() {
  if (!addForm.value.node_id) {
    ElMessage.warning('请输入节点 ID')
    return
  }
  
  addLoading.value = true
  try {
    const addresses = addForm.value.addresses
      ? addForm.value.addresses.split('\n').map(a => a.trim()).filter(a => a)
      : []
    
    await api.addNeighbor(addForm.value.node_id, addresses)
    ElMessage.success('邻居已添加')
    addDialogVisible.value = false
    await fetchNeighbors()
  } catch (e) {
    console.error('Add failed:', e)
    ElMessage.error('添加邻居失败')
  }
  addLoading.value = false
}

function shortenId(id: string): string {
  if (!id || id.length <= 16) return id || '-'
  return id.slice(0, 8) + '...' + id.slice(-6)
}

function getStatusType(status: string): string {
  switch (status) {
    case 'online': return 'success'
    case 'offline': return 'danger'
    default: return 'info'
  }
}

function getTypeLabel(type: string): string {
  switch (type) {
    case 'normal': return '普通'
    case 'super': return '超级节点'
    case 'relay': return '中继'
    default: return type || '-'
  }
}

const onlineCount = computed(() => 
  neighbors.value.filter(n => n.status === 'online').length
)
</script>

<template>
  <div class="neighbors-page">
    <div class="page-header">
      <h2><el-icon><Connection /></el-icon> 邻居管理</h2>
      <div class="header-actions">
        <el-button :icon="RefreshRight" @click="fetchNeighbors" :loading="loading">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="showAddDialog">添加邻居</el-button>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-row">
      <el-card class="stat-card">
        <div class="stat-value">{{ neighbors.length }}</div>
        <div class="stat-label">总邻居数</div>
      </el-card>
      <el-card class="stat-card online">
        <div class="stat-value">{{ onlineCount }}</div>
        <div class="stat-label">在线</div>
      </el-card>
      <el-card class="stat-card offline">
        <div class="stat-value">{{ neighbors.length - onlineCount }}</div>
        <div class="stat-label">离线</div>
      </el-card>
    </div>

    <!-- 邻居列表 -->
    <el-card class="neighbors-table" v-loading="loading">
      <el-table :data="neighbors" stripe style="width: 100%">
        <el-table-column label="节点 ID" min-width="180">
          <template #default="{ row }">
            <code class="node-id" :title="row.node_id">{{ shortenId(row.node_id) }}</code>
          </template>
        </el-table-column>
        
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.type === 'super' ? 'warning' : 'info'" size="small">
              {{ getTypeLabel(row.type) }}
            </el-tag>
          </template>
        </el-table-column>
        
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ row.status === 'online' ? '在线' : row.status === 'offline' ? '离线' : '未知' }}
            </el-tag>
          </template>
        </el-table-column>
        
        <el-table-column label="信任分" width="100">
          <template #default="{ row }">
            <el-progress 
              :percentage="(row.trust_score || 0) * 100" 
              :stroke-width="6"
              :show-text="false"
              :color="[
                { color: '#f56c6c', percentage: 30 },
                { color: '#e6a23c', percentage: 60 },
                { color: '#67c23a', percentage: 100 }
              ]"
            />
            <span class="trust-text">{{ ((row.trust_score || 0) * 100).toFixed(0) }}%</span>
          </template>
        </el-table-column>
        
        <el-table-column label="声誉" width="80">
          <template #default="{ row }">
            {{ row.reputation || 0 }}
          </template>
        </el-table-column>
        
        <el-table-column label="Ping 统计" width="120">
          <template #default="{ row }">
            <span class="ping-stats">
              <span class="success">✓{{ row.success_pings || 0 }}</span>
              <span class="fail">✗{{ row.failed_pings || 0 }}</span>
            </span>
          </template>
        </el-table-column>
        
        <el-table-column label="最后在线" width="160">
          <template #default="{ row }">
            {{ row.last_seen || '-' }}
          </template>
        </el-table-column>
        
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button 
              size="small" 
              :icon="Position"
              @click="pingNeighbor(row.node_id)"
              :loading="pingLoading === row.node_id"
              title="Ping"
            />
            <el-button 
              size="small" 
              type="danger"
              :icon="Delete"
              @click="removeNeighbor(row.node_id)"
              title="移除"
            />
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && neighbors.length === 0" description="暂无邻居节点" />
    </el-card>

    <!-- 添加邻居对话框 -->
    <el-dialog v-model="addDialogVisible" title="添加邻居" width="500px">
      <el-form :model="addForm" label-width="100px">
        <el-form-item label="节点 ID" required>
          <el-input 
            v-model="addForm.node_id" 
            placeholder="输入节点公钥或ID"
          />
        </el-form-item>
        <el-form-item label="地址列表">
          <el-input 
            v-model="addForm.addresses" 
            type="textarea"
            :rows="3"
            placeholder="可选，每行一个地址，如:&#10;/ip4/192.168.1.100/tcp/18345"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="addNeighbor" :loading="addLoading">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.neighbors-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: #fff;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  background: var(--el-bg-color-overlay);
  text-align: center;
  padding: 10px 0;
}

.stat-card .stat-value {
  font-size: 32px;
  font-weight: bold;
  color: #fff;
}

.stat-card .stat-label {
  font-size: 14px;
  color: #999;
  margin-top: 4px;
}

.stat-card.online .stat-value {
  color: #67c23a;
}

.stat-card.offline .stat-value {
  color: #f56c6c;
}

.neighbors-table {
  background: var(--el-bg-color-overlay);
}

.node-id {
  font-family: 'Consolas', monospace;
  background: rgba(0, 0, 0, 0.2);
  padding: 2px 6px;
  border-radius: 4px;
  cursor: pointer;
}

.trust-text {
  font-size: 12px;
  color: #999;
  margin-left: 8px;
}

.ping-stats {
  display: flex;
  gap: 8px;
  font-size: 13px;
}

.ping-stats .success {
  color: #67c23a;
}

.ping-stats .fail {
  color: #f56c6c;
}
</style>
