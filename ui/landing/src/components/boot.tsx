import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@/styles.css'
import { installTracking } from '@/lib/track'
import type { ReactNode } from 'react'
export function boot(version: string, node: ReactNode) {
  document.body.dataset.version = version
  document.getElementById('static-landing')?.remove()
  installTracking()
  createRoot(document.getElementById('root')!).render(<StrictMode>{node}</StrictMode>)
}
