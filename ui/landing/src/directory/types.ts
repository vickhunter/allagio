export type DirectoryKind = 'maker' | 'publisher'

export type DirectoryCard = {
  id: string
  slug: string
  kind: DirectoryKind
  name: string
  url: string
  logo_url: string | null
  summary: string
  languages: string[]
  categories?: string[]
  topics?: string[]
  open_offers?: number
  monthly_pages?: number
  joined_at: string
  verified: boolean
}

export type DirectoryList = {
  items: DirectoryCard[]
  next_cursor: string | null
  total: number
}
