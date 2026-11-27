import type { CSSProperties, ComponentPropsWithoutRef } from "react"

import { cn } from "@/lib/utils"

// Magic UI shimmer-button adapted for anchors: a rotating conic spark travels
// around the perimeter while the solid backdrop keeps the label readable.
export interface ShimmerButtonProps extends ComponentPropsWithoutRef<"a"> {
  shimmerColor?: string
  shimmerSize?: string
  shimmerDuration?: string
  borderRadius?: string
  background?: string
}

export function ShimmerButton({
  shimmerColor = "#ffffff",
  shimmerSize = "0.05em",
  shimmerDuration = "3s",
  borderRadius = "100px",
  background = "rgb(35 39 32)",
  className,
  children,
  style,
  ...props
}: ShimmerButtonProps) {
  return (
    <a
      style={
        {
          "--spread": "90deg",
          "--shimmer-color": shimmerColor,
          "--radius": borderRadius,
          "--speed": shimmerDuration,
          "--cut": shimmerSize,
          "--bg": background,
          ...style,
        } as CSSProperties
      }
      className={cn(
        "group relative z-0 inline-flex transform-gpu items-center justify-center overflow-hidden border border-white/10 px-6 py-3 whitespace-nowrap text-white transition-transform duration-300 ease-in-out [background:var(--bg)] [border-radius:var(--radius)] active:translate-y-px",
        className
      )}
      {...props}
    >
      <div className="shimmer-spark-container" aria-hidden="true">
        <div className="shimmer-spark-slide">
          <div className="shimmer-spark-rotate" />
        </div>
      </div>
      <span className="relative z-10">{children}</span>
      <div className="shimmer-highlight" aria-hidden="true" />
      <div className="shimmer-base" aria-hidden="true" />
    </a>
  )
}
