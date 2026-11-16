import type { ReactNode } from 'react'
import {
  BarChart3,
  Bookmark,
  Heart,
  ListMusic,
  Mail,
  MessageCircle,
  Mic,
  Music2,
  Pin,
  Play,
  Repeat2,
  Share2,
  SquarePlay,
} from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import type { ShowcaseId, ShowcaseMock } from '@/i18n/messages'

export function platformFromHostname(hostname: string): ShowcaseId {
  const host = hostname.toLowerCase()
  if (/(^|\.)(x\.com|twitter\.com)$/.test(host)) return 'x'
  if (host.includes('pinterest.') || /(^|\.)pin\.it$/.test(host)) return 'pinterest'
  if (/(^|\.)instagram\.com$/.test(host)) return 'instagram'
  if (/(^|\.)tiktok\.com$/.test(host)) return 'tiktok'
  if (/(^|\.)youtube\.com$/.test(host) || /(^|\.)youtu\.be$/.test(host)) return 'youtube'
  if (host.endsWith('substack.com') || /(^|\.)(beehiiv\.com|buttondown\.com|buttondown\.email)$/.test(host)) return 'newsletter'
  return 'blog'
}

function OfferBlock({ line, tag, demo, dark }: { line: string; tag: string; demo?: string; dark?: boolean }) {
  return (
    <div className={dark ? 'mt-3 rounded-lg border border-white/25 bg-white/10 p-2.5' : 'mt-3 rounded-lg border border-pub/30 bg-pub-soft/60 p-2.5'}>
      <span className="flex flex-wrap items-center gap-1.5">
        <span className={dark ? 'text-[10px] font-semibold uppercase tracking-wider text-white/80' : 'text-[10px] font-semibold uppercase tracking-wider text-pub'}>{tag}</span>
        {demo && <Badge variant="neutral" className="px-1.5 py-0 text-[9px]">{demo}</Badge>}
      </span>
      <p className={dark ? 'mt-1 text-[13px] leading-snug text-white' : 'mt-1 text-[13px] leading-snug text-t1'}>
        <mark className="post-hi">{line}</mark>
      </p>
    </div>
  )
}

function Avatar() {
  return <div className="h-10 w-10 shrink-0 rounded-full bg-linear-to-br from-setup to-pub" aria-hidden="true" />
}

function XFrame({ mock, tag, demo }: { mock: ShowcaseMock; tag: string; demo?: string }) {
  return (
    <div className="rounded-2xl border border-line bg-s0 p-4">
      <div className="flex items-center gap-3">
        <Avatar />
        <div className="min-w-0">
          <p className="truncate text-sm font-bold text-t1">{mock.author}</p>
          <p className="truncate text-sm text-t2">{mock.handle}</p>
        </div>
        <span className="ml-auto font-display text-lg font-bold text-t1" aria-hidden="true">𝕏</span>
      </div>
      <p className="mt-3 text-[15px] leading-relaxed text-t1">{mock.body}</p>
      {mock.link && (
        <div className="mt-3 overflow-hidden rounded-xl border border-line" aria-hidden="true">
          <div className="h-24 bg-linear-to-r from-setup/70 via-pub/60 to-amber-300/70" />
          <div className="bg-s1 p-2.5">
            <p className="truncate text-xs font-semibold text-t1">{mock.link.title}</p>
            <p className="mt-0.5 truncate text-xs text-t2">{mock.link.domain}</p>
          </div>
        </div>
      )}
      <OfferBlock line={mock.offer} tag={tag} demo={demo} />
      <div className="mt-3 flex items-center justify-between text-t2" aria-hidden="true">
        <MessageCircle className="h-4 w-4" />
        <Repeat2 className="h-4 w-4" />
        <Heart className="h-4 w-4" />
        <BarChart3 className="h-4 w-4" />
      </div>
    </div>
  )
}

function PinterestFrame({ mock, tag, demo }: { mock: ShowcaseMock; tag: string; demo?: string }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-line bg-s0">
      <div className="relative aspect-[4/5] bg-linear-to-br from-amber-200 via-orange-100 to-stone-300">
        <div className="absolute left-6 top-10 h-24 w-32 rounded-xl bg-s1/90 shadow-sm" aria-hidden="true" />
        <div className="absolute left-10 top-16 h-10 w-24 rounded-t-full bg-linear-to-br from-stone-400 to-stone-500" aria-hidden="true" />
        <div className="absolute bottom-8 right-8 h-20 w-14 rounded-lg bg-linear-to-br from-pub/40 to-pub/70" aria-hidden="true" />
        <div className="absolute right-5 top-5 flex h-8 w-8 items-center justify-center rounded-full bg-s1/90" aria-hidden="true">
          <Pin className="h-4 w-4 text-t1" />
        </div>
      </div>
      <div className="p-4">
        <p className="font-display text-base font-bold text-t1">{mock.title}</p>
        <p className="mt-1 text-sm leading-relaxed text-t2">{mock.body}</p>
        <OfferBlock line={mock.offer} tag={tag} demo={demo} />
        <p className="mt-2 text-xs text-t2">{mock.handle}</p>
      </div>
    </div>
  )
}

function InstagramFrame({ mock, tag, demo }: { mock: ShowcaseMock; tag: string; demo?: string }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-line bg-s0">
      <div className="flex items-center gap-2.5 p-3">
        <div className="h-8 w-8 shrink-0 rounded-full bg-linear-to-br from-rose-300 via-amber-200 to-setup" aria-hidden="true" />
        <p className="truncate text-sm font-semibold text-t1">{mock.author}</p>
      </div>
      <div className="relative aspect-square bg-linear-to-br from-sky-200 via-teal-100 to-orange-200">
        <div className="absolute bottom-0 left-0 right-0 h-1/3 bg-linear-to-t from-stone-400/50 to-transparent" aria-hidden="true" />
        <div className="absolute left-1/2 top-1/2 h-16 w-10 -translate-x-1/2 -translate-y-1/4 rounded-full bg-s1/60 blur-[1px]" aria-hidden="true" />
      </div>
      <div className="p-3">
        <div className="flex items-center gap-3 text-t1" aria-hidden="true">
          <Heart className="h-5 w-5" />
          <MessageCircle className="h-5 w-5" />
          <Share2 className="h-5 w-5" />
          <Bookmark className="ml-auto h-5 w-5" />
        </div>
        <p className="mt-2 text-sm font-semibold text-t1">{mock.meta}</p>
        <p className="mt-1 text-sm leading-relaxed text-t1"><span className="font-semibold">{mock.author} </span>{mock.body}</p>
        <OfferBlock line={mock.offer} tag={tag} demo={demo} />
      </div>
    </div>
  )
}

function TikTokFrame({ mock, tag, demo }: { mock: ShowcaseMock; tag: string; demo?: string }) {
  return (
    <div className="relative mx-auto aspect-[9/16] max-h-[430px] w-full max-w-[242px] overflow-hidden rounded-2xl bg-t1">
      <div className="absolute inset-0 bg-linear-to-b from-stone-600 via-stone-500 to-amber-700/80" aria-hidden="true" />
      <div className="absolute left-0 right-0 top-1/3 mx-auto h-28 w-28 rounded-2xl bg-s1/20 blur-sm" aria-hidden="true" />
      <div className="absolute bottom-16 right-2.5 flex flex-col items-center gap-4 text-white" aria-hidden="true">
        <Heart className="h-6 w-6" />
        <MessageCircle className="h-6 w-6" />
        <Bookmark className="h-6 w-6" />
        <Share2 className="h-6 w-6" />
      </div>
      <div className="absolute bottom-0 left-0 right-0 p-3 pr-12">
        <p className="text-sm font-semibold text-white">@{mock.author}</p>
        <p className="mt-1 text-[13px] leading-snug text-white/90">{mock.body}</p>
        <OfferBlock line={mock.offer} tag={tag} demo={demo} dark />
        <p className="mt-2 flex items-center gap-1.5 truncate text-xs text-white/80">
          <Music2 className="h-3 w-3 shrink-0" aria-hidden="true" />{mock.meta}
        </p>
      </div>
    </div>
  )
}

function YouTubeFrame({ mock, tag, demo }: { mock: ShowcaseMock; tag: string; demo?: string }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-line bg-s0">
      <div className="relative flex aspect-video items-center justify-center bg-linear-to-br from-stone-700 via-stone-600 to-setup/70">
        <SquarePlay className="h-14 w-14 text-white/90" aria-hidden="true" />
        <div className="absolute bottom-2 right-2 rounded bg-black/70 px-1.5 py-0.5 text-xs font-medium text-white" aria-hidden="true">12:48</div>
      </div>
      <div className="p-4">
        <p className="font-display text-base font-bold leading-snug text-t1">{mock.title}</p>
        <div className="mt-2 flex items-center gap-2.5">
          <div className="h-8 w-8 shrink-0 rounded-full bg-linear-to-br from-pub to-setup" aria-hidden="true" />
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold text-t1">{mock.author}</p>
            <p className="truncate text-xs text-t2">{mock.meta}</p>
          </div>
        </div>
        <div className="mt-3 rounded-xl bg-s2/60 p-3">
          <p className="text-[13px] leading-relaxed text-t2">{mock.body}</p>
          <OfferBlock line={mock.offer} tag={tag} demo={demo} />
        </div>
      </div>
    </div>
  )
}

function NewsletterFrame({ mock, tag, demo }: { mock: ShowcaseMock; tag: string; demo?: string }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-line bg-s0">
      <div className="flex items-center gap-2.5 border-b border-line p-3">
        <Mail className="h-4 w-4 shrink-0 text-t2" aria-hidden="true" />
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-t1">{mock.author}</p>
          <p className="truncate text-xs text-t2">{mock.handle}</p>
        </div>
      </div>
      <div className="p-4">
        <p className="font-display text-lg font-bold leading-snug text-t1">{mock.title}</p>
        <p className="mt-2 text-sm leading-relaxed text-t1">{mock.body}</p>
        <div className="mt-3 grid gap-2" aria-hidden="true">
          <div className="h-2 w-full rounded-full bg-s2" />
          <div className="h-2 w-3/4 rounded-full bg-s2" />
        </div>
        <OfferBlock line={mock.offer} tag={tag} demo={demo} />
        <p className="mt-3 border-t border-line pt-2 text-xs text-t2">{mock.meta}</p>
      </div>
    </div>
  )
}

function BlogFrame({ mock, tag, demo }: { mock: ShowcaseMock; tag: string; demo?: string }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-line bg-s0">
      <div className="relative h-28 bg-linear-to-r from-s2 via-pub-soft to-setup-soft" aria-hidden="true">
        <div className="absolute left-5 top-1/2 h-10 w-16 -translate-y-1/2 rounded-lg bg-s1/80 shadow-sm" />
        <div className="absolute right-8 top-1/2 h-14 w-14 -translate-y-1/2 rounded-full bg-pub/30" />
      </div>
      <div className="p-5">
        <p className="text-xs text-t2">{mock.author}{mock.meta ? ` · ${mock.meta}` : ''}</p>
        <p className="mt-1 font-display text-xl font-bold leading-snug text-t1">{mock.title}</p>
        <p className="mt-3 text-[15px] leading-relaxed text-t1">{mock.body}</p>
        <OfferBlock line={mock.offer} tag={tag} demo={demo} />
        <div className="mt-4 grid gap-2" aria-hidden="true">
          <div className="h-2.5 w-full rounded-full bg-s2" />
          <div className="h-2.5 w-11/12 rounded-full bg-s2" />
          <div className="h-2.5 w-4/5 rounded-full bg-s2" />
        </div>
      </div>
    </div>
  )
}

const waveHeights = [10, 22, 14, 30, 18, 26, 12, 34, 20, 28, 14, 24, 10, 30, 16, 26, 12, 20, 32, 14, 22, 10, 28, 18, 24, 12, 30, 16, 20, 26]

function PodcastFrame({ mock, tag, demo }: { mock: ShowcaseMock; tag: string; demo?: string }) {
  return (
    <div className="rounded-2xl border border-line bg-s0 p-4">
      <div className="flex items-center gap-3">
        <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-linear-to-br from-stone-700 to-setup text-white" aria-hidden="true">
          <Mic className="h-6 w-6" />
        </div>
        <div className="min-w-0">
          <p className="truncate text-xs text-t2">{mock.author}{mock.handle ? ` · ${mock.handle}` : ''}</p>
          <p className="mt-0.5 text-sm font-bold leading-snug text-t1">{mock.title}</p>
        </div>
      </div>
      <div className="mt-3 flex h-9 items-center gap-[3px]" aria-hidden="true">
        <Play className="h-4 w-4 shrink-0 fill-t1 text-t1" />
        {waveHeights.map((height, i) => (
          <div key={i} className="w-1 rounded-full bg-pub/70" style={{ height: `${height}px` }} />
        ))}
      </div>
      <OfferBlock line={mock.offer} tag={tag} demo={demo} />
      <p className="mt-2 flex items-center gap-1.5 text-xs text-t2">
        <ListMusic className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />{mock.meta}
      </p>
    </div>
  )
}

const frames: Record<ShowcaseId, (props: { mock: ShowcaseMock; tag: string; demo?: string }) => ReactNode> = {
  x: XFrame,
  pinterest: PinterestFrame,
  instagram: InstagramFrame,
  tiktok: TikTokFrame,
  youtube: YouTubeFrame,
  newsletter: NewsletterFrame,
  blog: BlogFrame,
  podcast: PodcastFrame,
}

export function PlatformPostFrame({ platform, mock, tag, demo }: { platform: ShowcaseId; mock: ShowcaseMock; tag: string; demo?: string }) {
  const Frame = frames[platform] ?? frames.blog
  return <Frame mock={mock} tag={tag} demo={demo} />
}
