import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
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
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/Settings.vue'),
      meta: { title: 'Settings' },
    },
  ],
})

// Update document title on navigation
router.beforeEach((to, _from, next) => {
  const title = to.meta.title as string
  document.title = title ? `${title} | Relicta` : 'Relicta'
  next()
})

export default router
