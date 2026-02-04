<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Message, RefreshRight, Edit, Delete, View, Check } from '@element-plus/icons-vue'
import api, { type MailSummary, type MailMessage } from '@/api'

const activeTab = ref('inbox')
const inboxMessages = ref<MailSummary[]>([])
const outboxMessages = ref<MailSummary[]>([])
const loading = ref(true)
const currentMessage = ref<MailMessage | null>(null)
const viewDialogVisible = ref(false)

// 发送对话框
const sendDialogVisible = ref(false)
const sendForm = ref({
  to: '',
  subject: '',
  content: ''
})
const sendLoading = ref(false)

// 分页
const inboxTotal = ref(0)
const outboxTotal = ref(0)
const pageSize = 20
const inboxPage = ref(1)
const outboxPage = ref(1)

onMounted(async () => {
  await fetchMessages()
})

async function fetchMessages() {
  loading.value = true
  try {
    const [inboxData, outboxData] = await Promise.all([
      api.getMailboxInbox(pageSize, (inboxPage.value - 1) * pageSize),
      api.getMailboxOutbox(pageSize, (outboxPage.value - 1) * pageSize)
    ])
    inboxMessages.value = inboxData.messages || []
    outboxMessages.value = outboxData.messages || []
    inboxTotal.value = inboxData.total || 0
    outboxTotal.value = outboxData.total || 0
  } catch (e) {
    console.error('Failed to fetch messages:', e)
    ElMessage.error('获取邮件列表失败')
  }
  loading.value = false
}

async function viewMessage(id: string) {
  try {
    currentMessage.value = await api.readMail(id)
    viewDialogVisible.value = true
    // 刷新列表以更新已读状态
    await fetchMessages()
  } catch (e) {
    console.error('Failed to read message:', e)
    ElMessage.error('读取邮件失败')
  }
}

async function markAsRead(id: string) {
  try {
    await api.markMailRead(id)
    ElMessage.success('已标记为已读')
    await fetchMessages()
  } catch (e) {
    console.error('Mark read failed:', e)
    ElMessage.error('操作失败')
  }
}

async function deleteMessage(id: string) {
  try {
    await ElMessageBox.confirm(
      '确定要删除这封邮件吗？',
      '确认删除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    
    await api.deleteMail(id)
    ElMessage.success('邮件已删除')
    await fetchMessages()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Delete failed:', e)
      ElMessage.error('删除失败')
    }
  }
}

function showSendDialog() {
  sendForm.value = { to: '', subject: '', content: '' }
  sendDialogVisible.value = true
}

async function sendMail() {
  if (!sendForm.value.to || !sendForm.value.content) {
    ElMessage.warning('请填写收件人和内容')
    return
  }
  
  sendLoading.value = true
  try {
    await api.sendMail(sendForm.value.to, sendForm.value.subject, sendForm.value.content)
    ElMessage.success('邮件已发送')
    sendDialogVisible.value = false
    await fetchMessages()
  } catch (e) {
    console.error('Send failed:', e)
    ElMessage.error('发送失败')
  }
  sendLoading.value = false
}

function shortenId(id: string): string {
  if (!id || id.length <= 16) return id || '-'
  return id.slice(0, 8) + '...' + id.slice(-6)
}

function getStatusType(status: string): string {
  switch (status) {
    case 'read': return 'info'
    case 'delivered': return 'success'
    case 'pending': return 'warning'
    case 'failed': return 'danger'
    default: return 'info'
  }
}

function getStatusLabel(status: string): string {
  switch (status) {
    case 'read': return '已读'
    case 'delivered': return '已送达'
    case 'pending': return '发送中'
    case 'failed': return '失败'
    default: return status
  }
}

const unreadCount = computed(() => 
  inboxMessages.value.filter(m => m.status !== 'read').length
)
</script>

<template>
  <div class="mailbox-page">
    <div class="page-header">
      <h2><el-icon><Message /></el-icon> 邮箱</h2>
      <div class="header-actions">
        <el-button :icon="RefreshRight" @click="fetchMessages" :loading="loading">刷新</el-button>
        <el-button type="primary" :icon="Edit" @click="showSendDialog">写邮件</el-button>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-row">
      <el-card class="stat-card">
        <div class="stat-value">{{ inboxTotal }}</div>
        <div class="stat-label">收件箱</div>
      </el-card>
      <el-card class="stat-card unread">
        <div class="stat-value">{{ unreadCount }}</div>
        <div class="stat-label">未读</div>
      </el-card>
      <el-card class="stat-card">
        <div class="stat-value">{{ outboxTotal }}</div>
        <div class="stat-label">发件箱</div>
      </el-card>
    </div>

    <!-- 邮件列表 -->
    <el-card class="mailbox-content" v-loading="loading">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="收件箱" name="inbox">
          <el-table :data="inboxMessages" stripe style="width: 100%">
            <el-table-column label="发件人" width="180">
              <template #default="{ row }">
                <code class="node-id" :title="row.from">{{ shortenId(row.from) }}</code>
              </template>
            </el-table-column>
            
            <el-table-column label="主题" min-width="200">
              <template #default="{ row }">
                <span :class="{ 'unread-subject': row.status !== 'read' }">
                  {{ row.subject || '(无主题)' }}
                </span>
                <el-tag v-if="row.encrypted" type="warning" size="small" style="margin-left: 8px">
                  加密
                </el-tag>
              </template>
            </el-table-column>
            
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="getStatusType(row.status)" size="small">
                  {{ getStatusLabel(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            
            <el-table-column label="时间" width="160">
              <template #default="{ row }">
                {{ row.timestamp }}
              </template>
            </el-table-column>
            
            <el-table-column label="操作" width="150" fixed="right">
              <template #default="{ row }">
                <el-button size="small" :icon="View" @click="viewMessage(row.id)" title="查看" />
                <el-button 
                  v-if="row.status !== 'read'"
                  size="small" 
                  :icon="Check"
                  @click="markAsRead(row.id)"
                  title="标为已读"
                />
                <el-button size="small" type="danger" :icon="Delete" @click="deleteMessage(row.id)" title="删除" />
              </template>
            </el-table-column>
          </el-table>

          <el-empty v-if="inboxMessages.length === 0" description="收件箱为空" />
          
          <el-pagination
            v-if="inboxTotal > pageSize"
            class="pagination"
            v-model:current-page="inboxPage"
            :page-size="pageSize"
            :total="inboxTotal"
            layout="prev, pager, next"
            @current-change="fetchMessages"
          />
        </el-tab-pane>

        <el-tab-pane label="发件箱" name="outbox">
          <el-table :data="outboxMessages" stripe style="width: 100%">
            <el-table-column label="收件人" width="180">
              <template #default="{ row }">
                <code class="node-id" :title="row.to">{{ shortenId(row.to) }}</code>
              </template>
            </el-table-column>
            
            <el-table-column label="主题" min-width="200">
              <template #default="{ row }">
                {{ row.subject || '(无主题)' }}
                <el-tag v-if="row.encrypted" type="warning" size="small" style="margin-left: 8px">
                  加密
                </el-tag>
              </template>
            </el-table-column>
            
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="getStatusType(row.status)" size="small">
                  {{ getStatusLabel(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            
            <el-table-column label="时间" width="160">
              <template #default="{ row }">
                {{ row.timestamp }}
              </template>
            </el-table-column>
            
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <el-button size="small" :icon="View" @click="viewMessage(row.id)" title="查看" />
                <el-button size="small" type="danger" :icon="Delete" @click="deleteMessage(row.id)" title="删除" />
              </template>
            </el-table-column>
          </el-table>

          <el-empty v-if="outboxMessages.length === 0" description="发件箱为空" />
          
          <el-pagination
            v-if="outboxTotal > pageSize"
            class="pagination"
            v-model:current-page="outboxPage"
            :page-size="pageSize"
            :total="outboxTotal"
            layout="prev, pager, next"
            @current-change="fetchMessages"
          />
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- 查看邮件对话框 -->
    <el-dialog v-model="viewDialogVisible" title="查看邮件" width="600px">
      <div v-if="currentMessage" class="mail-view">
        <div class="mail-header">
          <div class="mail-field">
            <span class="label">发件人:</span>
            <code>{{ currentMessage.from }}</code>
          </div>
          <div class="mail-field">
            <span class="label">收件人:</span>
            <code>{{ currentMessage.to }}</code>
          </div>
          <div class="mail-field">
            <span class="label">主题:</span>
            <span>{{ currentMessage.subject || '(无主题)' }}</span>
          </div>
          <div class="mail-field">
            <span class="label">时间:</span>
            <span>{{ currentMessage.timestamp }}</span>
          </div>
        </div>
        <el-divider />
        <div class="mail-content">
          {{ currentMessage.content }}
        </div>
      </div>
      <template #footer>
        <el-button @click="viewDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 发送邮件对话框 -->
    <el-dialog v-model="sendDialogVisible" title="写邮件" width="600px">
      <el-form :model="sendForm" label-width="80px">
        <el-form-item label="收件人" required>
          <el-input 
            v-model="sendForm.to" 
            placeholder="输入收件人节点ID"
          />
        </el-form-item>
        <el-form-item label="主题">
          <el-input 
            v-model="sendForm.subject" 
            placeholder="邮件主题"
          />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input 
            v-model="sendForm.content" 
            type="textarea"
            :rows="8"
            placeholder="邮件内容"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sendDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="sendMail" :loading="sendLoading">发送</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.mailbox-page {
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

.stat-card.unread .stat-value {
  color: #e6a23c;
}

.mailbox-content {
  background: var(--el-bg-color-overlay);
}

.node-id {
  font-family: 'Consolas', monospace;
  background: rgba(0, 0, 0, 0.2);
  padding: 2px 6px;
  border-radius: 4px;
}

.unread-subject {
  font-weight: bold;
  color: #fff;
}

.pagination {
  margin-top: 16px;
  justify-content: center;
}

.mail-view .mail-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.mail-view .mail-field {
  display: flex;
  gap: 10px;
}

.mail-view .mail-field .label {
  color: #999;
  width: 60px;
  flex-shrink: 0;
}

.mail-view .mail-field code {
  font-family: 'Consolas', monospace;
  background: rgba(0, 0, 0, 0.2);
  padding: 2px 6px;
  border-radius: 4px;
  word-break: break-all;
}

.mail-view .mail-content {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.6;
  color: #fff;
}
</style>
