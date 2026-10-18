import { LegalDocument } from '@/legal/Document'
import { terms } from '@/legal/copy'

export function App() {
  return (
    <LegalDocument
      title={terms.title}
      lede={terms.lede}
      sections={terms.sections}
      otherHref="/privacy"
      otherLabel="Read the privacy draft"
    />
  )
}
