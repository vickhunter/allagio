import { LegalDocument } from '@/legal/Document'
import { privacy } from '@/legal/copy'

export function App() {
  return (
    <LegalDocument
      title={privacy.title}
      lede={privacy.lede}
      sections={privacy.sections}
      otherHref="/terms"
      otherLabel="Read the terms draft"
    />
  )
}
