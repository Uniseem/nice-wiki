import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
export default defineConfig({ plugins: [vue()], base: "./", build: { lib: { entry: "src/main.ts", formats: ["es"], fileName: "knowledge" }, cssCodeSplit: false } });
