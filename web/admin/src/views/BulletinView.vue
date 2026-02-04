<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChatDotRound, RefreshRight, Plus, Search, Bell, Delete } from '@element-plus/icons-vue'
import api, { type BulletinMessage } from '@/api'

const messages = ref<BulletinMessage[]>([])
const subscriptions = ref<string[]>([])
const loading = ref(true)
const searchKeyword = ref('')
const selectedTopic = ref('')

// 发布对话框
const publishDialogVisible = ref(false)
const publishForm = ref({
  topic: '',
  content: '',
  ttl: 3600
})
const publishLoading = ref(false)

// 订阅对话框
const subscribeDialogVisible = ref(false)
const subscribeTopic = ref('')
const subscribeLoading = ref(false)

// 常用话题
const popularTopics = ['general', 'tasks', 'announcements', 'help', 'trade']

onMounted(async () => {
  await Promise.all([
    fetchMessages(),
    fetchSubscriptions()
  ])
})

async function fetchMessages(topic?: string) {
  loading.value = true
  try {
    let data
    if (topic) {
      data = await api.getBulletinByTopic(topic, 50)
    } else if (searchKeyword.value) {
      data = await api.searchBulletin(searchKeyword.value, 50)
    } else {
      // 默认获取 general 话题
      data = await api.getBulletinByTopic('general', 50)
    }
    messages.value = data.messages || []
  } catch (e) {
    console.error('Failed to fetch messages:', e)
    ElMessage.error('获取留言失败')
  }
  loading.value = false
}

async function fetchSubscriptions() {
  try {
    const data = await api.getBulletinSubscriptions()
    subscriptions.value = data.topics || []
  } catch (e) {
    console.error('Failed to fetch subscriptions:', e)
  }
}

async function selectTopic(topic: string) {
  selectedTopic.value = topic
  searchKeyword.value = ''
  await fetchMessages(topic)
}

async function searchMessages() {
  if (!searchKeyword.value.trim()) {
    ElMessage.warning('请输入搜索关键词')
    return
  }
  selectedTopic.value = ''
  await fetchMessages()
}

function showPublishDialog() {
  publishForm.value = { 
    topic: selectedTopic.value || 'general', 
    content: '', 
    ttl: 3600 
  }
  publishDialogVisible.value = true
}

async function publishMessage() {
  if (!publishForm.value.topic || !publishForm.value.content) {
    ElMessage.warning('请填写话题和内容')
    return
  }
  
  publishLoading.value = true
  try {
    await api.publishBulletin(
      publishForm.value.topic, 
      publishForm.value.content, 
      publishForm.value.ttl
    )
    ElMessage.success('留言已发布')
    publishDialogVisible.value = false
    await fetchMessages(publishForm.value.topic)
  } catch (e) {
    console.error('Publish failed:', e)
    ElMessage.error('发布失败')
  }
  publishLoading.value = false
}

function showSubscribeDialog() {
  subscribeTopic.value = ''
  subscribeDialogVisible.value = true
}

async function subscribeTopic_fn() {
  if (!subscribeTopic.value.trim()) {
    ElMessage.warning('请输入话题名称')
    return
  }
  
  subscribeLoading.value = true
  try {
    await api.subscribeBulletin(subscribeTopic.value)
    ElMessage.success('订阅成功')
    subscribeDialogVisible.value = false
    await fetchSubscriptions()
  } catch (e) {
    console.error('Subscribe failed:', e)
    ElMessage.error('订阅失败')
  }
  subscribeLoading.value = false
}

async function unsubscribeTopic_fn(topic: string) {
  try {
    await ElMessageBox.confirm(
      `确定要取消订阅话题 "${topic}" 吗？`,
      '确认取消订阅',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    
    await api.unsubscribeBulletin(topic)
    ElMessage.success('已取消订阅')
    await fetchSubscriptions()
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Unsubscribe failed:', e)
      ElMessage.error('取消订阅失败')
    }
  }
}

async function revokeMessage(messageId: string) {
  try {
    await ElMessageBox.confirm(
      '确定要撤回这条留言吗？',
      '确认撤回',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    
    await api.revokeBulletin(messageId)
    ElMessage.success('留言已撤回')
    await fetchMessages(selectedTopic.value || undefined)
  } catch (e: any) {
    if (e !== 'cancel') {
      console.error('Revoke failed:', e)
      ElMessage.error('撤回失败')
    }
  }
}

function shortenId(id: string): string {
  if (!id || id.length <= 16) return id || '-'
  return id.slice(0, 8) + '...' + id.slice(-6)
}

function getStatusType(status: string): string {
  switch (status) {
    case 'active': return 'success'
    case 'pinned': return 'warning'
    case 'expired': return 'info'
    case 'revoked': return 'danger'
    default: return 'info'
  }
}

function getStatusLabel(status: string): string {
  switch (status) {
    case 'active': return '有效'
    case 'pinned': return '置顶'
    case 'expired': return '已过期'
    case 'revoked': return '已撤回'
    default: return status
  }
}

function formatTTL(seconds: number): string {
  if (seconds < 60) return `${seconds}秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}分钟`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}小时`
  return `${Math.floor(seconds / 86400)}天`
}

</script>

<template>
  <div class="bulletin-page">
    <div class="page-header">
      <h2><el-icon><ChatDotRound /></el-icon> 留言板</h2>
      <div class="header-actions">
        <el-input
          v-model="searchKeyword"
          placeholder="搜索留言..."
          style="width: 200px"
          @keyup.enter="searchMessages"
        >
          <template #append>
            <el-button :icon="Search" @click="searchMessages" />
          </template>
        </el-input>
        <el-button :icon="RefreshRight" @click="fetchMessages(selectedTopic || undefined)" :loading="loading">刷新</el-button>
        <el-button :icon="Bell" @click="showSubscribeDialog">订阅话题</el-button>
        <el-button type="primary" :icon="Plus" @click="showPublishDialog">发布留言</el-button>
      </div>
    </div>

    <div class="bulletin-layout">
      <!-- 侧边栏 - 话题列表 -->
      <div class="sidebar">
        <el-card class="topics-card">
          <template #header>
            <span>话题</span>
          </template>
          
          <div class="topic-section">
            <div class="section-title">热门话题</div>
            <div 
              v-for="topic in popularTopics" 
              :key="topic"
              class="topic-item"
              :class="{ active: selectedTopic === topic }"
              @click="selectTopic(topic)"
            >
              <span class="topic-name">#{{ topic }}</span>
            </div>
          </div>

          <el-divider />

          <div class="topic-section">
            <div class="section-title">我的订阅 ({{ subscriptions.length }})</div>
            <div 
              v-for="topic in subscriptions" 
              :key="topic"
              class="topic-item subscribed"
              :class="{ active: selectedTopic === topic }"
            >
              <span class="topic-name" @click="selectTopic(topic)">#{{ topic }}</span>
              <el-button 
                size="small" 
                type="danger"
                :icon="Delete"
                circle
                @click.stop="unsubscribeTopic_fn(topic)"
              />
            </div>
            <div v-if="subscriptions.length === 0" class="empty-text">
              暂无订阅
            </div>
          </div>
        </el-card>
      </div>

      <!-- 主内容区 - 留言列表 -->
      <div class="main-content">
        <el-card class="messages-card" v-loading="loading">
          <template #header>
            <div class="messages-header">
              <span v-if="selectedTopic">#{{ selectedTopic }}</span>
              <span v-else-if="searchKeyword">搜索: "{{ searchKeyword }}"</span>
              <span v-else>全部留言</span>
              <el-tag>{{ messages.length }} 条</el-tag>
            </div>
          </template>

          <div class="messages-list">
            <div 
              v-for="msg in messages" 
              :key="msg.message_id"
              class="message-item"
              :class="{ revoked: msg.status === 'revoked' }"
            >
              <div class="message-header">
                <div class="author-info">
                  <code class="author" :title="msg.author">{{ shortenId(msg.author) }}</code>
                  <el-tag v-if="msg.reputation >= 80" type="success" size="small">高信誉</el-tag>
                </div>
                <div class="message-meta">
                  <el-tag :type="getStatusType(msg.status)" size="small">
                    {{ getStatusLabel(msg.status) }}
                  </el-tag>
                  <span class="time">{{ msg.timestamp }}</span>
                </div>
              </div>
              
              <div class="message-topic">
                <el-tag size="small" @click="selectTopic(msg.topic)" style="cursor: pointer">
                  #{{ msg.topic }}
                </el-tag>
                <span v-for="tag in msg.tags" :key="tag" class="tag">{{ tag }}</span>
              </div>
              
              <div class="message-content">
                {{ msg.content }}
              </div>

              <div class="message-footer">
                <span class="reputation">声誉: {{ msg.reputation.toFixed(1) }}</span>
                <span v-if="msg.expires_at" class="expires">过期: {{ msg.expires_at }}</span>
                <el-button 
                  v-if="msg.status === 'active'"
                  size="small" 
                  type="danger" 
                  text
                  @click="revokeMessage(msg.message_id)"
                >
                  撤回
                </el-button>
              </div>
            </div>

            <el-empty v-if="messages.length === 0" description="暂无留言" />
          </div>
        </el-card>
      </div>
    </div>

    <!-- 发布留言对话框 -->
    <el-dialog v-model="publishDialogVisible" title="发布留言" width="600px">
      <el-form :model="publishForm" label-width="80px">
        <el-form-item label="话题" required>
          <el-select v-model="publishForm.topic" filterable allow-create placeholder="选择或输入话题">
            <el-option v-for="topic in popularTopics" :key="topic" :label="'#' + topic" :value="topic" />
          </el-select>
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input 
            v-model="publishForm.content" 
            type="textarea"
            :rows="6"
            placeholder="输入留言内容..."
            maxlength="2000"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="有效期">
          <el-select v-model="publishForm.ttl">
            <el-option label="1小时" :value="3600" />
            <el-option label="6小时" :value="21600" />
            <el-option label="1天" :value="86400" />
            <el-option label="3天" :value="259200" />
            <el-option label="7天" :value="604800" />
          </el-select>
          <span class="ttl-hint">{{ formatTTL(publishForm.ttl) }}后过期</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="publishDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="publishMessage" :loading="publishLoading">发布</el-button>
      </template>
    </el-dialog>

    <!-- 订阅话题对话框 -->
    <el-dialog v-model="subscribeDialogVisible" title="订阅话题" width="400px">
      <el-form>
        <el-form-item label="话题名称">
          <el-select v-model="subscribeTopic" filterable allow-create placeholder="选择或输入话题名称">
            <el-option v-for="topic in popularTopics" :key="topic" :label="'#' + topic" :value="topic" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="subscribeDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="subscribeTopic_fn" :loading="subscribeLoading">订阅</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.bulletin-page {
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 10px;
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
  flex-wrap: wrap;
}

.bulletin-layout {
  display: grid;
  grid-template-columns: 250px 1fr;
  gap: 20px;
}

.sidebar .topics-card {
  background: var(--el-bg-color-overlay);
  position: sticky;
  top: 20px;
}

.topic-section {
  margin-bottom: 8px;
}

.section-title {
  font-size: 12px;
  color: #999;
  margin-bottom: 8px;
  text-transform: uppercase;
}

.topic-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.2s;
}

.topic-item:hover {
  background: rgba(255, 255, 255, 0.1);
}

.topic-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.topic-name {
  font-weight: 500;
}

.empty-text {
  color: #666;
  font-size: 13px;
  padding: 8px 12px;
}

.main-content .messages-card {
  background: var(--el-bg-color-overlay);
}

.messages-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.messages-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.message-item {
  background: rgba(0, 0, 0, 0.2);
  border-radius: 8px;
  padding: 16px;
  transition: transform 0.2s;
}

.message-item:hover {
  transform: translateY(-2px);
}

.message-item.revoked {
  opacity: 0.6;
}

.message-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.author-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.author {
  font-family: 'Consolas', monospace;
  background: rgba(0, 0, 0, 0.3);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 13px;
}

.message-meta {
  display: flex;
  align-items: center;
  gap: 10px;
}

.time {
  color: #999;
  font-size: 13px;
}

.message-topic {
  display: flex;
  gap: 6px;
  margin-bottom: 10px;
}

.tag {
  background: rgba(255, 255, 255, 0.1);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  color: #aaa;
}

.message-content {
  color: #fff;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.message-footer {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  font-size: 13px;
  color: #999;
}

.ttl-hint {
  margin-left: 10px;
  color: #999;
  font-size: 13px;
}

@media (max-width: 768px) {
  .bulletin-layout {
    grid-template-columns: 1fr;
  }
  
  .sidebar {
    display: none;
  }
}
</style>
