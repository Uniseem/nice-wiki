import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: "dist", assetsDir: "assets", manifest: true, sourcemap: false, emptyOutDir: true,
    lib: { entry: "src/main.ts", formats: ["es"], fileName: "model-gateway" },
  },
});
