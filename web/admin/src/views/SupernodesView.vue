<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RefreshRight, Bell, User, Check, Timer, Position } from '@element-plus/icons-vue'
import api, { type SupernodeInfo, type CandidateInfo } from '@/api'

const supernodes = ref<SupernodeInfo[]>([])
const candidates = ref<CandidateInfo[]>([])
const loading = ref(true)
const activeTab = ref('supernodes')

// 申请候选人
const applyLoading = ref(false)
const stakeAmount = ref(1000)

onMounted(async () => {
  await fetchData()
})

async function fetchData() {
  loading.value = true
  try {
    const [snData, candData] = await Promise.all([
      api.getSupernodeList(),
      api.getCandidateList()
    ])
    supernodes.value = snData.supernodes || []
    candidates.value = candData.candidates || []
  } catch (e) {
    console.error('Failed to fetch data:', e)
    ElMessage.error('获取数据失败')
  }
  loading.value = false
}

async function applyAsCandidate() {
  const { value: stake } = await ElMessageBox.prompt('请输入抵押金额', '申请成为候选人', {
    confirmButtonText: '申请',
    cancelButtonText: '取消',
    inputValue: '1000',
    inputPattern: /^\d+$/,
    inputErrorMessage: '请输入有效数字'
  }).catch(() => ({ value: null }))
  
  if (stake) {
    applyLoading.value = true
    try {
      await api.applyAsCandidate(parseInt(stake))
      ElMessage.success('申请已提交')
      await fetchData()
    } catch (e) {
      console.error('Apply failed:', e)
      ElMessage.error('申请失败')
    }
    applyLoading.value = false
  }
}

async function withdrawCandidate() {
  try {
    await ElMessageBox.confirm('确定要撤销候选资格吗？', '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.withdrawCandidate()
    ElMessage.success('已撤销候选资格')
    await fetchData()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Withdraw failed:', e)
      ElMessage.error('撤销失败')
    }
  }
}

async function voteForCandidate(candidateId: string) {
  try {
    await api.voteForCandidate(candidateId)
    ElMessage.success('投票成功')
    await fetchData()
  } catch (e) {
    console.error('Vote failed:', e)
    ElMessage.error('投票失败')
  }
}

async function startElection() {
  try {
    await ElMessageBox.confirm('确定要启动新一轮选举吗？', '启动选举', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info'
    })
    
    await api.startElection()
    ElMessage.success('选举已启动')
    await fetchData()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Start election failed:', e)
      ElMessage.error('启动选举失败')
    }
  }
}

function shortenId(id: string): string {
  if (!id || id.length <= 16) return id || '-'
  return id.slice(0, 8) + '...' + id.slice(-6)
}

function formatTime(ts: string): string {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}
</script>

<template>
  <div class="supernodes-view">
    <!-- 操作栏 -->
    <div class="toolbar">
      <el-button type="primary" :icon="Position" @click="applyAsCandidate" :loading="applyLoading">
        申请成为候选人
      </el-button>
      <el-button type="warning" @click="withdrawCandidate">撤销候选</el-button>
      <el-button type="success" :icon="Timer" @click="startElection">启动选举</el-button>
      <el-button :icon="RefreshRight" @click="fetchData" :loading="loading">刷新</el-button>
    </div>

    <!-- 标签页 -->
    <el-tabs v-model="activeTab" class="tabs">
      <el-tab-pane label="超级节点" name="supernodes">
        <el-row :gutter="16">
          <el-col :span="8" v-for="sn in supernodes" :key="sn.node_id">
            <el-card class="supernode-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <el-icon class="star-icon" color="#f7ba2a"><Bell /></el-icon>
                  <span class="node-id">{{ shortenId(sn.node_id) }}</span>
                </div>
              </template>
              <el-descriptions :column="1" size="small">
                <el-descriptions-item label="任期">第 {{ sn.term }} 期</el-descriptions-item>
                <el-descriptions-item label="声誉">{{ sn.reputation?.toFixed(2) || '-' }}</el-descriptions-item>
                <el-descriptions-item label="抵押">{{ sn.stake || 0 }}</el-descriptions-item>
                <el-descriptions-item label="状态">
                  <el-tag :type="sn.status === 'active' ? 'success' : 'info'" size="small">
                    {{ sn.status === 'active' ? '活跃' : sn.status }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="当选时间">{{ formatTime(sn.elected_at) }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col v-if="supernodes.length === 0" :span="24">
            <el-empty description="暂无超级节点" />
          </el-col>
        </el-row>
      </el-tab-pane>

      <el-tab-pane label="候选人" name="candidates">
        <el-table :data="candidates" v-loading="loading" stripe>
          <el-table-column label="节点 ID" width="200">
            <template #default="{ row }">
              <span class="node-id-mono">{{ shortenId(row.node_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="抵押金额" prop="stake" width="120" />
          <el-table-column label="得票数" prop="votes" width="100" />
          <el-table-column label="声誉" width="100">
            <template #default="{ row }">
              {{ row.reputation?.toFixed(2) || '-' }}
            </template>
          </el-table-column>
          <el-table-column label="申请时间" width="160">
            <template #default="{ row }">
              {{ formatTime(row.applied_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" size="small" :icon="Check" @click="voteForCandidate(row.node_id)">
                投票
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="candidates.length === 0" description="暂无候选人" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.supernodes-view {
  padding: 20px;
}

.toolbar {
  margin-bottom: 16px;
  display: flex;
  gap: 12px;
}

.tabs {
  margin-bottom: 16px;
}

.supernode-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.star-icon {
  font-size: 20px;
}

.node-id, .node-id-mono {
  font-family: monospace;
  color: var(--el-color-primary);
}
</style>
