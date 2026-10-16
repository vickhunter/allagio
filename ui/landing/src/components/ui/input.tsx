import * as React from 'react'
import { cn } from '@/lib/utils'
export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(({ className, ...p }, ref) => (
  <input ref={ref} className={cn('min-h-11 w-full rounded-[10px] border border-line-strong bg-s1 px-3 text-base text-t1 placeholder:text-t2 aria-[invalid=true]:border-maker', className)} {...p} />
))
Input.displayName = 'Input'
export const Kbd = ({ className, ...p }: React.HTMLAttributes<HTMLElement>) => <kbd className={cn('rounded border border-line-strong border-b-2 bg-s1 px-1.5 py-px font-mono text-[11px] text-t1', className)} {...p} />
