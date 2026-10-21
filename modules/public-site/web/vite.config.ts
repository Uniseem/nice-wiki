import { cpSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// 公开页的样式表与交互岛：输出到 web-dist/site（或 NW_WEB_OUT/site），portal 以 /_site/ 提供。
// 文件名固定（site.css / site.js），Go 输出的页面直接引用；拆出的懒加载块带哈希。
const here = fileURLToPath(new URL('.', import.meta.url));
const root = resolve(here, '../../..');
const out = resolve(process.env.NW_WEB_OUT ? resolve(process.env.NW_WEB_OUT) : resolve(root, 'web-dist'), 'site');

// 公式由浏览器端 KaTeX 渲染：把它的脚本、样式和字体原样复制到 site/katex。
const katexAssets = () => ({
  name: 'katex-assets',
  closeBundle() {
    for (const name of ['katex.min.js', 'katex.min.css', 'fonts']) {
      cpSync(resolve(root, 'node_modules/katex/dist', name), resolve(out, 'katex', name), { recursive: true });
    }
  },
});

export default defineConfig({
  root: here,
  base: '/_site/',
  publicDir: false,
  plugins: [react(), tailwindcss(), katexAssets()],
  resolve: {
    alias: { '@ui': resolve(root, 'libs/platform/web/ui'), '@modules': resolve(root, 'modules') },
  },
  build: {
    outDir: out,
    emptyOutDir: true,
    sourcemap: false,
    cssCodeSplit: false,
    rollupOptions: {
      input: resolve(here, 'src/site.tsx'),
      output: {
        entryFileNames: 'site.js',
        chunkFileNames: 'chunks/[name]-[hash].js',
        assetFileNames: (asset) => ([...(asset.names ?? []), asset.name ?? ''].some((name) => name.endsWith('.css')) ? 'site.css' : 'assets/[name]-[hash][extname]'),
      },
    },
  },
});
