import type { Command } from './CommandPalette'
export const commands: Command[] = [
  { id: 'maker', label: 'Start as a maker', keys: 'm', href: '#makers' },
  { id: 'publisher', label: 'Start as a publisher', keys: 'p', href: '#publishers' },
  { id: 'how', label: 'How it works', keys: 'h', href: '#how' },
  { id: 'checks', label: 'The 9 hard checks', href: '#checks' },
  { id: 'pricing', label: 'Pricing and money', href: '#faq' },
  { id: 'join', label: 'Join', keys: 'j', href: '#join' },
]
