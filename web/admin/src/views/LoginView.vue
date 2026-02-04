<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Lock } from '@element-plus/icons-vue'

const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()

const token = ref('')
const loading = ref(false)
const errorMsg = ref('')
const autoLoginAttempted = ref(false)
const urlToken = ref('')

// Extract token from URL for display
function extractUrlToken() {
  const urlParams = new URLSearchParams(window.location.search)
  const extractedToken = urlParams.get('token')
  if (extractedToken) {
    urlToken.value = extractedToken
    console.log('[LoginView] Found token in URL:', extractedToken.substring(0, 8) + '...')
  } else {
    // Try to extract from initial URL that might be in referrer or other sources
    const fullUrl = document.referrer || window.location.href
    console.log('[LoginView] Checking referrer/full URL:', fullUrl)
    if (fullUrl.includes('token=')) {
      const match = fullUrl.match(/token=([^&]+)/)
      if (match) {
        urlToken.value = match[1]
        console.log('[LoginView] Extracted token from referrer:', match[1].substring(0, 8) + '...')
      }
    }
  }
}

function useUrlToken() {
  if (urlToken.value) {
    token.value = urlToken.value
    handleLogin()
  }
}

// Auto-login with URL token
async function tryAutoLogin(urlToken: string) {
  console.log('[LoginView] tryAutoLogin called with:', urlToken ? urlToken.substring(0, 8) + '...' : 'null')
  console.log('[LoginView] autoLoginAttempted:', autoLoginAttempted.value, 'loading:', loading.value)
  
  if (urlToken && !autoLoginAttempted.value && !loading.value) {
    console.log('[LoginView] Starting auto-login with token:', urlToken.substring(0, 8) + '...')
    autoLoginAttempted.value = true
    token.value = urlToken
    await handleLogin()
  } else {
    console.log('[LoginView] Auto-login skipped - urlToken exists:', !!urlToken, 'already attempted:', autoLoginAttempted.value, 'loading:', loading.value)
  }
}

// Watch for route query changes (in case route is not ready on mount)
watch(() => route.query.token, (newToken) => {
  console.log('[LoginView] Route token watcher triggered:', newToken ? (newToken as string).substring(0, 8) + '...' : 'null')
  if (newToken) {
    tryAutoLogin(newToken as string)
  }
}, { immediate: true })

// Also try on mount
onMounted(() => {
  console.log('[LoginView] Component mounted')
  console.log('[LoginView] Current URL:', window.location.href)
  console.log('[LoginView] Document referrer:', document.referrer)
  console.log('[LoginView] Route query:', route.query)
  
  // Extract token from URL
  extractUrlToken()
  
  const routeToken = route.query.token as string
  console.log('[LoginView] Route token:', routeToken ? routeToken.substring(0, 8) + '...' : 'null')
  console.log('[LoginView] Extracted URL token:', urlToken.value ? urlToken.value.substring(0, 8) + '...' : 'null')
  
  const tokenToUse = routeToken || urlToken.value
  if (tokenToUse) {
    tryAutoLogin(tokenToUse)
  } else {
    console.log('[LoginView] No token found, manual login required')
  }
})

async function handleLogin() {
  if (!token.value.trim()) {
    errorMsg.value = '请输入访问令牌'
    return
  }

  loading.value = true
  errorMsg.value = ''

  try {
    console.log('[LoginView] Calling login API...')
    const success = await authStore.login(token.value)
    console.log('[LoginView] Login result:', success, 'error:', authStore.error)
    
    if (success) {
      console.log('[LoginView] Login successful, redirecting to dashboard...')
      // Token stored in cookie by server, redirect to dashboard
      router.replace('/dashboard')
    } else {
      errorMsg.value = authStore.error || '登录失败，令牌无效'
      console.log('[LoginView] Login failed:', errorMsg.value)
    }
  } catch (e: any) {
    console.error('[LoginView] Login error:', e)
    errorMsg.value = e.message || '网络错误'
  } finally {
    loading.value = false
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
        
        <!-- Show token from console output for quick access -->
        <div v-if="urlToken" style="margin-top: 16px; padding: 12px; background: #2d3748; border-radius: 6px;">
          <p style="color: #4fc3f7; margin-bottom: 8px; font-size: 12px;">当前URL中的令牌：</p>
          <code style="color: #67c23a; font-size: 11px; word-break: break-all;">{{ urlToken }}</code>
          <el-button 
            type="success" 
            size="small" 
            @click="useUrlToken"
            style="margin-top: 8px; width: 100%;"
          >
            使用此令牌登录
          </el-button>
        </div>
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
