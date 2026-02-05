<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RefreshRight, Edit, Delete, Document, Position } from '@element-plus/icons-vue'
import api, { type AuditDeviation, type CollateralInfo, type PenaltyConfig } from '@/api'

const activeTab = ref('deviations')
const loading = ref(true)

// 审计偏差
const deviations = ref<AuditDeviation[]>([])

// 抵押物
const collaterals = ref<CollateralInfo[]>([])

// 惩罚配置
const penaltyConfig = ref<PenaltyConfig>({
  reputation_threshold: 0,
  reputation_penalty_rate: 0,
  collateral_penalty_rate: 0,
  cooldown_hours: 0
})

// 编辑惩罚配置
const showPenaltyDialog = ref(false)
const editingConfig = ref<PenaltyConfig>({ ...penaltyConfig.value })

// 手动惩罚
const showPenaltyNodeDialog = ref(false)
const penaltyForm = ref({
  nodeId: '',
  reason: '',
  amount: 100
})

onMounted(async () => {
  await fetchData()
})

async function fetchData() {
  loading.value = true
  try {
    const [devData, collData, confData] = await Promise.all([
      api.getAuditDeviations(),
      api.getCollateralList(),
      api.getPenaltyConfig()
    ])
    deviations.value = devData.deviations || []
    collaterals.value = collData.collaterals || []
    penaltyConfig.value = confData.config || penaltyConfig.value
  } catch (e) {
    console.error('Failed to fetch audit data:', e)
    ElMessage.error('获取审计数据失败')
  }
  loading.value = false
}

function openPenaltyConfig() {
  editingConfig.value = { ...penaltyConfig.value }
  showPenaltyDialog.value = true
}

async function savePenaltyConfig() {
  try {
    await api.updatePenaltyConfig(editingConfig.value)
    penaltyConfig.value = { ...editingConfig.value }
    showPenaltyDialog.value = false
    ElMessage.success('惩罚配置已更新')
  } catch (e) {
    console.error('Update config failed:', e)
    ElMessage.error('更新配置失败')
  }
}

function openPenaltyNode() {
  penaltyForm.value = { nodeId: '', reason: '', amount: 100 }
  showPenaltyNodeDialog.value = true
}

async function applyPenalty() {
  if (!penaltyForm.value.nodeId) {
    ElMessage.warning('请输入节点 ID')
    return
  }
  try {
    await api.applyManualPenalty(penaltyForm.value.nodeId, penaltyForm.value.reason, penaltyForm.value.amount)
    showPenaltyNodeDialog.value = false
    ElMessage.success('已对节点施加惩罚')
    await fetchData()
  } catch (e) {
    console.error('Apply penalty failed:', e)
    ElMessage.error('施加惩罚失败')
  }
}

async function revokeCollateral(nodeId: string) {
  try {
    await ElMessageBox.confirm(`确定要撤销节点 ${nodeId.slice(0, 8)}... 的抵押物吗？`, '确认撤销', {
      confirmButtonText: '撤销',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await api.revokeCollateral(nodeId)
    ElMessage.success('抵押物已撤销')
    await fetchData()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Revoke failed:', e)
      ElMessage.error('撤销失败')
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

function getSeverityType(severity: string): string {
  switch (severity) {
    case 'critical': return 'danger'
    case 'high': return 'warning'
    case 'medium': return 'primary'
    default: return 'info'
  }
}
</script>

<template>
  <div class="audit-view">
    <!-- 操作栏 -->
    <div class="toolbar">
      <el-button type="warning" :icon="Edit" @click="openPenaltyNode">手动惩罚</el-button>
      <el-button type="primary" :icon="Document" @click="openPenaltyConfig">惩罚配置</el-button>
      <el-button :icon="RefreshRight" @click="fetchData" :loading="loading">刷新</el-button>
    </div>

    <!-- 标签页 -->
    <el-tabs v-model="activeTab" class="tabs">
      <!-- 审计偏差 -->
      <el-tab-pane label="审计偏差" name="deviations">
        <el-table :data="deviations" v-loading="loading" stripe>
          <el-table-column label="节点 ID" width="200">
            <template #default="{ row }">
              <span class="node-id">{{ shortenId(row.node_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="偏差类型" prop="type" width="150" />
          <el-table-column label="严重程度" width="100">
            <template #default="{ row }">
              <el-tag :type="getSeverityType(row.severity)" size="small">
                {{ row.severity }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="描述" prop="description" min-width="200" show-overflow-tooltip />
          <el-table-column label="发现时间" width="160">
            <template #default="{ row }">
              {{ formatTime(row.detected_at) }}
            </template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.resolved ? 'success' : 'warning'" size="small">
                {{ row.resolved ? '已处理' : '待处理' }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="deviations.length === 0" description="暂无审计偏差" />
      </el-tab-pane>

      <!-- 抵押物管理 -->
      <el-tab-pane label="抵押物管理" name="collaterals">
        <el-table :data="collaterals" v-loading="loading" stripe>
          <el-table-column label="节点 ID" width="200">
            <template #default="{ row }">
              <span class="node-id">{{ shortenId(row.node_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="抵押金额" prop="amount" width="120">
            <template #default="{ row }">
              <el-icon><Position /></el-icon> {{ row.amount }}
            </template>
          </el-table-column>
          <el-table-column label="锁定状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.locked ? 'danger' : 'success'" size="small">
                {{ row.locked ? '锁定' : '可用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="抵押时间" width="160">
            <template #default="{ row }">
              {{ formatTime(row.deposited_at) }}
            </template>
          </el-table-column>
          <el-table-column label="到期时间" width="160">
            <template #default="{ row }">
              {{ formatTime(row.expires_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button type="danger" size="small" :icon="Delete" @click="revokeCollateral(row.node_id)">
                撤销
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="collaterals.length === 0" description="暂无抵押记录" />
      </el-tab-pane>

      <!-- 惩罚配置 -->
      <el-tab-pane label="惩罚配置" name="config">
        <el-card shadow="never">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="声誉阈值">{{ penaltyConfig.reputation_threshold }}</el-descriptions-item>
            <el-descriptions-item label="声誉惩罚比例">{{ (penaltyConfig.reputation_penalty_rate * 100).toFixed(1) }}%</el-descriptions-item>
            <el-descriptions-item label="抵押物惩罚比例">{{ (penaltyConfig.collateral_penalty_rate * 100).toFixed(1) }}%</el-descriptions-item>
            <el-descriptions-item label="冷却时间">{{ penaltyConfig.cooldown_hours }} 小时</el-descriptions-item>
          </el-descriptions>
          <el-button type="primary" style="margin-top: 16px" @click="openPenaltyConfig">修改配置</el-button>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 惩罚配置对话框 -->
    <el-dialog v-model="showPenaltyDialog" title="惩罚配置" width="450px">
      <el-form :model="editingConfig" label-width="120px">
        <el-form-item label="声誉阈值">
          <el-input-number v-model="editingConfig.reputation_threshold" :min="0" :max="100" />
        </el-form-item>
        <el-form-item label="声誉惩罚比例">
          <el-slider v-model="editingConfig.reputation_penalty_rate" :min="0" :max="1" :step="0.01" show-input />
        </el-form-item>
        <el-form-item label="抵押物惩罚比例">
          <el-slider v-model="editingConfig.collateral_penalty_rate" :min="0" :max="1" :step="0.01" show-input />
        </el-form-item>
        <el-form-item label="冷却时间(小时)">
          <el-input-number v-model="editingConfig.cooldown_hours" :min="0" :max="720" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showPenaltyDialog = false">取消</el-button>
        <el-button type="primary" @click="savePenaltyConfig">保存</el-button>
      </template>
    </el-dialog>

    <!-- 手动惩罚对话框 -->
    <el-dialog v-model="showPenaltyNodeDialog" title="手动惩罚节点" width="450px">
      <el-form :model="penaltyForm" label-width="80px">
        <el-form-item label="节点 ID" required>
          <el-input v-model="penaltyForm.nodeId" placeholder="输入节点 ID" />
        </el-form-item>
        <el-form-item label="惩罚原因">
          <el-input v-model="penaltyForm.reason" type="textarea" :rows="3" placeholder="描述惩罚原因" />
        </el-form-item>
        <el-form-item label="惩罚金额">
          <el-input-number v-model="penaltyForm.amount" :min="1" :max="10000" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showPenaltyNodeDialog = false">取消</el-button>
        <el-button type="danger" @click="applyPenalty">确认惩罚</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.audit-view {
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

.node-id {
  font-family: monospace;
  color: var(--el-color-primary);
}
</style>
