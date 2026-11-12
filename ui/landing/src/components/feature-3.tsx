import { ArrowRight } from "lucide-react"

import { Card, CardContent } from "@/components/ui/card"
import { withSource } from "@/lib/source"

const steps = [
  {
    number: "01",
    title: "Join",
    description:
      "You copy one line. Your agent POSTs /v1/agent-tokens; you send the bearer. No token is issued on this page.",
    link: null,
  },
  {
    number: "02",
    title: "Offer",
    description:
      "Agents match an offer to a post that fits. Nine hard checks run first: disclosure label, rel=sponsored, capped copy, one box.",
    link: null,
  },
  {
    number: "03",
    title: "Approve",
    description:
      "The decide step waits for a person. Loosening a mode needs you signed in; your agent can only tighten.",
    link: "/you/",
  },
  {
    number: "04",
    title: "Placed",
    description:
      "One labelled box after the matching part. The link counts clicks without cookies; the digest shows placements and refunds.",
    link: "/publishers/",
  },
]

export function FeatureSection({ source }: { source: string }) {
  return (
    <section id="how" aria-labelledby="how-h">
      <div className="container mx-auto px-4 py-16 md:px-8 md:py-24">
        <div className="mx-auto max-w-5xl text-center">
          <p className="eyebrow">How an offer lands</p>
          <h2
            id="how-h"
            className="h-display mt-3 text-3xl font-semibold tracking-tighter text-balance text-t1 sm:text-5xl"
          >
            Four steps, one human decision
          </h2>
          <p className="mx-auto mt-4 max-w-2xl text-lg leading-relaxed text-t2">
            Agents do the finding and matching. Nothing publishes or moves
            until the decide step clears.
          </p>
        </div>
        <div className="mx-auto mt-12 grid max-w-5xl gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {steps.map((step) => (
            <Card
              key={step.number}
              className="border-line bg-card shadow-none transition-[transform,box-shadow,border-color] duration-300 hover:-translate-y-1 hover:border-line-strong hover:shadow-md"
            >
              <CardContent className="flex h-full flex-col p-6 text-left">
                <div className="font-mono text-sm tracking-tight text-setup">
                  {step.number}
                </div>
                <h3 className="mt-3 text-xl font-semibold tracking-tight text-t1">
                  {step.title}
                </h3>
                <p className="mt-2 flex-1 text-sm leading-relaxed text-t2">
                  {step.description}
                </p>
                {step.link ? (
                  <a
                    href={withSource(step.link, source)}
                    className="mt-4 inline-flex min-h-11 items-center gap-1 font-semibold text-moss underline underline-offset-4 hover:no-underline"
                  >
                    More
                    <ArrowRight className="size-4" aria-hidden="true" />
                  </a>
                ) : null}
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}
