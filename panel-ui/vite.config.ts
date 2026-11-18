/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath } from "node:url";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  server: {
    // During development the API runs locally (panel-api on :8080).
    proxy: { "/v1": process.env.EQ_PANEL_API ?? "http://127.0.0.1:8080" },
  },
  build: { sourcemap: false, assetsInlineLimit: 0 },
  test: { environment: "jsdom", setupFiles: ["src/test-setup.ts"], css: false },
});
