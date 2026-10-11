import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The API server used by "npm run dev", started separately (e.g. "make run").
const apiTarget = process.env.ENS_API_URL ?? 'https://localhost:8443'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // The bundle is served from the embedded binary, size matters less than for a public site.
    chunkSizeWarningLimit: 1024,
  },
  server: {
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true,
        secure: false,
        configure: (proxy) => {
          // The API rejects cross-origin writes; the dev server origin differs
          // from the API origin, so drop the header when proxying.
          proxy.on('proxyReq', (req) => req.removeHeader('origin'))
        },
      },
    },
  },
})
