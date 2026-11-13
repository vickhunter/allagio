import { cn } from '@/lib/utils'
import { LANGS, useLocale, type Lang } from '.'

// Compact locale picker: native <select> shows the current language's own
// name and keeps keyboard/AT behaviour for free.
export function LangSwitcher({ className }: { className?: string }) {
  const { lang, setLang, t } = useLocale()
  return (
    <select
      aria-label={t.chrome.language}
      value={lang}
      onChange={(event) => setLang(event.target.value as Lang)}
      data-track="lang_switch"
      className={cn(
        'min-h-11 cursor-pointer rounded-full border border-line bg-s1 px-2.5 text-[13px] font-semibold text-t1 transition-colors duration-150 hover:bg-s2',
        className
      )}
    >
      {LANGS.map((item) => (
        <option key={item.id} value={item.id}>
          {item.name}
        </option>
      ))}
    </select>
  )
}
