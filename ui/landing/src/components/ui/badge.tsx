import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'
const badgeVariants = cva('inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 font-mono text-[11px] font-medium uppercase tracking-wider', {
  variants: { variant: {
    neutral: 'border-line bg-s2 text-t1', maker: 'border-transparent bg-maker-soft text-maker', pub: 'border-transparent bg-pub-soft text-pub', setup: 'border-transparent bg-setup-soft text-setup-edge',
  } }, defaultVariants: { variant: 'neutral' },
})
export function Badge({ className, variant, ...p }: React.HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ variant }), className)} {...p} />
}
