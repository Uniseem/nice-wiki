import { cpSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// 公开页的样式表与交互岛：输出到 web-dist/site（或 NW_WEB_OUT/site），portal 以 /_site/ 提供。
// 文件名固定（site.css / site.js），Go 输出的页面按 /_site/v/<提交号>/site.js 引用（portal 去掉版本段再找文件），
// 升级后 CDN 和浏览器缓存立即失效；拆出的懒加载块带哈希。base 用相对路径，懒加载块和预加载都相对 site.js 解析，
// 保证整张模块图都在同一个带版本的路径下（混用带版本与不带版本的地址会加载出两份 React）。
const here = fileURLToPath(new URL('.', import.meta.url));
const root = resolve(here, '../../..');
const out = resolve(process.env.NW_WEB_OUT ? resolve(process.env.NW_WEB_OUT) : resolve(root, 'web-dist'), 'site');

// 公式由浏览器端 KaTeX 渲染：把它的脚本、样式和字体原样复制到 site/katex。
const katexAssets = () => ({
  name: 'katex-assets',
  closeBundle() {
    cpSync(resolve(root, 'docs/licenses/lora-OFL.txt'), resolve(out, 'lora-OFL.txt'));
    for (const name of ['katex.min.js', 'katex.min.css', 'fonts']) {
      cpSync(resolve(root, 'node_modules/katex/dist', name), resolve(out, 'katex', name), { recursive: true });
    }
  },
});

export default defineConfig({
  root: here,
  base: './',
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
