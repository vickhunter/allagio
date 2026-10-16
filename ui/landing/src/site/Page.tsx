import type { ReactNode } from 'react'
import { Header, type PageKey } from './Header'
import { Footer } from './Footer'
export function Page({ page, children }: { page: PageKey; children: ReactNode }) {
  return (
    <div id="top" className="min-h-screen">
      <a href="#main" className="skip sr-only focus:not-sr-only focus:fixed focus:left-3 focus:top-3 focus:z-50 focus:inline-flex focus:min-h-11 focus:items-center focus:rounded-md focus:bg-s1 focus:px-3">Skip to content</a>
      <Header page={page} />
      <main id="main">{children}</main>
      <Footer />
    </div>
  )
}
