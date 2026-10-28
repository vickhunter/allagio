import { useEffect, useState } from 'react'
import { hostSpeech } from '@/protocol/hosted.mjs'

// Reads this origin's discovery document. A failed or unreadable check stays
// null. Callers show the unknown line and do not guess a version.
export function useHostSpeech() {
  const [doc, setDoc] = useState<unknown>(null)
  useEffect(() => {
    const ac = new AbortController()
    fetch('/.well-known/orp.json', { headers: { accept: 'application/json' }, cache: 'no-store', signal: ac.signal })
      .then(async (res) => {
        if (!res.ok) throw new Error(String(res.status))
        return res.json()
      })
      .then((body) => {
        if (!ac.signal.aborted) setDoc(body)
      })
      .catch(() => {
        if (!ac.signal.aborted) setDoc(null)
      })
    return () => ac.abort()
  }, [])
  return { doc, line: hostSpeech(doc) }
}
