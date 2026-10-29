import { useEffect, useState } from 'react'
import { hostSpeech, hostSpeechUnknown } from '@/protocol/hosted.mjs'

// Reads this origin's discovery document. A failed or unreadable check stays
// neutral. It does not guess that the host is silent.
export function useHostSpeech() {
  const [line, setLine] = useState(hostSpeechUnknown)
  useEffect(() => {
    const ac = new AbortController()
    fetch('/.well-known/orp.json', { headers: { accept: 'application/json' }, cache: 'no-store', signal: ac.signal })
      .then(async (res) => {
        if (!res.ok) throw new Error(String(res.status))
        return res.json()
      })
      .then((doc) => {
        if (!ac.signal.aborted) setLine(hostSpeech(doc))
      })
      .catch(() => {
        if (!ac.signal.aborted) setLine(hostSpeechUnknown)
      })
    return () => ac.abort()
  }, [])
  return line
}
