<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, RefreshRight, Check, View, Timer } from '@element-plus/icons-vue'
import api, { type TaskInfo } from '@/api'

const tasks = ref<TaskInfo[]>([])
const loading = ref(true)
const activeTab = ref('all')

// 创建任务对话框
const createDialogVisible = ref(false)
const createForm = ref({
  type: 'compute',
  description: '',
  target: '',
  payload: ''
})
const createLoading = ref(false)

// 任务详情对话框
const detailDialogVisible = ref(false)
const selectedTask = ref<TaskInfo | null>(null)

onMounted(async () => {
  await fetchTasks()
})

async function fetchTasks() {
  loading.value = true
  try {
    const data = await api.getTaskList()
    tasks.value = data.tasks || []
  } catch (e) {
    console.error('Failed to fetch tasks:', e)
    ElMessage.error('获取任务列表失败')
  }
  loading.value = false
}

const filteredTasks = computed(() => {
  if (activeTab.value === 'all') return tasks.value
  return tasks.value.filter(t => t.status === activeTab.value)
})

function showCreateDialog() {
  createForm.value = { type: 'compute', description: '', target: '', payload: '' }
  createDialogVisible.value = true
}

async function createTask() {
  if (!createForm.value.description) {
    ElMessage.warning('请输入任务描述')
    return
  }
  
  createLoading.value = true
  try {
    let payload = {}
    if (createForm.value.payload) {
      try {
        payload = JSON.parse(createForm.value.payload)
      } catch {
        ElMessage.warning('Payload 格式错误，请输入有效 JSON')
        createLoading.value = false
        return
      }
    }
    
    await api.createTask({
      type: createForm.value.type,
      description: createForm.value.description
    })
    ElMessage.success('任务已创建')
    createDialogVisible.value = false
    await fetchTasks()
  } catch (e) {
    console.error('Create task failed:', e)
    ElMessage.error('创建任务失败')
  }
  createLoading.value = false
}

async function acceptTask(taskId: string) {
  try {
    await api.acceptTask(taskId)
    ElMessage.success('已接受任务')
    await fetchTasks()
  } catch (e) {
    console.error('Accept task failed:', e)
    ElMessage.error('接受任务失败')
  }
}

async function submitResult(taskId: string) {
  const { value: result } = await ElMessageBox.prompt('请输入任务结果', '提交结果', {
    confirmButtonText: '提交',
    cancelButtonText: '取消',
    inputType: 'textarea'
  }).catch(() => ({ value: null }))
  
  if (result) {
    try {
      await api.submitTask(taskId, result)
      ElMessage.success('结果已提交')
      await fetchTasks()
    } catch (e) {
      console.error('Submit result failed:', e)
      ElMessage.error('提交结果失败')
    }
  }
}

function showTaskDetail(task: TaskInfo) {
  selectedTask.value = task
  detailDialogVisible.value = true
}

function getStatusType(status: string): string {
  switch (status) {
    case 'pending': return 'warning'
    case 'accepted': return 'primary'
    case 'completed': return 'success'
    case 'failed': return 'danger'
    default: return 'info'
  }
}

function getStatusLabel(status: string): string {
  switch (status) {
    case 'pending': return '待接受'
    case 'accepted': return '进行中'
    case 'completed': return '已完成'
    case 'failed': return '失败'
    default: return status
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
  <div class="tasks-view">
    <!-- 操作栏 -->
    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="showCreateDialog">创建任务</el-button>
      <el-button :icon="RefreshRight" @click="fetchTasks" :loading="loading">刷新</el-button>
    </div>

    <!-- 标签页 -->
    <el-tabs v-model="activeTab" class="tabs">
      <el-tab-pane label="全部" name="all" />
      <el-tab-pane label="待接受" name="pending" />
      <el-tab-pane label="进行中" name="accepted" />
      <el-tab-pane label="已完成" name="completed" />
    </el-tabs>

    <!-- 任务列表 -->
    <el-table :data="filteredTasks" v-loading="loading" stripe>
      <el-table-column label="任务 ID" width="200">
        <template #default="{ row }">
          <span class="task-id">{{ shortenId(row.task_id) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="类型" prop="type" width="100" />
      <el-table-column label="描述" prop="description" min-width="200" show-overflow-tooltip />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ getStatusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="进度" width="100">
        <template #default="{ row }">
          <el-progress :percentage="row.progress || 0" :stroke-width="6" />
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button-group size="small">
            <el-button :icon="View" @click="showTaskDetail(row)">详情</el-button>
            <el-button v-if="row.status === 'pending'" type="primary" :icon="Check" @click="acceptTask(row.task_id)">
              接受
            </el-button>
            <el-button v-if="row.status === 'accepted'" type="success" @click="submitResult(row.task_id)">
              提交
            </el-button>
          </el-button-group>
        </template>
      </el-table-column>
    </el-table>

    <!-- 创建任务对话框 -->
    <el-dialog v-model="createDialogVisible" title="创建任务" width="500px">
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="类型">
          <el-select v-model="createForm.type" placeholder="选择任务类型">
            <el-option label="计算任务" value="compute" />
            <el-option label="数据任务" value="data" />
            <el-option label="验证任务" value="verify" />
            <el-option label="中继任务" value="relay" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createForm.description" type="textarea" :rows="3" placeholder="任务描述" />
        </el-form-item>
        <el-form-item label="目标节点">
          <el-input v-model="createForm.target" placeholder="目标节点 ID（可选）" />
        </el-form-item>
        <el-form-item label="Payload">
          <el-input v-model="createForm.payload" type="textarea" :rows="4" placeholder='{"key": "value"}' />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="createTask" :loading="createLoading">创建</el-button>
      </template>
    </el-dialog>

    <!-- 任务详情对话框 -->
    <el-dialog v-model="detailDialogVisible" title="任务详情" width="600px">
      <template v-if="selectedTask">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="任务 ID" :span="2">{{ selectedTask.task_id }}</el-descriptions-item>
          <el-descriptions-item label="类型">{{ selectedTask.type }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusType(selectedTask.status)">{{ getStatusLabel(selectedTask.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="描述" :span="2">{{ selectedTask.description }}</el-descriptions-item>
          <el-descriptions-item label="分配节点" :span="2">{{ selectedTask.assignee_id || '-' }}</el-descriptions-item>
          <el-descriptions-item label="奖励">{{ selectedTask.reward || 0 }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatTime(selectedTask.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="创建者" :span="2">{{ selectedTask.creator_id || '-' }}</el-descriptions-item>
          <el-descriptions-item label="结果" :span="2">
            <pre v-if="selectedTask.result">{{ selectedTask.result }}</pre>
            <span v-else>-</span>
          </el-descriptions-item>
        </el-descriptions>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.tasks-view {
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

.task-id {
  font-family: monospace;
  color: var(--el-color-primary);
}

pre {
  background: var(--el-fill-color-light);
  padding: 8px;
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
</style>
