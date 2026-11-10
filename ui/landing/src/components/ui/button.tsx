import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

// shadcn/ui Button, re-skinned to Allagio tokens. "keycap" is the ONE primary per page (rule 30).
const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[10px] text-sm font-semibold transition-colors duration-150 disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0 min-h-11',
  {
    variants: {
      variant: {
        keycap: 'keycap p-0 text-base',
        default: 'bg-moss text-tinv hover:opacity-90 px-4',
        destructive: 'bg-maker text-tinv hover:opacity-90 px-4',
        outline: 'border border-line-strong bg-s1 text-t1 hover:bg-s2 px-4',
        secondary: 'bg-s2 text-t1 hover:bg-line px-4',
        ghost: 'text-t1 hover:bg-s2 px-3',
        link: 'text-setup underline underline-offset-4 hover:no-underline px-0 min-h-0',
        solid: 'bg-t1 text-s0 hover:bg-[rgb(60_66_55)] px-5',
        inverse: 'bg-s0 text-t1 hover:bg-s2 px-4',
      },
      size: { default: '', sm: 'min-h-9 text-[13px]', lg: 'text-base', icon: 'size-11 p-0' },
    },
    defaultVariants: { variant: 'outline', size: 'default' },
  },
)

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> { asChild?: boolean }

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(({ className, variant, size, asChild, children, ...props }, ref) => {
  const Comp = asChild ? Slot : 'button'
  if (variant === 'keycap') {
    // keycap needs an inner face span
    if (asChild && React.isValidElement(children)) {
      const child = children as React.ReactElement<{ children?: React.ReactNode }>
      return <Slot ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props}>{React.cloneElement(child, {}, <span>{child.props.children}</span>)}</Slot>
    }
    return <button ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props}><span>{children}</span></button>
  }
  return <Comp ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props}>{children}</Comp>
})
Button.displayName = 'Button'
export { buttonVariants }
