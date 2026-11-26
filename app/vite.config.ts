import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import { viteSingleFile } from "vite-plugin-singlefile";
import { resolve } from "path";

const entry = process.env.APP_ENTRY;

export default defineConfig({
  plugins: [vue(), tailwindcss(), viteSingleFile()],
  build: {
    outDir: "dist",
    emptyOutDir: false,
    rollupOptions: {
      input: entry
        ? resolve(import.meta.dirname, "src/apps", `${entry}.html`)
        : undefined,
    },
  },
  resolve: {
    alias: {
      "@": resolve(import.meta.dirname, "src"),
      vue: "vue/dist/vue.esm-bundler.js",
    },
  },
});
