import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { VitePWA } from 'vite-plugin-pwa';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),

    // NOTE: @tanstack/router-plugin is intentionally NOT used.
    //
    // It exists for file-based route generation, which would produce a
    // generated `routeTree.gen.ts` that must be regenerated before the project
    // typechecks — an extra failure mode for a route tree this small. Routes are
    // declared explicitly in src/router.tsx instead.
    //
    // If the route count ever grows past ~30 screens, revisit: the plugin's
    // autoCodeSplitting is genuinely useful at that size.

    // Registers the service worker that caches the APP SHELL.
    //
    // Important distinction: this caches HTML/JS/CSS so the application *loads*
    // with no network. It knows nothing about patients — that is Dexie's job.
    // Both are required; see docs/00-ARCHITECTURE.md §3.4.
    VitePWA({
      // 'prompt', NOT 'autoUpdate'. With autoUpdate a workstation can swap the
      // running app out from under a nurse mid-form. Let the user choose when
      // to reload.
      registerType: 'prompt',
      manifest: {
        name: 'PHC Patient Records',
        short_name: 'PHC Records',
        description: 'Offline-capable patient records and referral management',
        theme_color: '#0d9488',
        background_color: '#ffffff',
        display: 'standalone',
        start_url: '/',
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        navigateFallback: '/index.html',
        // Never let the SW answer for the API or public referral routes.
        navigateFallbackDenylist: [/^\/api/, /^\/r\//],
        // The API must never be served from cache — stale clinical data is
        // worse than no data. Dexie owns offline reads.
        runtimeCaching: [],
      },
    }),
  ],

  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },

  server: {
    port: 5173,
    // Proxy the API in development so the browser sees a single origin and
    // CORS never enters the picture locally. In production the SPA calls the
    // absolute Cloud Run URL and CORS applies.
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      // /health and /ready live at the ROOT, not under /api/v1 — they are
      // Cloud Run's probe endpoints. Without these two entries the dashboard's
      // infrastructure checks 404 against the dev server and look like an outage.
      '/health': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ready': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },

  build: {
    outDir: 'dist',
    sourcemap: true,
    // Keep the initial bundle small: this app must load fast on a slow PHC
    // link, and every kilobyte is paid for on a metered connection.
    chunkSizeWarningLimit: 600,
  },
});
