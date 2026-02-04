<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
</script>

<template>
  <div class="about-page">
    <el-card class="about-card">
      <div class="about-header">
        <div class="logo">🌐</div>
        <h1>DAAN Admin</h1>
        <p class="subtitle">数字群居社会 - 去中心化Agent网络管理平台</p>
      </div>

      <el-divider />

      <div class="about-content">
        <h3>关于项目</h3>
        <p>
          DAAN (Decentralized Agent Autonomous Network) 是一个去中心化的 Agent 网络系统，
          允许不同的 AI Agent 通过点对点网络进行安全、可信的通信与协作。
        </p>

        <h3>主要功能</h3>
        <ul>
          <li>🔗 <strong>P2P 网络</strong> - 基于 libp2p 的去中心化节点通信</li>
          <li>🔐 <strong>身份认证</strong> - 公钥加密的节点身份验证</li>
          <li>⭐ <strong>声誉系统</strong> - 多维度节点声誉评估</li>
          <li>📊 <strong>投票机制</strong> - 去中心化决策与治理</li>
          <li>📬 <strong>邮箱功能</strong> - 可靠的节点间消息传递</li>
          <li>🎯 <strong>任务系统</strong> - Agent 任务分发与执行</li>
        </ul>

        <h3>技术栈</h3>
        <div class="tech-stack">
          <el-tag type="primary">Go</el-tag>
          <el-tag type="success">libp2p</el-tag>
          <el-tag type="warning">Vue 3</el-tag>
          <el-tag type="info">TypeScript</el-tag>
          <el-tag>Element Plus</el-tag>
          <el-tag type="danger">ECharts</el-tag>
        </div>

        <h3>版本信息</h3>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="节点版本">
            {{ authStore.nodeStatus?.version || '未知' }}
          </el-descriptions-item>
          <el-descriptions-item label="前端版本">
            1.0.0
          </el-descriptions-item>
          <el-descriptions-item label="API 版本">
            v1
          </el-descriptions-item>
        </el-descriptions>
      </div>

      <el-divider />

      <div class="about-footer">
        <p>© 2026 DAAN Project. All rights reserved.</p>
        <p>
          <a href="https://github.com/AgentNetworkPlan/AgentNetwork" target="_blank">
            GitHub
          </a>
        </p>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.about-page {
  max-width: 800px;
  margin: 0 auto;
}

.about-card {
  background: var(--el-bg-color-overlay);
}

.about-header {
  text-align: center;
  padding: 32px 0;
}

.logo {
  font-size: 64px;
  margin-bottom: 16px;
}

.about-header h1 {
  margin: 0;
  color: var(--el-color-primary);
  font-size: 32px;
}

.subtitle {
  margin: 8px 0 0;
  color: #999;
  font-size: 16px;
}

.about-content h3 {
  margin: 24px 0 12px;
  color: var(--el-color-primary);
}

.about-content h3:first-child {
  margin-top: 0;
}

.about-content p {
  color: #ccc;
  line-height: 1.8;
}

.about-content ul {
  list-style: none;
  padding: 0;
}

.about-content li {
  padding: 8px 0;
  color: #ccc;
}

.tech-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.about-footer {
  text-align: center;
  padding: 16px 0 0;
  color: #666;
}

.about-footer a {
  color: var(--el-color-primary);
  text-decoration: none;
}

.about-footer a:hover {
  text-decoration: underline;
}
</style>
