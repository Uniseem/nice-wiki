import vue from '@vitejs/plugin-vue';
import { defineConfig } from 'vite';

// 后台壳由 portal 在 /_admin/ 下提供静态文件，所以资源基址固定为 /_admin/。
export default defineConfig({
  base: '/_admin/',
  plugins: [vue()],
  build: { outDir: 'dist', sourcemap: false, emptyOutDir: true },
});
