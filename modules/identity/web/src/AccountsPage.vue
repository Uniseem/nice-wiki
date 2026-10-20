<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

const permissionLabels: Record<string, string> = {
  edit_documents: '编辑文档', publish_documents: '发布与删除', manage_translations: '管理翻译', manage_languages: '管理语言',
  manage_resources: '管理资源', manage_qa: '问答管理', manage_ai: '模型接入', manage_memory: '长期记忆', manage_site: '站点设置', view_stats: '查看统计',
};
const allPermissions = Object.keys(permissionLabels);

type Account = { accountId: string; accountName: string; isSuperadmin: boolean; permissions: string[]; status: string; revision: number; mustChangePassword: boolean };
const accounts = ref<Account[]>([]);
const error = ref('');
const notice = ref('');
const name = ref('');
const password = ref('');
const chosen = ref<string[]>([...allPermissions]);
const busy = ref(false);

async function load() {
  accounts.value = (await api<{ accounts: Account[] }>('/v1/accounts')).accounts;
}
async function run(action: () => Promise<void>, done = '') {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); if (done) notice.value = done; await load(); } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}
const create = () => run(async () => {
  await api('/v1/accounts', { method: 'POST', body: { accountName: name.value, initialPassword: password.value, permissions: chosen.value } });
  name.value = ''; password.value = '';
}, '子账号已创建，对方首次登录需要修改口令。');
const togglePermission = (account: Account, key: string) => run(async () => {
  const next = account.permissions.includes(key) ? account.permissions.filter((p) => p !== key) : [...account.permissions, key];
  await api(`/v1/accounts/${account.accountId}/permissions`, { method: 'PUT', body: { accountId: account.accountId, permissions: next, expectedRevision: account.revision } });
});
const setStatus = (account: Account, status: string) => {
  if (status === 'deleted' && !window.confirm(`确认删除账号“${account.accountName}”？`)) return;
  return run(async () => { await api(`/v1/accounts/${account.accountId}/status`, { method: 'PUT', body: { accountId: account.accountId, status, expectedRevision: account.revision } }); });
};
const reset = (account: Account) => {
  const value = window.prompt(`为“${account.accountName}”设置新的初始口令（至少 8 位）`);
  if (!value) return;
  return run(async () => { await api(`/v1/accounts/${account.accountId}/password:reset`, { method: 'POST', body: { accountId: account.accountId, initialPassword: value, expectedRevision: account.revision } }); }, '口令已重置，对方下次登录需要修改。');
};
onMounted(() => run(async () => {}));
</script>

<template>
  <div class="page">
    <h1>账号与权限</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>
    <table>
      <thead><tr><th>账号</th><th>状态</th><th>功能权限</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="a in accounts" :key="a.accountId">
          <td>{{ a.accountName }}<span v-if="a.isSuperadmin" class="hint"> （超级管理员）</span></td>
          <td>{{ ({ active: '正常', disabled: '已停用', deleted: '已删除' } as any)[a.status] ?? a.status }}</td>
          <td>
            <span v-if="a.isSuperadmin" class="hint">全部权限</span>
            <label v-else v-for="p in allPermissions" :key="p" class="perm"><input type="checkbox" :checked="a.permissions.includes(p)" :disabled="busy" @change="togglePermission(a, p)">{{ permissionLabels[p] }}</label>
          </td>
          <td>
            <div v-if="!a.isSuperadmin" class="row">
              <button v-if="a.status === 'active'" :disabled="busy" @click="setStatus(a, 'disabled')">停用</button>
              <button v-else-if="a.status === 'disabled'" :disabled="busy" @click="setStatus(a, 'active')">恢复</button>
              <button :disabled="busy" @click="reset(a)">重置口令</button>
              <button class="danger" :disabled="busy" @click="setStatus(a, 'deleted')">删除</button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
    <fieldset>
      <legend>新建子账号</legend>
      <form @submit.prevent="create">
        <div class="row">
          <input type="text" v-model="name" placeholder="账号名（创建后不可修改）" required maxlength="64">
          <input type="password" v-model="password" placeholder="初始口令（至少 8 位）" required minlength="8" autocomplete="new-password">
        </div>
        <div style="margin:.6rem 0"><label v-for="p in allPermissions" :key="p" class="perm"><input type="checkbox" :value="p" v-model="chosen">{{ permissionLabels[p] }}</label></div>
        <button class="primary" :disabled="busy">创建</button>
      </form>
    </fieldset>
  </div>
</template>
