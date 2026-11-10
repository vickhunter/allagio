import { cpSync, readFileSync } from 'node:fs'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'vite'
import { resolve } from 'node:path'

const feeFile = resolve(__dirname, 'src/lib/fee.mjs')

function feeModule() {
  const send = (req: { url?: string }, res: { setHeader: (name: string, value: string) => void; end: (body: string) => void }, next: () => void) => {
    const path = req.url?.split('?')[0] ?? ''
    if (path !== '/landing-v9/fee.mjs' && path !== '/fee.mjs') {
      next()
      return
    }
    res.setHeader('content-type', 'text/javascript; charset=utf-8')
    res.end(readFileSync(feeFile, 'utf8'))
  }
  return {
    name: 'offer-rails-fee-module',
    configureServer(server: { middlewares: { use: (fn: typeof send) => void } }) {
      server.middlewares.use(send)
    },
    writeBundle() {
      cpSync(feeFile, resolve(__dirname, 'dist/fee.mjs'))
    },
  }
}

// v9: home (C editorial) + two deep-dives served at /makers/ and /publishers/.
// Assets are rooted at /landing-v9/ so they do not collide with /assets/*.
export default defineConfig({
  base: '/landing-v9/',
  plugins: [react(), tailwindcss(), feeModule()],
  resolve: { alias: { '@': resolve(__dirname, 'src') } },
  build: {
    target: 'es2022',
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: { index: resolve(__dirname, 'index.html'), makers: resolve(__dirname, 'makers/index.html'), publishers: resolve(__dirname, 'publishers/index.html'), publishersSetup: resolve(__dirname, 'publishers/setup/index.html'), useCases: resolve(__dirname, 'use-cases/index.html'), protocol: resolve(__dirname, 'protocol/index.html'), you: resolve(__dirname, 'you/index.html'), privacy: resolve(__dirname, 'privacy/index.html'), terms: resolve(__dirname, 'terms/index.html'), agents: resolve(__dirname, 'agents/index.html'), notFound: resolve(__dirname, 'not-found/index.html'), profile: resolve(__dirname, 'profile/index.html') },
    },
  },
})
