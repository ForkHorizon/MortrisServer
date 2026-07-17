import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Local dev only — proxies to the Go server so `npm run dev` works
    // without CORS. In production the built assets are embedded into the
    // same binary that serves /api, so this proxy doesn't exist there.
    proxy: {
      '/api': 'http://localhost:8099',
    },
  },
  build: {
    outDir: 'dist',
  },
})
