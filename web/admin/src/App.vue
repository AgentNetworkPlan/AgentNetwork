<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  Setting,
  Menu,
  HomeFilled,
  Connection,
  Document,
  List,
  InfoFilled,
  SwitchButton
} from '@element-plus/icons-vue'

const authStore = useAuthStore()
const router = useRouter()
const isCollapse = ref(false)

const menuItems = [
  { index: '/dashboard', title: '仪表盘', icon: HomeFilled },
  { index: '/topology', title: '网络拓扑', icon: Connection },
  { index: '/endpoints', title: 'API 浏览器', icon: Document },
  { index: '/logs', title: '日志查看', icon: List },
  { index: '/about', title: '关于', icon: InfoFilled },
]

onMounted(async () => {
  // Check if authenticated
  if (!authStore.isAuthenticated) {
    router.push('/login')
  }
})

const handleLogout = async () => {
  await authStore.logout()
  router.push('/login')
}

const toggleCollapse = () => {
  isCollapse.value = !isCollapse.value
}
</script>

<template>
  <div class="app-container dark">
    <!-- Login page (no layout) -->
    <router-view v-if="$route.path === '/login'" />

    <!-- Main layout -->
    <el-container v-else class="main-layout">
      <!-- Sidebar -->
      <el-aside :width="isCollapse ? '64px' : '200px'" class="sidebar">
        <div class="logo">
          <span class="logo-icon">🌐</span>
          <span v-if="!isCollapse" class="logo-text">DAAN Admin</span>
        </div>

        <el-menu
          :default-active="$route.path"
          :collapse="isCollapse"
          :collapse-transition="false"
          router
          class="sidebar-menu"
        >
          <el-menu-item
            v-for="item in menuItems"
            :key="item.index"
            :index="item.index"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </el-menu>

        <div class="sidebar-footer">
          <el-button
            :icon="isCollapse ? Menu : Setting"
            circle
            @click="toggleCollapse"
          />
          <el-button
            v-if="!isCollapse"
            :icon="SwitchButton"
            type="danger"
            @click="handleLogout"
          >
            退出
          </el-button>
        </div>
      </el-aside>

      <!-- Main content -->
      <el-container>
        <el-header class="header">
          <div class="header-left">
            <h2>{{ $route.meta.title || 'DAAN 管理平台' }}</h2>
          </div>
          <div class="header-right">
            <el-tag type="success" effect="dark">
              在线
            </el-tag>
            <span class="node-id" v-if="authStore.nodeStatus">
              {{ authStore.nodeStatus.node_id?.slice(0, 12) }}...
            </span>
          </div>
        </el-header>

        <el-main class="main-content">
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<style scoped>
.app-container {
  height: 100vh;
  background: var(--el-bg-color);
}

.main-layout {
  height: 100%;
}

.sidebar {
  background: var(--el-bg-color-overlay);
  border-right: 1px solid var(--el-border-color);
  display: flex;
  flex-direction: column;
  transition: width 0.3s;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-bottom: 1px solid var(--el-border-color);
}

.logo-icon {
  font-size: 24px;
}

.logo-text {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-color-primary);
}

.sidebar-menu {
  flex: 1;
  border: none;
  --el-menu-bg-color: transparent;
}

.sidebar-footer {
  padding: 16px;
  display: flex;
  gap: 8px;
  justify-content: center;
  border-top: 1px solid var(--el-border-color);
}

.header {
  background: var(--el-bg-color-overlay);
  border-bottom: 1px solid var(--el-border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}

.header-left h2 {
  margin: 0;
  font-size: 18px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.node-id {
  font-family: monospace;
  color: var(--el-text-color-secondary);
}

.main-content {
  background: var(--el-bg-color);
  padding: 20px;
  overflow: auto;
}
</style>
