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
      <div className="py-14">
        <div className="container mx-auto px-4 md:px-8">
          <div className="mx-auto max-w-5xl text-center">
            <p className="eyebrow">{t.faq.eyebrow}</p>
            <h2
              id="faq-h"
              className="h-display mt-3 text-3xl font-semibold tracking-tighter text-balance text-t1 sm:text-5xl"
            >
              {t.faq.heading}
            </h2>
          </div>
          <div className="container mx-auto my-12 max-w-[640px] space-y-12">
            <Accordion
              type="single"
              collapsible
              className="flex w-full flex-col items-center justify-center"
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
                  <AccordionContent className="text-left text-base leading-relaxed text-t2">
                    {item.a}
                  </AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </div>
        </div>
      </div>
    </section>
  )
}
