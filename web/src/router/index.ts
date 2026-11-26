import { createRouter, createWebHistory } from 'vue-router'
import { getAuthMode } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/Login.vue'),
      meta: { title: 'Sign in' },
    },
    {
      path: '/',
      name: 'dashboard',
      component: () => import('@/views/Dashboard.vue'),
      meta: { title: 'Dashboard' },
    },
    {
      path: '/releases',
      name: 'releases',
      component: () => import('@/views/ReleasePipeline.vue'),
      meta: { title: 'Release Pipeline' },
    },
    {
      path: '/releases/:id',
      name: 'release-detail',
      component: () => import('@/views/ReleaseDetail.vue'),
      meta: { title: 'Release Details' },
    },
    {
      path: '/governance',
      name: 'governance',
      component: () => import('@/views/GovernanceAnalytics.vue'),
      meta: { title: 'Governance Analytics' },
    },
    {
      path: '/team',
      name: 'team',
      component: () => import('@/views/TeamPerformance.vue'),
      meta: { title: 'Team Performance' },
    },
    {
      path: '/approvals',
      name: 'approvals',
      component: () => import('@/views/ApprovalWorkflow.vue'),
      meta: { title: 'Approval Workflow' },
    },
    {
      path: '/audit',
      name: 'audit',
      component: () => import('@/views/AuditTrail.vue'),
      meta: { title: 'Audit Trail' },
    },
    {
      path: '/analytics',
      name: 'analytics',
      component: () => import('@/views/Analytics.vue'),
      meta: { title: 'Analytics' },
    },
    {
      path: '/repos',
      name: 'repos',
      component: () => import('@/views/MultiRepo.vue'),
      meta: { title: 'Repositories' },
    },
    {
      path: '/observability',
      name: 'observability',
      component: () => import('@/views/Observability.vue'),
      meta: { title: 'Observability' },
    },
    {
      path: '/memory',
      name: 'memory',
      component: () => import('@/views/ReleaseMemory.vue'),
      meta: { title: 'Release Memory' },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/Settings.vue'),
      meta: { title: 'Settings' },
    },
  ],
})

// Update document title on navigation
router.beforeEach(async (to) => {
  const title = to.meta.title as string
  document.title = title ? `${title} | Relicta` : 'Relicta'
  if (to.name === 'login') return true

  try {
    const mode = await getAuthMode()
    if (mode === 'none' || mode === '') return true
  } catch {
    // Keep the local, unauthenticated dashboard usable if its optional mode
    // discovery request is unavailable; protected API requests still enforce auth.
    return true
  }

  const hasApiKey = !!localStorage.getItem('relicta_api_key')
  const hasSession = !!sessionStorage.getItem('relicta_access_token')
  if (hasApiKey || hasSession) return true
  return { name: 'login', query: { redirect: to.fullPath } }
})

window.addEventListener('relicta:unauthorized', () => {
  useAuthStore().logout()
  if (router.currentRoute.value.name !== 'login') {
    void router.replace({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
  }
})

export default router
