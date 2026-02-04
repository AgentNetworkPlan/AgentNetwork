<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import api, { type APIEndpoint } from '@/api'
import { Search, CopyDocument } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const endpoints = ref<APIEndpoint[]>([])
const loading = ref(true)
const searchQuery = ref('')
const selectedCategory = ref('')

onMounted(async () => {
  await fetchEndpoints()
})

async function fetchEndpoints() {
  loading.value = true
  try {
    const data = await api.getEndpoints()
    endpoints.value = data.endpoints
  } catch (e) {
    console.error('Failed to fetch endpoints:', e)
  }
  loading.value = false
}

const categories = computed(() => {
  const cats = new Set(endpoints.value.map(e => e.category))
  return Array.from(cats)
})

const filteredEndpoints = computed(() => {
  let result = endpoints.value

  if (selectedCategory.value) {
    result = result.filter(e => e.category === selectedCategory.value)
  }

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(e => 
      e.path.toLowerCase().includes(query) ||
      e.description.toLowerCase().includes(query)
    )
  }

  return result
})

function getMethodType(method: string): string {
  switch (method) {
    case 'GET': return 'success'
    case 'POST': return 'primary'
    case 'PUT': return 'warning'
    case 'DELETE': return 'danger'
    default: return 'info'
  }
}

function copyEndpoint(endpoint: APIEndpoint) {
  const text = `curl -X ${endpoint.method} http://127.0.0.1:18345${endpoint.path}`
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制到剪贴板')
}
</script>

<template>
  <div class="endpoints-page" v-loading="loading">
    <!-- Header -->
    <div class="page-header">
      <h2>HTTP API 浏览器</h2>
      <p>探索和测试 DAAN 节点提供的 HTTP API 接口</p>
    </div>

    <!-- Filters -->
    <div class="filters">
      <el-input
        v-model="searchQuery"
        placeholder="搜索 API..."
        :prefix-icon="Search"
        clearable
        style="width: 300px;"
      />

      <el-select
        v-model="selectedCategory"
        placeholder="选择分类"
        clearable
        style="width: 150px;"
      >
        <el-option
          v-for="cat in categories"
          :key="cat"
          :label="cat"
          :value="cat"
        />
      </el-select>

      <span class="endpoint-count">
        共 {{ filteredEndpoints.length }} 个接口
      </span>
    </div>

    <!-- Endpoints List -->
    <div class="endpoints-list">
      <el-card
        v-for="endpoint in filteredEndpoints"
        :key="`${endpoint.method}-${endpoint.path}`"
        class="endpoint-card"
      >
        <div class="endpoint-header">
          <el-tag :type="getMethodType(endpoint.method)" effect="dark">
            {{ endpoint.method }}
          </el-tag>
          <code class="endpoint-path">{{ endpoint.path }}</code>
          <el-button
            :icon="CopyDocument"
            size="small"
            text
            @click="copyEndpoint(endpoint)"
          />
        </div>

        <div class="endpoint-description">
          {{ endpoint.description }}
        </div>

        <div class="endpoint-footer">
          <el-tag size="small" type="info">{{ endpoint.category }}</el-tag>
        </div>
      </el-card>
    </div>

    <!-- Empty state -->
    <el-empty
      v-if="!loading && filteredEndpoints.length === 0"
      description="没有找到匹配的 API"
    />

    <!-- API Usage Guide -->
    <el-card class="usage-card">
      <template #header>使用指南</template>
      <div class="usage-content">
        <h4>基本 URL</h4>
        <code>http://127.0.0.1:18345</code>

        <h4>示例请求</h4>
        <pre><code>curl http://127.0.0.1:18345/v1/health

# 带签名的请求
curl -H "X-Signature: &lt;signature&gt;" \
     -H "X-Public-Key: &lt;public_key&gt;" \
     http://127.0.0.1:18345/v1/peers</code></pre>

        <h4>Python 客户端</h4>
        <pre><code>import requests

base_url = "http://127.0.0.1:18345"

# 健康检查
response = requests.get(f"{base_url}/v1/health")
print(response.json())</code></pre>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.endpoints-page {
  max-width: 1000px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h2 {
  margin: 0 0 8px;
}

.page-header p {
  margin: 0;
  color: #999;
}

.filters {
  display: flex;
  gap: 16px;
  align-items: center;
  margin-bottom: 24px;
}

.endpoint-count {
  margin-left: auto;
  color: #999;
  font-size: 14px;
}

.endpoints-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 24px;
}

.endpoint-card {
  background: var(--el-bg-color-overlay);
}

.endpoint-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.endpoint-path {
  font-family: 'Consolas', monospace;
  font-size: 15px;
  flex: 1;
}

.endpoint-description {
  margin: 12px 0;
  color: #999;
  font-size: 14px;
}

.endpoint-footer {
  display: flex;
  justify-content: flex-end;
}

.usage-card {
  background: var(--el-bg-color-overlay);
}

.usage-content h4 {
  margin: 16px 0 8px;
  color: var(--el-color-primary);
}

.usage-content h4:first-child {
  margin-top: 0;
}

.usage-content code {
  font-family: 'Consolas', monospace;
  background: rgba(0, 0, 0, 0.3);
  padding: 2px 8px;
  border-radius: 4px;
}

.usage-content pre {
  background: rgba(0, 0, 0, 0.3);
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
}

.usage-content pre code {
  background: none;
  padding: 0;
}
</style>
