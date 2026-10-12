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
      { path: '', component: () => import('./views/DashboardView.vue') },
      { path: 'docs', component: () => import('./views/docs/DocsView.vue'), meta: { permission: 'editDocs' } },
      { path: 'assets', component: () => import('./views/assets/AssetsView.vue'), meta: { permission: 'manageAssets' } },
      { path: 'preview/:id', component: () => import('./views/docs/DocPreviewView.vue'), meta: { permission: 'editDocs' } },
      { path: 'docs/:id/history', component: () => import('./views/docs/DocHistoryView.vue'), meta: { permission: 'publishDocs' } },
      { path: 'trash', component: () => import('./views/docs/TrashView.vue'), meta: { permission: 'publishDocs' } },
      { path: 'accounts', component: () => import('./views/system/AccountsView.vue'), meta: { superadmin: true } },
      { path: 'audit', component: () => import('./views/system/AuditLogView.vue'), meta: { superadmin: true } },
      { path: 'backups', component: () => import('./views/system/BackupView.vue'), meta: { superadmin: true } },
      { path: 'jobs', component: () => import('./views/system/JobsView.vue'), meta: { superadmin: true } },
      { path: 'system', component: () => import('./views/system/SystemSettingsView.vue'), meta: { superadmin: true } },
      { path: 'stats', component: () => import('./views/stats/StatsView.vue'), meta: { permission: 'viewStats' } },
      { path: 'models', component: () => import('./views/ai/ChannelsView.vue'), meta: { superadmin: true } },
      { path: 'ai-settings', component: () => import('./views/ai/TiersView.vue'), meta: { permission: 'aiSettings' } },
      { path: 'qa', component: () => import('./views/ai/QaRecordsView.vue'), meta: { permission: 'manageQA' } },
      { path: 'qa-cache', component: () => import('./views/ai/QaCacheView.vue'), meta: { permission: 'manageQA' } },
      { path: 'qa-debug', component: () => import('./views/ai/QaDebugView.vue'), meta: { permission: 'aiSettings' } },
      { path: 'guard', component: () => import('./views/ai/GuardView.vue'), meta: { permission: 'aiSettings' } },
      { path: 'memory', component: () => import('./views/memory/MemoryView.vue'), meta: { permission: 'aiMemory' } },
      { path: 'memory-tasks', component: () => import('./views/memory/MemoryTasksView.vue'), meta: { permission: 'aiMemory' } },
      { path: 'languages', component: () => import('./views/i18n/LanguagesView.vue'), meta: { permission: 'manageLanguages' } },
      { path: 'translations', component: () => import('./views/i18n/TranslationProgressView.vue'), meta: { permission: 'manageTranslations' } },
      { path: 'glossary', component: () => import('./views/i18n/GlossaryView.vue'), meta: { permission: 'manageTranslations' } },
      { path: 'translations/:language/:id', component: () => import('./views/i18n/TranslationEditView.vue'), meta: { permission: 'manageTranslations' } },
      { path: 'site', component: () => import('./views/site/SiteSettingsView.vue'), meta: { permission: 'siteSettings' } },
      { path: 'site/home', component: () => import('./views/site/HomeHeroView.vue'), meta: { permission: 'siteSettings' } },
      { path: 'site/banner', component: () => import('./views/site/BannerView.vue'), meta: { permission: 'siteSettings' } },
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
