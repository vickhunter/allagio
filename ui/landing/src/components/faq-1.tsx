import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion"
import { useLocale } from "@/i18n"

export function FAQ() {
  const { t } = useLocale()
  return (
    <section id="faq" aria-labelledby="faq-h">
      <div className="mx-auto max-w-6xl px-5 py-16 md:py-20">
        <div className="grid gap-10 lg:grid-cols-[minmax(0,0.42fr)_minmax(0,0.58fr)] lg:items-start">
          <div>
            <h2
              id="faq-h"
              className="h-display text-3xl font-bold tracking-tighter text-balance text-t1 sm:text-4xl"
            >
              {t.faq.heading}
            </h2>
          </div>
          <Accordion
            type="single"
            collapsible
            className="w-full border-t border-line"
          >
            {t.faq.items.map((item) => (
              <AccordionItem
                key={item.q}
                value={item.q}
                className="w-full border-line"
              >
                <AccordionTrigger className="text-left text-base font-semibold text-t1">
                  {item.q}
                </AccordionTrigger>
                <AccordionContent className="text-left text-sm leading-relaxed text-t2 sm:text-base">
                  {item.a}
                </AccordionContent>
              </AccordionItem>
            ))}
          </Accordion>
        </div>
      </div>
    </section>
  )
}
