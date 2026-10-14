import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": "/src",
    },
  },
  build: {
    outDir: "dist",
    assetsDir: "assets",
    manifest: true,
    sourcemap: false,
    emptyOutDir: true,
  },
});
