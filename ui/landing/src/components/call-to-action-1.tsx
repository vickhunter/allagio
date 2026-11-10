import { ChevronRight } from "lucide-react"

import { cn } from "@/lib/utils"
import { buttonVariants } from "@/components/ui/button"
import { Marquee } from "@/components/ui/marquee"
import { example, feed } from "@/content"
import { signInHref, withSource } from "@/lib/source"

const firstRow = feed.slice(0, 3)
const secondRow = feed.slice(3)

const whoLabel: Record<string, string> = {
  maker: "Maker",
  agent: "Agent",
  pub: "Publisher",
}

const FeedCard = ({
  who,
  text,
  meta,
}: {
  who: string
  text: string
  meta: string
}) => {
  return (
    <figure
      className={cn(
        "relative w-72 overflow-hidden rounded-2xl border p-4",
        "border-line bg-card/90 hover:bg-card"
      )}
    >
      <figcaption className="eyebrow">{whoLabel[who] ?? who}</figcaption>
      <p className="mt-2 text-sm font-medium text-t1">{text}</p>
      <p className="mt-1 font-mono text-xs text-t2">{meta}</p>
    </figure>
  )
}

export function CallToAction({ source }: { source: string }) {
  const signIn = signInHref("/owner", source)
  return (
    <section id="cta" aria-labelledby="cta-h">
      <div className="container mx-auto flex w-full flex-col items-center justify-center p-4 md:p-10">
        <div className="relative flex w-full flex-col items-center justify-center overflow-hidden rounded-3xl border border-line bg-s1 p-10">
          <div className="absolute rotate-35" aria-hidden="true">
            <Marquee pauseOnHover className="[--duration:20s]" repeat={3}>
              {firstRow.map((item) => (
                <FeedCard key={`a-${item.text}`} {...item} />
              ))}
            </Marquee>
            <Marquee reverse pauseOnHover className="[--duration:20s]" repeat={3}>
              {secondRow.map((item) => (
                <FeedCard key={`b-${item.text}`} {...item} />
              ))}
            </Marquee>
            <Marquee pauseOnHover className="[--duration:20s]" repeat={3}>
              {firstRow.map((item) => (
                <FeedCard key={`c-${item.text}`} {...item} />
              ))}
            </Marquee>
            <Marquee reverse pauseOnHover className="[--duration:20s]" repeat={3}>
              {secondRow.map((item) => (
                <FeedCard key={`d-${item.text}`} {...item} />
              ))}
            </Marquee>
          </div>
          <div className="z-10 flex flex-col items-center text-center">
            <p className="eyebrow">Worked example · illustrative</p>
            <h2
              id="cta-h"
              className="h-display mt-3 max-w-xl text-3xl font-semibold tracking-tighter text-balance text-t1 sm:text-4xl"
            >
              On a {example.price} sale at {example.commission}, the publisher
              earns {example.publisher}.
            </h2>
            <p className="mt-3 max-w-xl text-base text-balance text-t2">
              Give your agent this line to start. Copying it does not create a
              token.
            </p>
            <div className="mt-5 flex flex-col items-center gap-3">
              <a
                href="#join"
                className={cn(buttonVariants({ size: "lg" }), "rounded-full px-6")}
              >
                Back to the join line
              </a>
              <div className="flex flex-wrap items-center justify-center gap-4">
                <a
                  href={signIn}
                  data-track="join_submit"
                  data-track-cta="Sign in with Google"
                  className={cn(
                    buttonVariants({ size: "lg", variant: "outline" }),
                    "group rounded-full px-6"
                  )}
                >
                  Sign in with Google
                  <ChevronRight
                    className="ml-1 size-4 transition-all duration-300 ease-out group-hover:translate-x-1"
                    aria-hidden="true"
                  />
                </a>
                <a
                  href={withSource("/agents/", source)}
                  data-track="secondary_click"
                  data-track-to="agents"
                  className="inline-flex min-h-11 items-center font-semibold text-t2 underline underline-offset-4 hover:no-underline"
                >
                  MCP and setup
                </a>
              </div>
            </div>
          </div>
          <div className="absolute inset-x-0 bottom-0 h-full bg-linear-to-b from-transparent to-s1 to-70%" />
        </div>
      </div>
    </section>
  )
}
