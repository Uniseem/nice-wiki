import { cpSync } from 'node:fs';
import vue from '@vitejs/plugin-vue';
import { defineConfig } from 'vite';

// 公开页的公式渲染用 KaTeX：构建后把它的脚本、样式和字体放到 dist/katex，由 portal 以 /_admin/katex/ 提供。
const katexAssets = () => ({
  name: 'katex-assets',
  closeBundle() {
    for (const name of ['katex.min.js', 'katex.min.css', 'fonts']) {
      cpSync(`node_modules/katex/dist/${name}`, `${this.environment?.config?.build?.outDir ?? 'dist'}/katex/${name}`, { recursive: true });
    }
  },
});

// 后台壳由 portal 在 /_admin/ 下提供静态文件，所以资源基址固定为 /_admin/。
export default defineConfig({
  base: '/_admin/',
  plugins: [vue(), katexAssets()],
  build: { outDir: 'dist', sourcemap: false, emptyOutDir: true },
});
