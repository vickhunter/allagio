declare module '@/lib/fee.mjs' {
  export function splitSale(saleCents: number, commissionBps: number, feeOfCommissionBps: number): {
    saleCents: number
    commissionBps: number
    feeOfCommissionBps: number
    publisherCents: number
    feeCents: number
    makerCents: number
  }
  export const notebookSplit: {
    saleCents: number
    commissionBps: number
    feeOfCommissionBps: number
    publisherCents: number
    feeCents: number
    makerCents: number
  }
  export function euro(cents: number): string
}

declare module '@/lib/modes.mjs' {
  export const approvalModes: readonly { id: string; name: string; note: string }[]
  export const signInStory: string
  export const personSignsInBecause: string
}
