import vue from '@vitejs/plugin-vue';
import { cpSync } from 'node:fs';
import { resolve } from 'node:path';
import { defineConfig } from 'vite';

// Excalidraw 默认从 esm.sh 取字体（被 CSP 挡住，也不该依赖外网）：构建时把字体放到 module.js 旁边，由 drawing-board 指过来。
const excalidrawFonts = () => {
  let outDir = '';
  return {
    name: 'excalidraw-fonts',
    configResolved(config: { root: string; build: { outDir: string } }) { outDir = resolve(config.root, config.build.outDir); },
    closeBundle() {
      cpSync(resolve('node_modules/@excalidraw/excalidraw/dist/prod/fonts'), resolve(outDir, 'excalidraw/fonts'), { recursive: true });
    },
  };
};

export default defineConfig({
  plugins: [vue(), excalidrawFonts()],
  define: { 'process.env.NODE_ENV': '"production"' }, // 库模式不会自动替换
  build: {
    outDir: 'dist',
    manifest: true,
    sourcemap: false,
    lib: {
      entry: 'src/main.ts',
      formats: ['es'],
      fileName: () => 'module.js',
    },
  },
});
