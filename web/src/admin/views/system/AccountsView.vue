<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, requireData, errorMessage } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'
type Account = components['schemas']['Account']
type Permission = components['schemas']['Permission']
const labels: Record<Permission, string> = { editDocs: '编辑文档', publishDocs: '发布文档', manageTranslations: '管理译文', manageLanguages: '管理语言', manageAssets: '管理资源库', manageQA: '问答管理', aiSettings: 'AI 设置', aiMemory: 'AI 记忆', siteSettings: '站点设置', viewStats: '查看统计' }
const permissions = Object.keys(labels) as Permission[]
const accounts = ref<Account[]>([])
const error = ref('')
const busy = ref(false)
const form = reactive({ username: '', password: '', permissions: [...permissions] })
async function refresh() { accounts.value = requireData(await client.GET('/api/admin/accounts')) }
onMounted(async () => { try { await refresh() } catch (cause) { error.value = errorMessage(cause) } })
async function create() {
  error.value = ''; busy.value = true
  try { requireData(await client.POST('/api/admin/accounts', { body: form })); form.username = ''; form.password = ''; await refresh() }
  catch (cause) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}
async function toggle(account: Account) {
  try { requireData(await client.PUT('/api/admin/accounts/{id}', { params: { path: { id: account.id } }, body: { disabled: !account.disabled, permissions: account.permissions } })); await refresh() }
  catch (cause) { error.value = errorMessage(cause) }
}
</script>
<template>
  <PageHeader
    title="账号管理"
    description="新账号使用初始密码登录后，必须先修改密码。"
  />
  <p
    v-if="error"
    role="alert"
    class="mb-5 text-danger"
  >
    {{ error }}
  </p>
  <form
    class="mb-8 grid max-w-2xl gap-4 rounded-card border border-border bg-surface p-6"
    @submit.prevent="create"
  >
    <label class="grid gap-2 text-sm">账号名<input
      v-model="form.username"
      required
      maxlength="64"
      autocomplete="off"
      class="nw-input"
    ></label>
    <label class="grid gap-2 text-sm">初始密码<input
      v-model="form.password"
      required
      minlength="8"
      type="password"
      autocomplete="new-password"
      class="nw-input"
    ></label>
    <fieldset class="grid gap-3 sm:grid-cols-2">
      <legend class="mb-3 text-sm">
        功能权限
      </legend><label
        v-for="permission in permissions"
        :key="permission"
        class="flex gap-2 text-sm"
      ><input
        v-model="form.permissions"
        type="checkbox"
        :value="permission"
      >{{ labels[permission] }}</label>
    </fieldset>
    <button
      :disabled="busy"
      class="nw-button"
    >
      创建账号
    </button>
  </form>
  <div class="grid gap-3">
    <div
      v-for="account in accounts"
      :key="account.id"
      class="flex items-center justify-between rounded-card border border-border p-4"
    >
      <div>
        <p>
          {{ account.username }}<span
            v-if="account.superadmin"
            class="ms-3 text-xs text-primary"
          >超级管理员</span>
        </p><p class="mt-1 text-xs text-muted">
          {{ account.disabled ? '已停用' : account.mustChangePassword ? '等待修改初始密码' : '正常' }}
        </p>
      </div><button
        v-if="!account.superadmin"
        class="text-sm text-primary"
        @click="toggle(account)"
      >
        {{ account.disabled ? '启用' : '停用' }}
      </button>
    </div>
  </div>
</template>
