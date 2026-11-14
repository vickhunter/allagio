import type { CSSProperties, ComponentPropsWithoutRef } from "react"

import { cn } from "@/lib/utils"

// Magic UI shimmer-button adapted for anchors: an animated shine sweep plus
// a conic-gradient border flash, both CSS-driven and reduced-motion safe.
export interface ShimmerButtonProps extends ComponentPropsWithoutRef<"a"> {
  shimmerColor?: string
  shimmerSize?: string
  shimmerDuration?: string
  borderRadius?: string
  background?: string
}

export function ShimmerButton({
  shimmerColor = "#ffffff",
  shimmerSize = "0.08em",
  shimmerDuration = "3s",
  borderRadius = "100px",
  background = "var(--ink)",
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
      className={cn("group shimmer-button", className)}
      {...props}
    >
      <span className="absolute inset-0 overflow-hidden rounded-[var(--radius)]">
        <span className="shimmer-spark absolute inset-0 rounded-[var(--radius)]" />
      </span>
      <span className="relative z-10">{children}</span>
      <span className="shimmer-base absolute -z-10 inset-[var(--cut)] rounded-[var(--radius)]" />
    </a>
  )
}
