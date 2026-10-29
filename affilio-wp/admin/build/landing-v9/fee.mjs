// The one sale split. Home, /makers/, and /landing-v9/how-it-works/ read this.
// Cents, so the figure cannot drift by a float. Commission is a share of the
// sale. The fee is a share of that commission and is paid by the maker.
// The maker keeps the sale minus the publisher commission minus the fee.

export function splitSale(saleCents, commissionBps, feeOfCommissionBps) {
  const publisherCents = Math.round((saleCents * commissionBps) / 10000)
  const feeCents = Math.round((publisherCents * feeOfCommissionBps) / 10000)
  const makerCents = saleCents - publisherCents - feeCents
  return { saleCents, commissionBps, feeOfCommissionBps, publisherCents, feeCents, makerCents }
}

// €24.00 sale, 10% commission, fee 10% of that commission.
export const notebookSplit = splitSale(2400, 1000, 1000)

export function euro(cents) {
  const sign = cents < 0 ? '-' : ''
  const abs = Math.abs(cents)
  return `${sign}€${(abs / 100).toFixed(2)}`
}
