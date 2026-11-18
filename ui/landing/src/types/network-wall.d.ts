declare module '@/lib/network-wall.mjs' {
  export interface Face {
    listed: boolean
    name?: string
    role?: string
    host?: string
  }
  export interface WallDeal {
    kind: string
    state: string
    updated_at: string
    parties: number
    offerer: Face
    acceptor: Face
  }
  export interface WallOffer {
    offer_id: string
    product_name: string
    product_tagline: string
    website_url: string
    primary_category: string
    commission_type: string
    commission_value: number
    commission_currency: string
    [key: string]: unknown
  }
  export function wantsDealsFixtures(): boolean
  export function dealsFixtureData(): { deals: WallDeal[]; offers: WallOffer[] }
  export function listRecentDeals(fetchImpl?: typeof fetch): Promise<WallDeal[]>
  export function listPublicOffers(fetchImpl?: typeof fetch): Promise<WallOffer[]>
  export function partyText(face: Face, roleWords: Record<string, string>): { text: string; named: boolean }
  export function commissionText(offer: { commission_type?: string; commission_value?: number; commission_currency?: string }): string
}
