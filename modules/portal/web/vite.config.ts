import { cpSync, existsSync, readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// 后台单页应用：壳在本目录，各域页面源码在 modules/<域>/web/src，由这里统一打包。
// 输出到仓库根的 web-dist/（或 NW_WEB_OUT），portal 以 /_admin/ 提供；公开页岛另由 public-site/web 打包进 web-dist/site。
const here = fileURLToPath(new URL('.', import.meta.url));
const root = resolve(here, '../../..');
const out = process.env.NW_WEB_OUT ? resolve(process.env.NW_WEB_OUT) : resolve(root, 'web-dist');

// Excalidraw 默认从 esm.sh 取字体（被 CSP 挡住，也不该依赖外网）：构建时复制到 /_admin/excalidraw/。
const excalidrawFonts = () => ({
  name: 'excalidraw-fonts',
  closeBundle() {
    cpSync(resolve(root, 'node_modules/@excalidraw/excalidraw/dist/prod/fonts'), resolve(out, 'excalidraw/fonts'), { recursive: true });
  },
});

// 后台译文：各模块 web/i18n.json（加上 libs/platform/web/i18n.json）是“中文原文 → 各语言译文”，
// 这里按语言拆成虚拟模块 virtual:admin-messages/<语言>，浏览器只下载当前语言（见 libs/platform/web/ui/i18n.ts）。
const adminMessages = () => {
  const prefix = 'virtual:admin-messages/';
  const files = () => [resolve(root, 'libs/platform/web/i18n.json'), ...readdirSync(resolve(root, 'modules')).map((name) => resolve(root, 'modules', name, 'web/i18n.json'))].filter((file) => existsSync(file));
  return {
    name: 'admin-messages',
    resolveId: (id: string) => (id.startsWith(prefix) ? '\0' + id : null),
    load(this: { addWatchFile: (file: string) => void }, id: string) {
      if (!id.startsWith('\0' + prefix)) return null;
      const language = id.slice(prefix.length + 1);
      const messages: Record<string, string> = {};
      for (const file of files()) {
        this.addWatchFile(file);
        const entries = JSON.parse(readFileSync(file, 'utf8')) as Record<string, Record<string, string>>;
        for (const [source, translations] of Object.entries(entries)) if (translations[language]) messages[source] = translations[language];
      }
      return `export default ${JSON.stringify(messages)};`;
    },
  };
};

export default defineConfig({
  root: here,
  base: '/_admin/',
  plugins: [react(), tailwindcss(), excalidrawFonts(), adminMessages()],
  resolve: {
    alias: { '@ui': resolve(root, 'libs/platform/web/ui'), '@modules': resolve(root, 'modules') },
  },
  define: { 'process.env.IS_PREACT': '"false"' },
  server: { fs: { allow: [root] }, proxy: { '/v1': 'http://127.0.0.1:8080' } },
  build: { outDir: out, emptyOutDir: true, sourcemap: false, chunkSizeWarningLimit: 4096 },
});
