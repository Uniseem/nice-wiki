import vue from '@vitejs/plugin-vue';
import { defineConfig } from 'vite';
export default defineConfig({ plugins: [vue()], define: { 'process.env.NODE_ENV': '"production"' }, build: { outDir: 'dist', manifest: true, sourcemap: false, lib: { entry: 'src/main.ts', formats: ['es'], fileName: () => 'module.js' } } });
