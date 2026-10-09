import { defineConfig } from '/workspace/luci/desktop/node_modules/vitest/dist/config.js';
import react from '/workspace/luci/desktop/node_modules/@vitejs/plugin-react/dist/index.js';
export default defineConfig({
  root: '/workspace/luci/desktop', plugins: [react()],
  server: { fs: { allow: ['/workspace/luci', '/workspace/.luci-tools/session-authority-review-edd0'] } },
  test: {
    environment: 'jsdom', include: ['/workspace/.luci-tools/session-authority-review-edd0/authority-window.test.tsx'],
    setupFiles: ['/workspace/luci/desktop/src/test/setup.ts'], globals: false,
    restoreMocks: true, clearMocks: true, maxWorkers: 1,
  },
});
