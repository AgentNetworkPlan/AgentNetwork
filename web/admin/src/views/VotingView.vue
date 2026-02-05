<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, RefreshRight, Check, Delete, View } from '@element-plus/icons-vue'
import api, { type Proposal } from '@/api'

const proposals = ref<Proposal[]>([])
const loading = ref(true)
const activeTab = ref('all')

// 创建提案对话框
const createDialogVisible = ref(false)
const createForm = ref({
  title: '',
  description: '',
  options: 'yes,no,abstain',
  deadline: ''
})
const createLoading = ref(false)

// 提案详情对话框
const detailDialogVisible = ref(false)
const selectedProposal = ref<Proposal | null>(null)

onMounted(async () => {
  await fetchProposals()
})

async function fetchProposals() {
  loading.value = true
  try {
    const data = await api.getProposalList()
    proposals.value = data.proposals || []
  } catch (e) {
    console.error('Failed to fetch proposals:', e)
    ElMessage.error('获取提案列表失败')
  }
  loading.value = false
}

const filteredProposals = computed(() => {
  if (activeTab.value === 'all') return proposals.value
  return proposals.value.filter(p => p.status === activeTab.value)
})

function showCreateDialog() {
  createForm.value = { title: '', description: '', options: 'yes,no,abstain', deadline: '' }
  createDialogVisible.value = true
}

async function createProposal() {
  if (!createForm.value.title) {
    ElMessage.warning('请输入提案标题')
    return
  }
  
  createLoading.value = true
  try {
    const options = createForm.value.options.split(',').map(o => o.trim()).filter(o => o)
    const deadline = createForm.value.deadline || new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString()
    await api.createProposal({
      title: createForm.value.title,
      description: createForm.value.description,
      options,
      deadline
    })
    ElMessage.success('提案已创建')
    createDialogVisible.value = false
    await fetchProposals()
  } catch (e) {
    console.error('Create proposal failed:', e)
    ElMessage.error('创建提案失败')
  }
  createLoading.value = false
}

async function vote(proposalId: string, voteType: 'yes' | 'no' | 'abstain') {
  try {
    await api.vote(proposalId, voteType)
    ElMessage.success('投票成功')
    await fetchProposals()
  } catch (e) {
    console.error('Vote failed:', e)
    ElMessage.error('投票失败')
  }
}

async function finalizeProposal(proposalId: string) {
  try {
    await ElMessageBox.confirm('确定要结束此提案吗？', '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    await api.finalizeProposal(proposalId)
    ElMessage.success('提案已结束')
    await fetchProposals()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Finalize failed:', e)
      ElMessage.error('结束提案失败')
    }
  }
}

function showProposalDetail(proposal: Proposal) {
  selectedProposal.value = proposal
  detailDialogVisible.value = true
}

function getStatusType(status: string): string {
  switch (status) {
    case 'pending': return 'warning'
    case 'passed': return 'success'
    case 'rejected': return 'danger'
    case 'expired': return 'info'
    default: return 'info'
  }
}

function getStatusLabel(status: string): string {
  switch (status) {
    case 'pending': return '投票中'
    case 'passed': return '已通过'
    case 'rejected': return '已拒绝'
    case 'expired': return '已过期'
    default: return status
  }
}

function getTypeLabel(type: string): string {
  switch (type) {
    case 'general': return '一般'
    case 'kick': return '踢除节点'
    case 'upgrade': return '升级'
    case 'parameter': return '参数调整'
    default: return type
  }
}

function formatTime(ts: string): string {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}
</script>

<template>
  <div class="voting-view">
    <!-- 操作栏 -->
    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="showCreateDialog">创建提案</el-button>
      <el-button :icon="RefreshRight" @click="fetchProposals" :loading="loading">刷新</el-button>
    </div>

    <!-- 标签页 -->
    <el-tabs v-model="activeTab" class="tabs">
      <el-tab-pane label="全部" name="all" />
      <el-tab-pane label="投票中" name="pending" />
      <el-tab-pane label="已通过" name="passed" />
      <el-tab-pane label="已拒绝" name="rejected" />
    </el-tabs>

    <!-- 提案列表 -->
    <el-table :data="filteredProposals" v-loading="loading" stripe>
      <el-table-column label="标题" prop="title" min-width="200" show-overflow-tooltip />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ getStatusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="投票" width="200">
        <template #default="{ row }">
          <div class="vote-stats">
            <span class="vote-yes">👍 {{ row.yes_votes || 0 }}</span>
            <span class="vote-no">👎 {{ row.no_votes || 0 }}</span>
            <span class="vote-abstain">➖ {{ row.abstain_votes || 0 }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="280" fixed="right">
        <template #default="{ row }">
          <el-button-group size="small">
            <el-button :icon="View" @click="showProposalDetail(row)">详情</el-button>
            <template v-if="row.status === 'pending'">
              <el-button type="success" :icon="Check" @click="vote(row.proposal_id, 'yes')">赞成</el-button>
              <el-button type="danger" :icon="Delete" @click="vote(row.proposal_id, 'no')">反对</el-button>
              <el-button @click="finalizeProposal(row.proposal_id)">结束</el-button>
            </template>
          </el-button-group>
        </template>
      </el-table-column>
    </el-table>

    <!-- 创建提案对话框 -->
    <el-dialog v-model="createDialogVisible" title="创建提案" width="500px">
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="标题">
          <el-input v-model="createForm.title" placeholder="提案标题" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createForm.description" type="textarea" :rows="3" placeholder="提案详细描述" />
        </el-form-item>
        <el-form-item label="选项">
          <el-input v-model="createForm.options" placeholder="选项列表，用逗号分隔，如: yes,no,abstain" />
        </el-form-item>
        <el-form-item label="截止时间">
          <el-date-picker v-model="createForm.deadline" type="datetime" placeholder="选择截止时间" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="createProposal" :loading="createLoading">创建</el-button>
      </template>
    </el-dialog>

    <!-- 提案详情对话框 -->
    <el-dialog v-model="detailDialogVisible" title="提案详情" width="600px">
      <template v-if="selectedProposal">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="提案 ID" :span="2">{{ selectedProposal.proposal_id }}</el-descriptions-item>
          <el-descriptions-item label="标题" :span="2">{{ selectedProposal.title }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusType(selectedProposal.status)">{{ getStatusLabel(selectedProposal.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="描述" :span="2">{{ selectedProposal.description || '-' }}</el-descriptions-item>
          <el-descriptions-item label="选项" :span="2">{{ selectedProposal.options?.join(', ') || '-' }}</el-descriptions-item>
          <el-descriptions-item label="投票情况" :span="2">
            <span v-for="(count, opt) in selectedProposal.votes" :key="opt" style="margin-right: 12px">
              {{ opt }}: {{ count }}
            </span>
          </el-descriptions-item>
          <el-descriptions-item label="截止时间">{{ formatTime(selectedProposal.deadline) }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatTime(selectedProposal.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="提案人" :span="2">{{ selectedProposal.proposer_id || '-' }}</el-descriptions-item>
        </el-descriptions>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.voting-view {
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

.vote-stats {
  display: flex;
  gap: 12px;
  font-size: 13px;
}

.vote-yes {
  color: var(--el-color-success);
}

.vote-no {
  color: var(--el-color-danger);
}

.vote-abstain {
  color: var(--el-color-info);
}
</style>
