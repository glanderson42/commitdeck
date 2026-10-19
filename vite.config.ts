import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";
import { devCore } from "./scripts/dev-core";
export default defineConfig({
  plugins: [react(), devCore()],
  server: {
    port: 1420,
    strictPort: true,
    watch: { ignored: ["**/src-tauri/**", "**/bin/**"] },
  },
  clearScreen: false,
  optimizeDeps: { entries: ["index.html"] },
  test: {
    environment: "jsdom",
    globals: true,
    include: ["src/**/*.test.{ts,tsx}", "scripts/**/*.test.ts"],
  },
});
