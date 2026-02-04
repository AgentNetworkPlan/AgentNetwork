<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import api, { createWebSocket, type LogEntry } from '@/api'
import { Refresh, Download, Delete, Search } from '@element-plus/icons-vue'

const logs = ref<LogEntry[]>([])
const loading = ref(true)
const autoScroll = ref(true)
const searchQuery = ref('')
const selectedLevel = ref('')
const maxLogs = ref(500)

let logsWs: WebSocket | null = null
const logContainer = ref<HTMLElement | null>(null)

const levels = ['DEBUG', 'INFO', 'WARN', 'ERROR']

onMounted(async () => {
  await fetchLogs()
  setupWebSocket()
})

onUnmounted(() => {
  if (logsWs) {
    logsWs.close()
  }
})

async function fetchLogs() {
  loading.value = true
  try {
    const data = await api.getLogs(maxLogs.value)
    logs.value = data.logs
  } catch (e) {
    console.error('Failed to fetch logs:', e)
  }
  loading.value = false
}

function setupWebSocket() {
  logsWs = createWebSocket('logs')
  logsWs.onmessage = (event) => {
    try {
      const newLog = JSON.parse(event.data)
      logs.value.push(newLog)
      
      // Keep only last N logs
      if (logs.value.length > maxLogs.value) {
        logs.value = logs.value.slice(-maxLogs.value)
      }
      
      // Auto scroll
      if (autoScroll.value && logContainer.value) {
        setTimeout(() => {
          logContainer.value?.scrollTo({
            top: logContainer.value.scrollHeight,
            behavior: 'smooth'
          })
        }, 50)
      }
    } catch (e) {
      console.error('Failed to parse log:', e)
    }
  }
  logsWs.onclose = () => {
    setTimeout(setupWebSocket, 5000)
  }
}

const filteredLogs = computed(() => {
  let result = logs.value

  if (selectedLevel.value) {
    result = result.filter(log => log.level === selectedLevel.value)
  }

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(log => 
      log.message.toLowerCase().includes(query) ||
      log.module.toLowerCase().includes(query)
    )
  }

  return result
})

function getLevelClass(level: string): string {
  switch (level.toUpperCase()) {
    case 'DEBUG': return 'log-debug'
    case 'INFO': return 'log-info'
    case 'WARN': return 'log-warn'
    case 'ERROR': return 'log-error'
    default: return ''
  }
}

function getLevelType(level: string): string {
  switch (level.toUpperCase()) {
    case 'DEBUG': return 'info'
    case 'INFO': return 'success'
    case 'WARN': return 'warning'
    case 'ERROR': return 'danger'
    default: return 'info'
  }
}

function formatTime(timestamp: string): string {
  return new Date(timestamp).toLocaleTimeString()
}

function clearLogs() {
  logs.value = []
}

function downloadLogs() {
  const content = filteredLogs.value.map(log => 
    `[${log.timestamp}] [${log.level}] [${log.module}] ${log.message}`
  ).join('\n')
  
  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `daan-logs-${new Date().toISOString().slice(0, 10)}.txt`
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div class="logs-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索日志..."
          :prefix-icon="Search"
          clearable
          style="width: 250px;"
        />

        <el-select
          v-model="selectedLevel"
          placeholder="日志级别"
          clearable
          style="width: 120px;"
        >
          <el-option
            v-for="level in levels"
            :key="level"
            :label="level"
            :value="level"
          />
        </el-select>

        <el-switch
          v-model="autoScroll"
          active-text="自动滚动"
        />
      </div>

      <div class="toolbar-right">
        <span class="log-count">{{ filteredLogs.length }} 条日志</span>
        
        <el-button-group>
          <el-button :icon="Refresh" @click="fetchLogs" :loading="loading">
            刷新
          </el-button>
          <el-button :icon="Download" @click="downloadLogs">
            导出
          </el-button>
          <el-button :icon="Delete" type="danger" @click="clearLogs">
            清空
          </el-button>
        </el-button-group>
      </div>
    </div>

    <!-- Log container -->
    <div class="log-container" ref="logContainer" v-loading="loading">
      <div
        v-for="(log, index) in filteredLogs"
        :key="index"
        class="log-entry"
        :class="getLevelClass(log.level)"
      >
        <span class="log-time">{{ formatTime(log.timestamp) }}</span>
        <el-tag
          :type="getLevelType(log.level)"
          size="small"
          class="log-level"
        >
          {{ log.level }}
        </el-tag>
        <span class="log-module">[{{ log.module }}]</span>
        <span class="log-message">{{ log.message }}</span>
      </div>

      <el-empty
        v-if="!loading && filteredLogs.length === 0"
        description="暂无日志"
      />
    </div>
  </div>
</template>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 160px);
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--el-bg-color-overlay);
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.log-count {
  color: #999;
  font-size: 13px;
}

.log-container {
  flex: 1;
  background: #0d1424;
  border-radius: 8px;
  padding: 16px;
  overflow-y: auto;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 13px;
  line-height: 1.6;
}

.log-entry {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 4px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.log-entry:hover {
  background: rgba(255, 255, 255, 0.03);
}

.log-time {
  color: #666;
  flex-shrink: 0;
  width: 80px;
}

.log-level {
  flex-shrink: 0;
  width: 60px;
  text-align: center;
}

.log-module {
  color: #888;
  flex-shrink: 0;
  min-width: 100px;
}

.log-message {
  flex: 1;
  word-break: break-word;
}

/* Level colors */
.log-debug .log-message { color: #9e9e9e; }
.log-info .log-message { color: #ccc; }
.log-warn .log-message { color: #ff9800; }
.log-error .log-message { color: #f44336; }

/* Scrollbar */
.log-container::-webkit-scrollbar {
  width: 8px;
}

.log-container::-webkit-scrollbar-track {
  background: transparent;
}

.log-container::-webkit-scrollbar-thumb {
  background: #333;
  border-radius: 4px;
}
</style>
