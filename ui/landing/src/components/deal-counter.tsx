import { useEffect, useState } from "react"

import { NumberTicker } from "@/components/ui/number-ticker"
import { useLocale } from "@/i18n"

const TICKS = 41
const CX = 150
const CY = 132
const R = 108

// Manufact-style live counter: a tick arc over an animated deal count fed by
// the public /v1/stats endpoint. Renders nothing while stats are loading or
// when the endpoint is unreachable.
export function DealCounter() {
  const { t } = useLocale()
  const [deals, setDeals] = useState<number | null>(null)

  useEffect(() => {
    let live = true
    fetch("/v1/stats")
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (live && data && typeof data.deals === "number") setDeals(data.deals)
      })
      .catch(() => {})
    return () => {
      live = false
    }
  }, [])

  if (deals === null) return null

  const ticks = Array.from({ length: TICKS }, (_, i) => {
    const angle = Math.PI + (i / (TICKS - 1)) * Math.PI
    const major = i % 5 === 0
    const inner = major ? R - 12 : R - 6
    const x1 = CX + Math.cos(angle) * inner
    const y1 = CY + Math.sin(angle) * inner
    const x2 = CX + Math.cos(angle) * R
    const y2 = CY + Math.sin(angle) * R
    return (
      <line
        key={i}
        x1={x1}
        y1={y1}
        x2={x2}
        y2={y2}
        strokeWidth={major ? 2 : 1}
        strokeLinecap="round"
        className="stroke-line/40"
      />
    )
  })

  return (
    <section aria-labelledby="counter-h" className="border-t border-line py-16 text-center">
      <p className="kicker">{t.counter.kicker}</p>
      <h2 id="counter-h" className="sr-only">
        {t.counter.kicker}
      </h2>
      <div className="mx-auto mt-4 max-w-sm">
        <svg viewBox="0 0 300 140" aria-hidden="true" className="mx-auto w-full max-w-xs">
          {ticks}
        </svg>
        <p className="mt-[-3.5rem] text-sm text-t2">{t.counter.caption}</p>
        <p className="mt-2 text-6xl font-semibold tracking-tight sm:text-7xl">
          <NumberTicker value={deals} />
        </p>
      </div>
    </section>
  )
}
