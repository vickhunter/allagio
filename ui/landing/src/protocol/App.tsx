import { useEffect, useState } from 'react'
import { Page } from '@/site/Page'
import { Docs } from './Docs'

export function App() {
  const [pathname, setPathname] = useState(() => window.location.pathname)
  useEffect(() => {
    const sync = () => setPathname(window.location.pathname)
    window.addEventListener('popstate', sync)
    return () => window.removeEventListener('popstate', sync)
  }, [])
  return (
    <Page page="protocol">
      <Docs pathname={pathname} />
    </Page>
  )
}
