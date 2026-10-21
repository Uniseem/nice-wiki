<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';
import { uploadImage } from './upload';

const config = ref<any>(null);
const error = ref('');
const notice = ref('');
const busy = ref(false);

// 各分组表单的本地副本；保存后重新读取，使用最新 revision 做乐观并发检查。
const basic = ref({ name: '', themeColor: '', description: '', titleTemplate: '', indexingEnabled: true });
const footer = ref({ copyright: '', icpNumber: '', icpLink: '', sourceCodeURL: '' });
type Btn = { buttonId: string; text: string; style: 'primary' | 'secondary'; kind: 'external' | 'qa' | 'document'; value: string };
const home = ref({ mainTitle: '', subtitle: '', backgroundText: '', color: '', overlayPercent: 0, textColor: 'light', buttons: [] as Btn[] });
const banner = ref({ text: '', url: '', enabled: true });
const newId = () => crypto.randomUUID().replace(/-/g, '');
const targetOf = (b: { kind: string; value: string }) => b.kind === 'qa' ? { kind: 'qa' } : b.kind === 'document' ? { kind: 'document', documentId: b.value, blockId: null } : { kind: 'external', url: b.value };
const addButton = () => { if (home.value.buttons.length < 3) home.value.buttons.push({ buttonId: newId(), text: '', style: 'primary', kind: 'external', value: '' }); };

async function load() {
  const result = await api<{ config: any }>('/v1/config');
  config.value = result.config;
  const c = result.config;
  basic.value = { name: c.name, themeColor: c.themeColor, description: c.description, titleTemplate: c.titleTemplate, indexingEnabled: c.indexingEnabled };
  footer.value = { copyright: c.footer.copyright, icpNumber: c.footer.icpNumber ?? '', icpLink: c.footer.icpLink ?? '', sourceCodeURL: c.footer.sourceCodeURL };
  home.value = {
    mainTitle: c.home.mainTitle, subtitle: c.home.subtitle, backgroundText: c.home.backgroundText,
    color: c.home.background?.kind === 'color' ? c.home.background.color : '', overlayPercent: c.home.overlayPercent, textColor: c.home.textColor,
    buttons: c.home.buttons.map((b: any) => ({ buttonId: b.buttonId, text: b.text, style: b.style, kind: b.target.kind, value: b.target.url ?? b.target.documentId ?? '' })),
  };
  banner.value.enabled = c.banner ? c.banner.enabled : true;
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

// 站标/图标/分享图：选图片即上传并保存（字段名对应站点配置里的 logo / favicon / defaultShareImage），“清除”置空。
const brandingField = (field: 'logo' | 'favicon' | 'defaultShareImage', label: string, file: File | null) => save(label, async () => {
  const c = config.value;
  const ref = file ? await uploadImage(file) : null;
  const body = { name: c.name, logo: c.logo, favicon: c.favicon, themeColor: c.themeColor, defaultShareImage: c.defaultShareImage, expectedRevision: c.revision, [field]: ref };
  await api('/v1/branding', { method: 'PUT', body });
});
const pickImage = (field: 'logo' | 'favicon' | 'defaultShareImage', label: string, event: Event) => {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (file) return brandingField(field, label, file);
};

const saveFooter = () => save('页脚', async () => {
  const c = config.value;
  await api('/v1/footer', { method: 'PUT', body: { footer: { ...c.footer, copyright: footer.value.copyright, icpNumber: footer.value.icpNumber || null, icpLink: footer.value.icpLink || null, sourceCodeURL: footer.value.sourceCodeURL }, expectedRevision: c.revision } });
});

const saveHome = () => save('首页', async () => {
  const c = config.value;
  await api('/v1/home', { method: 'PUT', body: { home: {
    ...c.home, mainTitle: home.value.mainTitle, subtitle: home.value.subtitle, backgroundText: home.value.backgroundText,
    overlayPercent: Number(home.value.overlayPercent), textColor: home.value.textColor,
    background: home.value.color ? { kind: 'color', color: home.value.color } : c.home.background,
    buttons: home.value.buttons.map((b) => ({ buttonId: b.buttonId, text: b.text, style: b.style, target: targetOf(b) })),
  }, expectedRevision: c.revision, expectedHomeRevision: c.home.revision } });
});

const saveBanner = () => save('公告横幅', async () => {
  const c = config.value;
  await api('/v1/banner', { method: 'PUT', body: { announcementId: c.banner?.announcementId ?? null, body: { schemaVersion: 1, text: banner.value.text, target: banner.value.url ? { kind: 'external', url: banner.value.url } : null }, enabled: banner.value.enabled, startsAt: null, endsAt: null, expectedRevision: c.revision } });
  banner.value.text = ''; banner.value.url = '';
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
        <label>标题模板</label><input type="text" v-model="basic.titleTemplate" placeholder="{title} | {site}">
        <label>站点描述</label><input type="text" v-model="basic.description">
        <label>站标（页头显示）</label>
        <div class="row"><span class="hint">{{ config?.logo ? '已设置' : '未设置' }}</span><input type="file" accept="image/png,image/jpeg,image/webp" :disabled="busy" @change="pickImage('logo', '站标', $event)"><button v-if="config?.logo" type="button" :disabled="busy" @click="brandingField('logo', '站标', null)">清除</button></div>
        <label>浏览器图标（favicon）</label>
        <div class="row"><span class="hint">{{ config?.favicon ? '已设置' : '未设置' }}</span><input type="file" accept="image/png,image/jpeg,image/webp" :disabled="busy" @change="pickImage('favicon', '图标', $event)"><button v-if="config?.favicon" type="button" :disabled="busy" @click="brandingField('favicon', '图标', null)">清除</button></div>
        <label>默认分享图（社交平台预览）</label>
        <div class="row"><span class="hint">{{ config?.defaultShareImage ? '已设置' : '未设置' }}</span><input type="file" accept="image/png,image/jpeg,image/webp" :disabled="busy" @change="pickImage('defaultShareImage', '分享图', $event)"><button v-if="config?.defaultShareImage" type="button" :disabled="busy" @click="brandingField('defaultShareImage', '分享图', null)">清除</button></div>
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
        <label>背景颜色</label><input type="text" v-model="home.color" placeholder="#0F172A" pattern="#[0-9A-Fa-f]{6}">
        <label>遮罩浓度（0–100）</label><input type="number" min="0" max="100" v-model="home.overlayPercent">
        <label>文字颜色</label>
        <select v-model="home.textColor"><option value="light">浅色</option><option value="dark">深色</option></select>
        <label>按钮（最多 3 个）</label>
        <div v-for="(b, i) in home.buttons" :key="b.buttonId" class="row">
          <input type="text" v-model="b.text" placeholder="按钮文字" required>
          <select v-model="b.style"><option value="primary">主要</option><option value="secondary">次要</option></select>
          <select v-model="b.kind"><option value="external">外链</option><option value="document">文档</option><option value="qa">AI 问答</option></select>
          <input v-if="b.kind !== 'qa'" type="text" v-model="b.value" :placeholder="b.kind === 'document' ? '文档 ID' : 'https://…'" required>
          <button type="button" @click="home.buttons.splice(i, 1)">删除</button>
        </div>
        <button type="button" v-if="home.buttons.length < 3" @click="addButton">添加按钮</button>
        <div style="margin-top:.8rem"><button class="primary" :disabled="busy">保存</button></div>
      </form>
    </fieldset>

    <fieldset>
      <legend>公告横幅</legend>
      <p class="hint">{{ config.banner ? '当前已有一条横幅（' + (config.banner.enabled ? '展示中' : '已关闭') + '），保存会替换其内容。' : '当前没有横幅。' }}</p>
      <form @submit.prevent="saveBanner">
        <label>横幅文字</label><input type="text" v-model="banner.text" required maxlength="2000">
        <label>链接（可空）</label><input type="text" v-model="banner.url" placeholder="https://…">
        <label class="perm"><input type="checkbox" v-model="banner.enabled">展示</label>
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
