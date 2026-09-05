import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";

// Standalone Vitest config (kept separate from vite.config.ts so tooling that
// sniffs `vitest.config.*` detects the correct test runner for this repo).
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: "jsdom",
    include: ["src/**/*.vitest.ts"],
  },
});
