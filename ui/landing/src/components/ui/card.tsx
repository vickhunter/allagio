import * as React from 'react'
import { cn } from '@/lib/utils'
// shadcn/ui Card. Panels are the unit of focus: 1px warm border, no shadow.
export const Card = ({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) => <div className={cn('rounded-2xl border border-line bg-s1 text-t1', className)} {...p} />
export const CardHeader = ({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) => <div className={cn('flex flex-col gap-1.5 p-6 pb-3', className)} {...p} />
export const CardTitle = ({ className, ...p }: React.HTMLAttributes<HTMLHeadingElement>) => <h3 className={cn('font-display text-lg font-semibold tracking-tight', className)} {...p} />
export const CardDescription = ({ className, ...p }: React.HTMLAttributes<HTMLParagraphElement>) => <p className={cn('text-sm leading-relaxed text-t2', className)} {...p} />
export const CardContent = ({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) => <div className={cn('p-6 pt-0', className)} {...p} />
