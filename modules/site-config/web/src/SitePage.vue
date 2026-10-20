<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

const config = ref<any>(null);
const error = ref('');
const notice = ref('');
const busy = ref(false);

// 各分组表单的本地副本；保存后重新读取，使用最新 revision 做乐观并发检查。
const basic = ref({ name: '', themeColor: '', description: '', titleTemplate: '', indexingEnabled: true });
const footer = ref({ copyright: '', icpNumber: '', icpLink: '', sourceCodeURL: '' });
const home = ref({ mainTitle: '', subtitle: '', backgroundText: '' });

async function load() {
  const result = await api<{ config: any }>('/v1/config');
  config.value = result.config;
  const c = result.config;
  basic.value = { name: c.name, themeColor: c.themeColor, description: c.description, titleTemplate: c.titleTemplate, indexingEnabled: c.indexingEnabled };
  footer.value = { copyright: c.footer.copyright, icpNumber: c.footer.icpNumber ?? '', icpLink: c.footer.icpLink ?? '', sourceCodeURL: c.footer.sourceCodeURL };
  home.value = { mainTitle: c.home.mainTitle, subtitle: c.home.subtitle, backgroundText: c.home.backgroundText };
}

async function save(label: string, action: () => Promise<void>) {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); await load(); notice.value = `${label}已保存`; } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}

const saveBasic = () => save('站点设置', async () => {
  const c = config.value;
  await api('/v1/branding', { method: 'PUT', body: { name: basic.value.name, logo: c.logo, favicon: c.favicon, themeColor: basic.value.themeColor, defaultShareImage: c.defaultShareImage, expectedRevision: c.revision } });
  const fresh = (await api<{ config: any }>('/v1/config')).config;
  await api('/v1/config', { method: 'PUT', body: { publicBaseURL: fresh.publicBaseURL, titleTemplate: basic.value.titleTemplate, description: basic.value.description, indexingEnabled: basic.value.indexingEnabled, resourceLimits: fresh.resourceLimits, securityPolicy: fresh.securityPolicy, expectedRevision: fresh.revision } });
});

const saveFooter = () => save('页脚', async () => {
  const c = config.value;
  await api('/v1/footer', { method: 'PUT', body: { footer: { ...c.footer, copyright: footer.value.copyright, icpNumber: footer.value.icpNumber || null, icpLink: footer.value.icpLink || null, sourceCodeURL: footer.value.sourceCodeURL }, expectedRevision: c.revision } });
});

const saveHome = () => save('首页', async () => {
  const c = config.value;
  await api('/v1/home', { method: 'PUT', body: { home: { ...c.home, mainTitle: home.value.mainTitle, subtitle: home.value.subtitle, backgroundText: home.value.backgroundText }, expectedRevision: c.revision, expectedHomeRevision: c.home.revision } });
});

onMounted(async () => { try { await load(); } catch (e) { error.value = messageOf(e); } });
</script>

<template>
  <div class="page" v-if="config">
    <h1>站点设置</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>
    <p class="hint">公开地址：{{ config.publicBaseURL }} · 基准语言：{{ config.baseLanguage }} · 配置版本 {{ config.revision }}</p>

    <fieldset>
      <legend>基本信息</legend>
      <form @submit.prevent="saveBasic">
        <label>站点名称</label><input type="text" v-model="basic.name" required maxlength="120">
        <label>主题色</label><input type="text" v-model="basic.themeColor" placeholder="#2563EB" pattern="#[0-9A-Fa-f]{6}">
        <label>标题模板</label><input type="text" v-model="basic.titleTemplate" placeholder="%s - 站点名">
        <label>站点描述</label><input type="text" v-model="basic.description">
        <label class="perm"><input type="checkbox" v-model="basic.indexingEnabled">允许搜索引擎收录（关闭后 robots.txt 与 sitemap 将禁止索引）</label>
        <div style="margin-top:.8rem"><button class="primary" :disabled="busy">保存</button></div>
      </form>
    </fieldset>

    <fieldset>
      <legend>首页</legend>
      <form @submit.prevent="saveHome">
        <label>主标题</label><input type="text" v-model="home.mainTitle" required>
        <label>副标题</label><input type="text" v-model="home.subtitle">
        <label>背景文字</label><input type="text" v-model="home.backgroundText">
        <div style="margin-top:.8rem"><button class="primary" :disabled="busy">保存</button></div>
      </form>
    </fieldset>

    <fieldset>
      <legend>页脚</legend>
      <form @submit.prevent="saveFooter">
        <label>版权信息</label><input type="text" v-model="footer.copyright">
        <label>ICP 备案号</label><input type="text" v-model="footer.icpNumber">
        <label>ICP 备案链接</label><input type="text" v-model="footer.icpLink">
        <label>源码链接</label><input type="text" v-model="footer.sourceCodeURL" required>
        <div style="margin-top:.8rem"><button class="primary" :disabled="busy">保存</button></div>
      </form>
    </fieldset>
  </div>
  <p v-else-if="error" class="err">{{ error }}</p>
  <p v-else class="hint">加载中…</p>
</template>
