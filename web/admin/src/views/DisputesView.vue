<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RefreshRight, Document, Edit, Check, Lock, View } from '@element-plus/icons-vue'
import api, { type DisputeInfo, type EscrowInfo, type EvidenceInfo } from '@/api'

const activeTab = ref('disputes')
const loading = ref(true)

// 争议列表
const disputes = ref<DisputeInfo[]>([])

// 托管记录
const escrows = ref<EscrowInfo[]>([])

// 证据
const evidenceList = ref<EvidenceInfo[]>([])
const selectedDispute = ref<DisputeInfo | null>(null)
const showEvidenceDialog = ref(false)

// 创建争议
const showCreateDialog = ref(false)
const createForm = ref({
  taskId: '',
  respondentId: '',
  description: '',
  amount: 0
})

onMounted(async () => {
  await fetchData()
})

async function fetchData() {
  loading.value = true
  try {
    const [dispData, escData] = await Promise.all([
      api.getDisputeList(),
      api.getEscrowList()
    ])
    disputes.value = dispData.disputes || []
    escrows.value = escData.escrows || []
  } catch (e) {
    console.error('Failed to fetch data:', e)
    ElMessage.error('获取数据失败')
  }
  loading.value = false
}

function openCreate() {
  createForm.value = { taskId: '', respondentId: '', description: '', amount: 0 }
  showCreateDialog.value = true
}

async function createDispute() {
  if (!createForm.value.taskId || !createForm.value.respondentId) {
    ElMessage.warning('请填写必填项')
    return
  }
  try {
    await api.createDispute(createForm.value)
    showCreateDialog.value = false
    ElMessage.success('争议已创建')
    await fetchData()
  } catch (e) {
    console.error('Create dispute failed:', e)
    ElMessage.error('创建争议失败')
  }
}

async function viewEvidence(dispute: DisputeInfo) {
  selectedDispute.value = dispute
  try {
    const data = await api.getEvidence(dispute.dispute_id)
    evidenceList.value = data.evidence || []
    showEvidenceDialog.value = true
  } catch (e) {
    console.error('Get evidence failed:', e)
    ElMessage.error('获取证据失败')
  }
}

async function resolveDispute(disputeId: string, favor: 'plaintiff' | 'respondent') {
  const favorLabel = favor === 'plaintiff' ? '原告' : '被告'
  try {
    await ElMessageBox.confirm(`确定要判定 ${favorLabel} 胜诉吗？`, '解决争议', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await api.resolveDispute(disputeId, favor)
    ElMessage.success('争议已解决')
    await fetchData()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Resolve dispute failed:', e)
      ElMessage.error('解决争议失败')
    }
  }
}

async function releaseEscrow(escrowId: string) {
  try {
    await ElMessageBox.confirm('确定要释放该托管资金吗？', '释放托管', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info'
    })
    await api.releaseEscrow(escrowId)
    ElMessage.success('托管资金已释放')
    await fetchData()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Release escrow failed:', e)
      ElMessage.error('释放失败')
    }
  }
}

async function refundEscrow(escrowId: string) {
  try {
    await ElMessageBox.confirm('确定要退款该托管资金吗？', '退款', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await api.refundEscrow(escrowId)
    ElMessage.success('已退款')
    await fetchData()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Refund escrow failed:', e)
      ElMessage.error('退款失败')
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

function getStatusType(status: string): string {
  switch (status) {
    case 'resolved': return 'success'
    case 'pending': return 'warning'
    case 'in_progress': return 'primary'
    case 'released': return 'success'
    case 'refunded': return 'info'
    case 'locked': return 'danger'
    default: return 'info'
  }
}

function getStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    resolved: '已解决',
    pending: '待处理',
    in_progress: '处理中',
    released: '已释放',
    refunded: '已退款',
    locked: '锁定中'
  }
  return labels[status] || status
}
</script>

<template>
  <div class="disputes-view">
    <!-- 操作栏 -->
    <div class="toolbar">
      <el-button type="primary" :icon="Document" @click="openCreate">创建争议</el-button>
      <el-button :icon="RefreshRight" @click="fetchData" :loading="loading">刷新</el-button>
    </div>

    <el-tabs v-model="activeTab" class="tabs">
      <!-- 争议列表 -->
      <el-tab-pane label="争议处理" name="disputes">
        <el-table :data="disputes" v-loading="loading" stripe>
          <el-table-column label="争议 ID" width="160">
            <template #default="{ row }">
              <span class="id-mono">{{ shortenId(row.dispute_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="任务 ID" width="160">
            <template #default="{ row }">
              <span class="id-mono">{{ shortenId(row.task_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="原告" width="140">
            <template #default="{ row }">
              <span class="id-mono">{{ shortenId(row.plaintiff_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="被告" width="140">
            <template #default="{ row }">
              <span class="id-mono">{{ shortenId(row.respondent_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="争议金额" prop="amount" width="100" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" size="small">
                {{ getStatusLabel(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="160">
            <template #default="{ row }">
              {{ formatTime(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="280" fixed="right">
            <template #default="{ row }">
              <el-button-group>
                <el-button type="info" size="small" :icon="View" @click="viewEvidence(row)">证据</el-button>
                <el-button 
                  type="success" 
                  size="small" 
                  @click="resolveDispute(row.dispute_id, 'plaintiff')"
                  :disabled="row.status === 'resolved'"
                >
                  原告胜
                </el-button>
                <el-button 
                  type="warning" 
                  size="small" 
                  @click="resolveDispute(row.dispute_id, 'respondent')"
                  :disabled="row.status === 'resolved'"
                >
                  被告胜
                </el-button>
              </el-button-group>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="disputes.length === 0" description="暂无争议记录" />
      </el-tab-pane>

      <!-- 托管记录 -->
      <el-tab-pane label="资金托管" name="escrows">
        <el-table :data="escrows" v-loading="loading" stripe>
          <el-table-column label="托管 ID" width="160">
            <template #default="{ row }">
              <span class="id-mono">{{ shortenId(row.escrow_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="任务 ID" width="160">
            <template #default="{ row }">
              <span class="id-mono">{{ shortenId(row.task_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="付款方" width="140">
            <template #default="{ row }">
              <span class="id-mono">{{ shortenId(row.payer_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="收款方" width="140">
            <template #default="{ row }">
              <span class="id-mono">{{ shortenId(row.payee_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="托管金额" prop="amount" width="100" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" size="small">
                {{ getStatusLabel(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="160">
            <template #default="{ row }">
              {{ formatTime(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <el-button-group v-if="row.status === 'locked'">
                <el-button type="success" size="small" :icon="Check" @click="releaseEscrow(row.escrow_id)">释放</el-button>
                <el-button type="warning" size="small" :icon="Lock" @click="refundEscrow(row.escrow_id)">退款</el-button>
              </el-button-group>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="escrows.length === 0" description="暂无托管记录" />
      </el-tab-pane>
    </el-tabs>

    <!-- 创建争议对话框 -->
    <el-dialog v-model="showCreateDialog" title="创建争议" width="500px">
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="任务 ID" required>
          <el-input v-model="createForm.taskId" placeholder="输入关联的任务 ID" />
        </el-form-item>
        <el-form-item label="被告 ID" required>
          <el-input v-model="createForm.respondentId" placeholder="输入被告节点 ID" />
        </el-form-item>
        <el-form-item label="争议金额">
          <el-input-number v-model="createForm.amount" :min="0" />
        </el-form-item>
        <el-form-item label="争议描述">
          <el-input v-model="createForm.description" type="textarea" :rows="4" placeholder="描述争议的详细情况" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="createDispute">创建</el-button>
      </template>
    </el-dialog>

    <!-- 证据查看对话框 -->
    <el-dialog v-model="showEvidenceDialog" title="争议证据" width="600px">
      <el-descriptions v-if="selectedDispute" :column="2" border style="margin-bottom: 16px">
        <el-descriptions-item label="争议 ID">{{ shortenId(selectedDispute.dispute_id) }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ getStatusLabel(selectedDispute.status) }}</el-descriptions-item>
      </el-descriptions>
      
      <el-timeline v-if="evidenceList.length > 0">
        <el-timeline-item
          v-for="(ev, idx) in evidenceList"
          :key="idx"
          :timestamp="formatTime(ev.submitted_at)"
          placement="top"
        >
          <el-card shadow="never">
            <div class="evidence-header">
              <strong>提交方：</strong>
              <span class="id-mono">{{ shortenId(ev.submitter_id) }}</span>
            </div>
            <div class="evidence-type">
              <el-tag size="small">{{ ev.type }}</el-tag>
            </div>
            <div class="evidence-content">{{ ev.content }}</div>
          </el-card>
        </el-timeline-item>
      </el-timeline>
      <el-empty v-else description="暂无证据记录" />
      
      <template #footer>
        <el-button @click="showEvidenceDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.disputes-view {
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

.id-mono {
  font-family: monospace;
  color: var(--el-color-primary);
}

.text-muted {
  color: var(--el-text-color-placeholder);
}

.evidence-header {
  margin-bottom: 8px;
}

.evidence-type {
  margin-bottom: 8px;
}

.evidence-content {
  color: var(--el-text-color-regular);
  white-space: pre-wrap;
}
</style>
