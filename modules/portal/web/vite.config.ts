import { cpSync } from 'node:fs';
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

export default defineConfig({
  root: here,
  base: '/_admin/',
  plugins: [react(), tailwindcss(), excalidrawFonts()],
  resolve: {
    alias: { '@ui': resolve(root, 'libs/platform/web/ui'), '@modules': resolve(root, 'modules') },
  },
  define: { 'process.env.IS_PREACT': '"false"' },
  server: { fs: { allow: [root] }, proxy: { '/v1': 'http://127.0.0.1:8080' } },
  build: { outDir: out, emptyOutDir: true, sourcemap: false, chunkSizeWarningLimit: 4096 },
});
