import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import type { Messages } from './messages'
import { en } from './en'
import { zh } from './zh'
import { hi } from './hi'
import { es } from './es'
import { fr } from './fr'

export type Lang = 'en' | 'zh' | 'hi' | 'es' | 'fr'

export const LANGS: { id: Lang; name: string }[] = [
  { id: 'en', name: 'English' },
  { id: 'zh', name: '中文' },
  { id: 'hi', name: 'हिन्दी' },
  { id: 'es', name: 'Español' },
  { id: 'fr', name: 'Français' },
]

const STORAGE_KEY = 'allagio_lang'
const bundles: Record<Lang, Messages> = { en, zh, hi, es, fr }
const valid = new Set<string>(LANGS.map((lang) => lang.id))

export function normalizeLang(raw: string | null | undefined): Lang | '' {
  const value = (raw ?? '').trim().toLowerCase()
  if (!value) return ''
  const base = value.split(/[-_]/)[0]
  return (valid.has(base) ? base : '') as Lang | ''
}

export function resolveLang(): Lang {
  if (typeof window === 'undefined') return 'en'
  const param = normalizeLang(new URLSearchParams(window.location.search).get('lang'))
  if (param) {
    try { window.localStorage.setItem(STORAGE_KEY, param) } catch { /* private mode */ }
    return param
  }
  try {
    const stored = normalizeLang(window.localStorage.getItem(STORAGE_KEY))
    if (stored) return stored
  } catch { /* private mode */ }
  for (const lang of navigator.languages ?? [navigator.language]) {
    const hit = normalizeLang(lang)
    if (hit) return hit
  }
  return 'en'
}

type Locale = { lang: Lang; t: Messages; setLang: (lang: Lang) => void }
const LangContext = createContext<Locale | null>(null)

export function LangProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => resolveLang())
  useEffect(() => {
    document.documentElement.lang = lang
  }, [lang])
  const setLang = (next: Lang) => {
    setLangState(next)
    try { window.localStorage.setItem(STORAGE_KEY, next) } catch { /* private mode */ }
    const url = new URL(window.location.href)
    url.searchParams.set('lang', next)
    window.history.replaceState(null, '', url)
  }
  return <LangContext.Provider value={{ lang, t: bundles[lang], setLang }}>{children}</LangContext.Provider>
}

export function useLocale(): Locale {
  const ctx = useContext(LangContext)
  return ctx ?? { lang: 'en', t: en, setLang: () => {} }
}
