import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import { useAuthStore } from '@/modules/auth/stores/authStore'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/modules/auth/views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/auth/callback',
      name: 'auth-callback',
      component: () => import('@/modules/auth/views/AuthCallbackView.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: AppLayout,
      children: [
        {
          path: '',
          redirect: '/chat/new',
        },
        {
          path: 'chat/new',
          name: 'chat-new',
          component: () => import('@/modules/chat/views/ChatView.vue'),
        },
        {
          path: 'chat/:id',
          name: 'chat',
          component: () => import('@/modules/chat/views/ChatView.vue'),
          props: true,
        },
        {
          path: 'models',
          name: 'models',
          component: () => import('@/modules/model/views/ModelsListView.vue'),
        },
        {
          path: 'models/:id',
          name: 'model-editor',
          component: () => import('@/modules/model/views/ModelEditorView.vue'),
          props: true,
        },
        {
          path: 'simulations',
          name: 'simulations',
          component: () => import('@/modules/simulation/views/SimulationsListView.vue'),
        },
        {
          path: 'simulations/:id',
          name: 'simulation-editor',
          component: () => import('@/modules/simulation/views/SimulationEditorView.vue'),
          props: true,
        },
        {
          path: 'simulations/:id/run',
          name: 'simulation-run',
          component: () => import('@/modules/simulation/views/SimulationRunView.vue'),
          props: true,
        },
        {
          path: 'simulations/:id/analysis',
          name: 'simulation-analysis',
          component: () => import('@/modules/simulation/views/SimulationAnalysisView.vue'),
          props: true,
        },
      ],
    },
  ],
})

// Global login requirement: everything except meta.public routes.
router.beforeEach(async (to) => {
  if (to.meta.public) return true
  const auth = useAuthStore()
  await auth.init()
  if (!auth.isAuthenticated) {
    // A STORED session may just need a silent refresh before giving up.
    // Never attempt silent sign-in on a cold visit (no stored user): the dev
    // mock's /authorize auto-approves prompt=none, which would silently log
    // users in and make the login page unreachable.
    const renewed = auth.user ? await auth.renew() : null
    if (!renewed) return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (!auth.me) auth.loadMe() // fire-and-forget
  return true
})

export default router
