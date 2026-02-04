<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Lock, User } from '@element-plus/icons-vue'

const authStore = useAuthStore()
const router = useRouter()

const token = ref('')
const loading = ref(false)
const errorMsg = ref('')

// Check URL for token parameter
const urlParams = new URLSearchParams(window.location.search)
const urlToken = urlParams.get('token')
if (urlToken) {
  token.value = urlToken
  handleLogin()
}

async function handleLogin() {
  if (!token.value.trim()) {
    errorMsg.value = '请输入访问令牌'
    return
  }

  loading.value = true
  errorMsg.value = ''

  const success = await authStore.login(token.value)
  
  loading.value = false

  if (success) {
    router.push('/dashboard')
  } else {
    errorMsg.value = authStore.error || '登录失败'
  }
}
</script>

<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <div class="logo">🌐</div>
        <h1>DAAN Admin</h1>
        <p>数字群居社会 - 去中心化Agent网络管理平台</p>
      </div>

      <el-form @submit.prevent="handleLogin" class="login-form">
        <el-form-item>
          <el-input
            v-model="token"
            type="password"
            placeholder="请输入访问令牌"
            :prefix-icon="Lock"
            size="large"
            show-password
          />
        </el-form-item>

        <el-alert
          v-if="errorMsg"
          :title="errorMsg"
          type="error"
          show-icon
          :closable="false"
          style="margin-bottom: 16px;"
        />

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            @click="handleLogin"
            style="width: 100%;"
          >
            登 录
          </el-button>
        </el-form-item>
      </el-form>

      <div class="login-footer">
        <p>获取令牌：</p>
        <code>./daan-node token show</code>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  justify-content: center;
  align-items: center;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
}

.login-card {
  width: 400px;
  background: rgba(22, 33, 62, 0.9);
  border: 1px solid #2d3748;
  border-radius: 12px;
  padding: 40px;
  backdrop-filter: blur(10px);
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.logo {
  font-size: 48px;
  margin-bottom: 16px;
}

.login-header h1 {
  margin: 0;
  color: #4fc3f7;
  font-size: 28px;
}

.login-header p {
  margin: 8px 0 0;
  color: #999;
  font-size: 14px;
}

.login-form {
  margin-top: 24px;
}

.login-footer {
  text-align: center;
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid #2d3748;
  color: #999;
  font-size: 13px;
}

.login-footer code {
  display: block;
  margin-top: 8px;
  background: #0d1424;
  padding: 8px 16px;
  border-radius: 4px;
  font-family: 'Consolas', monospace;
  color: #4fc3f7;
}
</style>
