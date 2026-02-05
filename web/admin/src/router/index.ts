import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      redirect: '/dashboard'
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { title: '登录', public: true }
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/views/DashboardView.vue'),
      meta: { title: '仪表盘' }
    },
    {
      path: '/topology',
      name: 'topology',
      component: () => import('@/views/TopologyView.vue'),
      meta: { title: '网络拓扑' }
    },
    {
      path: '/neighbors',
      name: 'neighbors',
      component: () => import('@/views/NeighborsView.vue'),
      meta: { title: '邻居管理' }
    },
    {
      path: '/mailbox',
      name: 'mailbox',
      component: () => import('@/views/MailboxView.vue'),
      meta: { title: '邮箱' }
    },
    {
      path: '/bulletin',
      name: 'bulletin',
      component: () => import('@/views/BulletinView.vue'),
      meta: { title: '留言板' }
    },
    {
      path: '/endpoints',
      name: 'endpoints',
      component: () => import('@/views/EndpointsView.vue'),
      meta: { title: 'API 浏览器' }
    },
    {
      path: '/logs',
      name: 'logs',
      component: () => import('@/views/LogsView.vue'),
      meta: { title: '日志查看' }
    },
    {
      path: '/tasks',
      name: 'tasks',
      component: () => import('@/views/TasksView.vue'),
      meta: { title: '任务管理' }
    },
    {
      path: '/voting',
      name: 'voting',
      component: () => import('@/views/VotingView.vue'),
      meta: { title: '投票系统' }
    },
    {
      path: '/supernodes',
      name: 'supernodes',
      component: () => import('@/views/SupernodesView.vue'),
      meta: { title: '超级节点' }
    },
    {
      path: '/audit',
      name: 'audit',
      component: () => import('@/views/AuditView.vue'),
      meta: { title: '审计管理' }
    },
    {
      path: '/disputes',
      name: 'disputes',
      component: () => import('@/views/DisputesView.vue'),
      meta: { title: '争议处理' }
    },
    {
      path: '/about',
      name: 'about',
      component: () => import('@/views/AboutView.vue'),
      meta: { title: '关于' }
    }
  ]
})

// Navigation guard - simple auth check using cookie (set by server on login)
router.beforeEach((to, _from, next) => {
  console.log('[Router] Navigation guard triggered')
  console.log('[Router] From:', _from.path, 'To:', to.path)
  console.log('[Router] Target query:', to.query)
  console.log('[Router] Target meta:', to.meta)
  
  const isAuthenticated = localStorage.getItem('daan_authenticated') === 'true'
  console.log('[Router] Authentication status:', isAuthenticated)
  
  // Public routes (login page) - always accessible
  if (to.meta.public) {
    console.log('[Router] Public route - checking if already authenticated')
    // If already authenticated and going to login, redirect to dashboard
    if (isAuthenticated && to.name === 'login' && !to.query.token) {
      console.log('[Router] Already authenticated, redirecting to dashboard')
      next('/dashboard')
    } else {
      console.log('[Router] Allowing access to public route')
      next()
    }
    return
  }
  
  // Protected routes - require authentication
  if (!isAuthenticated) {
    console.log('[Router] Not authenticated - redirecting to login')
    next('/login')
  } else {
    console.log('[Router] Authenticated - allowing access to protected route')
    next()
  }
})

export default router
