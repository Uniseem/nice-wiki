import { mkdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig(({ command }) => ({
  plugins: [vue(), tailwindcss(), {
    name: 'preserve-embed-placeholder',
    closeBundle() {
      const directory = fileURLToPath(new URL('../server/internal/webui/dist', import.meta.url))
      mkdirSync(directory, { recursive: true })
      writeFileSync(`${directory}/.gitkeep`, '')
    },
  }],
  base: command === 'serve' ? '/' : '/static/',
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    port: 5173,
    strictPort: true,
    origin: 'http://localhost:5173',
    proxy: { '/api': 'http://localhost:8080', '/assets': 'http://localhost:8080' },
  },
  build: {
    outDir: '../server/internal/webui/dist',
    emptyOutDir: true,
    manifest: true,
    rollupOptions: {
      input: {
        admin: fileURLToPath(new URL('./admin/index.html', import.meta.url)),
        site: fileURLToPath(new URL('./src/site/main.ts', import.meta.url)),
      },
    },
  },
}))
