import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'
import { resolve } from 'node:path'

// v9: home (C editorial) + two deep-dives served at /makers/ and /publishers/.
// Assets are rooted at /landing-v9/ so they do not collide with /assets/*.
export default defineConfig({
  base: '/landing-v9/',
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': resolve(__dirname, 'src') } },
  build: {
    target: 'es2022',
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: { index: resolve(__dirname, 'index.html'), makers: resolve(__dirname, 'makers/index.html'), publishers: resolve(__dirname, 'publishers/index.html'), publishersSetup: resolve(__dirname, 'publishers/setup/index.html'), protocol: resolve(__dirname, 'protocol/index.html'), you: resolve(__dirname, 'you/index.html'), privacy: resolve(__dirname, 'privacy/index.html'), terms: resolve(__dirname, 'terms/index.html'), agents: resolve(__dirname, 'agents/index.html'), notFound: resolve(__dirname, 'not-found/index.html'), profile: resolve(__dirname, 'profile/index.html') },
    },
  },
})
