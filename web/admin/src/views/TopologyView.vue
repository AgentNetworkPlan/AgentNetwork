<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { GraphChart } from 'echarts/charts'
import { TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import api, { createWebSocket, type Topology, type TopologyNode, type TopologyLink } from '@/api'
import { Refresh } from '@element-plus/icons-vue'

use([CanvasRenderer, GraphChart, TooltipComponent, LegendComponent])

const topology = ref<Topology | null>(null)
const loading = ref(true)
const autoRefresh = ref(true)

let topologyWs: WebSocket | null = null

const chartOption = ref<any>({})

onMounted(async () => {
  await fetchTopology()
  if (autoRefresh.value) {
    setupWebSocket()
  }
})

onUnmounted(() => {
  if (topologyWs) {
    topologyWs.close()
  }
})

watch(autoRefresh, (value) => {
  if (value) {
    setupWebSocket()
  } else if (topologyWs) {
    topologyWs.close()
    topologyWs = null
  }
})

async function fetchTopology() {
  loading.value = true
  try {
    topology.value = await api.getTopology()
    updateChart()
  } catch (e) {
    console.error('Failed to fetch topology:', e)
  }
  loading.value = false
}

function setupWebSocket() {
  topologyWs = createWebSocket('topology')
  topologyWs.onmessage = (event) => {
    try {
      topology.value = JSON.parse(event.data)
      updateChart()
    } catch (e) {
      console.error('Failed to parse topology:', e)
    }
  }
  topologyWs.onclose = () => {
    if (autoRefresh.value) {
      setTimeout(setupWebSocket, 5000)
    }
  }
}

function updateChart() {
  if (!topology.value) return

  const nodes = topology.value.nodes.map((node: TopologyNode) => ({
    id: node.id,
    name: node.name || node.id.slice(0, 8),
    symbolSize: getNodeSize(node.type),
    itemStyle: {
      color: getNodeColor(node.type, node.status)
    },
    category: node.type,
    value: node.reputation
  }))

  const links = topology.value.links.map((link: TopologyLink) => ({
    source: link.source,
    target: link.target,
    lineStyle: {
      color: link.status === 'active' ? '#4fc3f7' : '#666',
      width: link.status === 'active' ? 2 : 1,
      opacity: link.status === 'active' ? 0.8 : 0.3
    }
  }))

  chartOption.value = {
    tooltip: {
      trigger: 'item',
      formatter: (params: any) => {
        if (params.dataType === 'node') {
          return `<strong>${params.name}</strong><br/>
                  类型: ${params.data.category}<br/>
                  声誉: ${(params.value * 100).toFixed(1)}%`
        }
        return ''
      }
    },
    legend: {
      data: ['self', 'peer', 'supernode', 'genesis'],
      textStyle: { color: '#999' },
      bottom: 10
    },
    series: [{
      type: 'graph',
      layout: 'force',
      animation: true,
      roam: true,
      draggable: true,
      data: nodes,
      links: links,
      categories: [
        { name: 'self' },
        { name: 'peer' },
        { name: 'supernode' },
        { name: 'genesis' }
      ],
      force: {
        repulsion: 300,
        gravity: 0.1,
        edgeLength: [100, 200],
        layoutAnimation: true
      },
      label: {
        show: true,
        position: 'bottom',
        color: '#ccc',
        fontSize: 10
      },
      lineStyle: {
        curveness: 0.1
      },
      emphasis: {
        focus: 'adjacency',
        lineStyle: {
          width: 4
        }
      }
    }]
  }
}

function getNodeSize(type: string): number {
  switch (type) {
    case 'self': return 50
    case 'genesis': return 45
    case 'supernode': return 40
    default: return 30
  }
}

function getNodeColor(type: string, status: string): string {
  if (status === 'offline') return '#666'
  switch (type) {
    case 'self': return '#4fc3f7'
    case 'genesis': return '#f06292'
    case 'supernode': return '#ffb74d'
    default: return '#81c784'
  }
}

function shortenId(id: string): string {
  if (id.length <= 16) return id
  return id.slice(0, 8) + '...' + id.slice(-6)
}
</script>

<template>
  <div class="topology-page">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-switch
          v-model="autoRefresh"
          active-text="实时更新"
          inactive-text="手动刷新"
        />
        <span v-if="topology" class="update-time">
          更新于: {{ new Date(topology.updated_at).toLocaleTimeString() }}
        </span>
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="fetchTopology" :loading="loading">
          刷新
        </el-button>
      </div>
    </div>

    <!-- Stats -->
    <div class="stats-bar">
      <div class="stat-item">
        <span class="stat-label">节点数</span>
        <span class="stat-value">{{ topology?.nodes.length || 0 }}</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">连接数</span>
        <span class="stat-value">{{ topology?.links.length || 0 }}</span>
      </div>
      <div class="stat-item legend">
        <span class="legend-item self">● 本节点</span>
        <span class="legend-item genesis">● 创世节点</span>
        <span class="legend-item supernode">● 超级节点</span>
        <span class="legend-item peer">● 普通节点</span>
      </div>
    </div>

    <!-- Chart -->
    <div class="chart-container" v-loading="loading">
      <v-chart 
        class="chart" 
        :option="chartOption" 
        autoresize
      />
    </div>

    <!-- Node List -->
    <el-card class="node-list-card">
      <template #header>节点列表</template>
      <el-table 
        :data="topology?.nodes || []" 
        stripe 
        max-height="300"
        style="width: 100%"
      >
        <el-table-column label="节点 ID" min-width="200">
          <template #default="{ row }">
            <code>{{ shortenId(row.id) }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="100">
          <template #default="{ row }">
            <el-tag 
              :type="row.type === 'self' ? 'primary' : 
                     row.type === 'genesis' ? 'danger' : 
                     row.type === 'supernode' ? 'warning' : 'success'"
              size="small"
            >
              {{ row.type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reputation" label="声誉" width="150">
          <template #default="{ row }">
            <el-progress 
              :percentage="row.reputation * 100" 
              :stroke-width="8"
              :show-text="false"
            />
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag 
              :type="row.status === 'online' ? 'success' : 'danger'"
              size="small"
            >
              {{ row.status === 'online' ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.topology-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--el-bg-color-overlay);
  padding: 12px 16px;
  border-radius: 8px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.update-time {
  color: #999;
  font-size: 13px;
}

.stats-bar {
  display: flex;
  gap: 24px;
  align-items: center;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.stat-label {
  color: #999;
  font-size: 13px;
}

.stat-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-color-primary);
}

.legend {
  margin-left: auto;
  gap: 16px;
}

.legend-item {
  font-size: 12px;
  color: #999;
}

.legend-item.self { color: #4fc3f7; }
.legend-item.genesis { color: #f06292; }
.legend-item.supernode { color: #ffb74d; }
.legend-item.peer { color: #81c784; }

.chart-container {
  background: var(--el-bg-color-overlay);
  border-radius: 8px;
  height: 500px;
}

.chart {
  width: 100%;
  height: 100%;
}

.node-list-card {
  background: var(--el-bg-color-overlay);
}

code {
  font-family: 'Consolas', monospace;
  background: rgba(0, 0, 0, 0.2);
  padding: 2px 6px;
  border-radius: 4px;
}
</style>
