import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from './stores/auth'
import type { components } from '../shared/api/schema.gen'

declare module 'vue-router' {
  interface RouteMeta { anonymous?: boolean; passwordChange?: boolean; permission?: components['schemas']['Permission']; superadmin?: boolean }
}
const router = createRouter({
  history: createWebHistory('/admin/'),
  routes: [
    { path: '/login', component: () => import('./views/auth/LoginView.vue'), meta: { anonymous: true } },
    { path: '/register', component: () => import('./views/auth/RegisterView.vue'), meta: { anonymous: true } },
    { path: '/password', component: () => import('./views/auth/ChangePasswordView.vue'), meta: { passwordChange: true } },
    { path: '/setup', component: () => import('./views/setup/SetupWizardView.vue'), meta: { superadmin: true } },
    { path: '/', component: () => import('./layouts/AdminLayout.vue'), children: [
      { path: '', redirect: '/docs' },
      { path: 'docs', component: () => import('./views/docs/DocsView.vue'), meta: { permission: 'editDocs' } },
      { path: 'assets', component: () => import('./views/assets/AssetsView.vue'), meta: { permission: 'manageAssets' } },
      { path: 'preview/:id', component: () => import('./views/docs/DocPreviewView.vue'), meta: { permission: 'editDocs' } },
      { path: 'accounts', component: () => import('./views/system/AccountsView.vue'), meta: { superadmin: true } },
      { path: 'audit', component: () => import('./views/system/AuditLogView.vue'), meta: { superadmin: true } },
      { path: 'site', component: () => import('./views/site/SiteSettingsView.vue'), meta: { permission: 'siteSettings' } },
      { path: 'profile', component: () => import('./views/ProfileView.vue') },
    ] },
    { path: '/:pathMatch(.*)*', component: () => import('./views/NotFoundView.vue'), meta: { anonymous: true } },
  ],
})
router.beforeEach(async to => {
  const auth = useAuthStore()
  await auth.initialize()
  if (!auth.setup?.registered && to.path !== '/register') return '/register'
  if (auth.setup?.registered && to.path === '/register') return auth.account ? '/docs' : '/login'
  if (!auth.account) return to.meta.anonymous ? true : '/login'
  if (auth.account.mustChangePassword && to.path !== '/password') return '/password'
  if (to.path === '/login') return '/docs'
  if (!auth.setup?.initialized && auth.account.superadmin && to.path !== '/setup' && to.path !== '/password') return '/setup'
  if (to.meta.superadmin && !auth.account.superadmin) return '/profile'
  if (to.meta.permission && !auth.account.superadmin && !auth.account.permissions.includes(to.meta.permission)) return '/profile'
  return true
})
export default router
