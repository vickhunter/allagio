import { useState } from 'react'
import { catalog } from '@/protocol/generated/catalog.mjs'
import { noMoney, notSpoken04 } from '@/protocol/hosted.mjs'
import { useSource, withSource } from '@/lib/source'

type Money = { amount_minor: number; currency: string }
type Seat = {
  id: string
  role: string
  required: boolean
  signed?: boolean
  terms?: {
    percent?: number
    amount?: Money
    per?: string
    hours?: number
    deposit?: { amount: Money; holder: string; purpose?: string }
  }
  identity?: { hash: string; issuer: string }
}
type DealExample = {
  body: {
    state?: string
    parties: Seat[]
    vacancies?: string[]
    terms?: { title?: string }
  }
}

const label = 'Illustrative, test mode, not legal advice.'

function example(slug: string, file: string) {
  const schema = catalog.schemas.find((item) => item.version === '0.4' && item.slug === slug)
  const found = schema?.valid.find((item) => item.name.endsWith(file))
  if (!found) throw new Error(`missing 0.4 ${slug} ${file}`)
  return found
}

function deal(file: string) {
  return JSON.parse(example('deal', file).json) as DealExample
}

function money(value: Money) {
  return `${value.amount_minor} ${value.currency}`
}

function CopyBlock({ label: name, code }: { label: string; code: string }) {
  const [state, setState] = useState('Copy')
  return (
    <div className="mt-4">
      <div className="mb-1 flex items-center justify-between gap-3">
        <p className="eyebrow">{name}</p>
        <button
          type="button"
          className="inline-flex min-h-11 items-center rounded-md border border-line bg-s1 px-3 text-sm font-semibold text-t1"
          aria-label={`Copy ${name}`}
          onClick={() => {
            const mark = (next: string) => {
              setState(next)
              window.setTimeout(() => setState('Copy'), 2000)
            }
            if (!navigator.clipboard) {
              mark('Copy failed')
              return
            }
            navigator.clipboard.writeText(code).then(() => mark('Copied'), () => mark('Copy failed'))
          }}
        >
          {state}
        </button>
      </div>
      <pre tabIndex={0} className="max-w-full overflow-x-auto rounded-xl border border-line bg-s1 p-4 font-mono text-[13px] leading-6 text-t1"><code>{code}</code></pre>
    </div>
  )
}

function Seats({ parties }: { parties: Seat[] }) {
  return (
    <div tabIndex={0} className="mt-3 max-w-full overflow-x-auto">
      <table className="w-full border-collapse text-left text-sm">
        <caption className="sr-only">Seats in this example</caption>
        <thead>
          <tr className="border-b border-line-strong">
            <th scope="col" className="py-2 pr-3 font-semibold">Party</th>
            <th scope="col" className="py-2 pr-3 font-semibold">Role</th>
            <th scope="col" className="py-2 pr-3 font-semibold">Required</th>
            <th scope="col" className="py-2 pr-3 font-semibold">Signed</th>
            <th scope="col" className="py-2 font-semibold">Terms</th>
          </tr>
        </thead>
        <tbody>
          {parties.map((seat) => (
            <tr key={seat.id} className="border-b border-line align-top">
              <th scope="row" className="py-2 pr-3 font-mono text-[13px] font-semibold">{seat.id}</th>
              <td className="py-2 pr-3">{seat.role}</td>
              <td className="py-2 pr-3">{seat.required ? 'Required' : 'Optional'}</td>
              <td className="py-2 pr-3">{seat.signed ? 'Signed' : 'Not signed'}</td>
              <td className="py-2 text-t2">{seat.terms ? JSON.stringify(seat.terms) : 'No terms on this seat.'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function ManyPage() {
  const source = useSource()
  const chain = deal('referral-chain.json')
  const ring = deal('link-swap-ring.json')
  const desc = chain.body.parties.map((seat) => `${seat.role} ${seat.terms?.percent} percent, required, signed`).join('. ')
  return (
    <>
      <p className="eyebrow">ORP 0.4</p>
      <h1 className="h-display mt-3 text-[2rem] leading-tight sm:text-4xl">Many-to-many deals</h1>
      <p className="mt-4 text-lg leading-8">ORP 0.4 adds seats. A valid 0.3 message stays valid. A plain 0.3 deal is still two parties. {notSpoken04} {noMoney}</p>
      <p className="mt-3 text-sm text-t2">{label}</p>
      <h2 className="h-display mt-10 text-2xl">Three parties</h2>
      <p className="mt-3 text-[16px] leading-7 text-t2">The referral example is three required seats. The deal state in the file is {chain.body.state}. All three are signed.</p>
      <svg viewBox="0 0 720 180" role="img" aria-labelledby="chain-title chain-desc" className="mt-4 aspect-[720/180] h-auto w-full max-w-3xl">
        <title id="chain-title">Three-party referral chain</title>
        <desc id="chain-desc">{desc}</desc>
        {chain.body.parties.map((seat, index) => {
          const x = 24 + index * 240
          return (
            <g key={seat.id}>
              {index < chain.body.parties.length - 1 ? <line x1={x + 176} y1={78} x2={x + 220} y2={78} stroke="currentColor" strokeWidth="2" /> : null}
              <rect x={x} y={28} width="176" height="100" rx="16" fill="rgb(253 253 247)" stroke="rgb(35 39 32)" strokeWidth="2" />
              <text x={x + 88} y={62} textAnchor="middle" fill="rgb(35 39 32)" fontSize="16" fontWeight="700">{seat.role}</text>
              <text x={x + 88} y={88} textAnchor="middle" fill="rgb(92 98 87)" fontSize="14">{seat.terms?.percent} percent</text>
              <text x={x + 88} y={110} textAnchor="middle" fill="rgb(92 98 87)" fontSize="13">required, signed</text>
            </g>
          )
        })}
      </svg>
      <p className="mt-4 text-[16px] leading-7 text-t2">A four-party ring in the files is four member seats at {ring.body.parties.map((seat) => seat.terms?.percent).join(', ')} percent. Each is required and signed. The state is {ring.body.state}.</p>
      <BadSplit />
      <p className="mt-3 text-[16px] leading-7 text-t2"><a className="underline underline-offset-4" href={withSource('/protocol/quickstart/#five-party', source)}>The five-seat walkthrough</a> is on the quickstart. <a className="underline underline-offset-4" href={withSource('/protocol/spec/0.4/deal/', source)}>The 0.4 deal page</a> is generated from the schema.</p>
    </>
  )
}

export function ThreePartyExample() {
  const found = example('deal', 'referral-chain.json')
  const parsed = JSON.parse(found.json) as DealExample
  return (
    <section className="mt-10" id="three-party">
      <h2 className="h-display text-2xl">A 3-party deal</h2>
      <p className="mt-3 text-sm text-t2">{label} {notSpoken04}</p>
      <p className="mt-3 text-[16px] leading-7 text-t2">Title in the file: {parsed.body.terms?.title}. State: {parsed.body.state}. The required seats that have signed are {parsed.body.parties.filter((seat) => seat.required && seat.signed).map((seat) => seat.role).join(', ')}.</p>
      <Seats parties={parsed.body.parties} />
      <CopyBlock label={found.name} code={found.json} />
    </section>
  )
}

export function FivePartyWalkthrough() {
  const found = example('deal', 'five-party-home-sale.json')
  const parsed = JSON.parse(found.json) as DealExample
  const leftFile = example('deal', 'mediator-left.json')
  const left = JSON.parse(leftFile.json) as DealExample
  const service = parsed.body.parties.find((seat) => seat.role === 'service')
  const signer = parsed.body.parties.find((seat) => seat.role === 'signer')
  const facilitator = parsed.body.parties.find((seat) => seat.role === 'facilitator')
  const deposit = service?.terms?.deposit
  return (
    <section className="mt-10" id="five-party">
      <h2 className="h-display text-2xl">A 5-party home sale</h2>
      <p className="mt-3 text-sm text-t2">{label} Title in the file: {parsed.body.terms?.title}.</p>
      <p className="mt-3 text-[16px] leading-7 text-t2">State in the file: {parsed.body.state}. {noMoney} The service seat is optional and not signed. Its amount is {service?.terms?.amount ? money(service.terms.amount) : 'missing'} minor units, per {service?.terms?.per}, hours {service?.terms?.hours}. That is EUR 40 per hour. The deposit is {deposit ? money(deposit.amount) : 'missing'} minor units, EUR 20, holder {deposit?.holder}, purpose {deposit?.purpose}. The holder is outside Offer Rails.</p>
      <p className="mt-3 text-[16px] leading-7 text-t2">The facilitator identity is a hash and an issuer ({facilitator?.identity?.issuer}). There is no REA credential-type field. The signer seat ({signer?.id}) is required and signed. It is not flagged human-backed, and the seat has no terms. Party terms have no payer field. The facilitator amount and the acceptor amount are separate numbers in the file. The file does not say who pays whom.</p>
      <p className="mt-3 text-[16px] leading-7 text-t2">Required seats that have signed: {parsed.body.parties.filter((seat) => seat.required && seat.signed).map((seat) => `${seat.role} (${seat.id})`).join('; ')}.</p>
      <Seats parties={parsed.body.parties} />
      <CopyBlock label={found.name} code={found.json} />
      <h3 className="h-display mt-8 text-xl">When the facilitator leaves</h3>
      <p className="mt-3 text-[16px] leading-7 text-t2">The leave example state is {left.body.state}. vacancies is {left.body.vacancies?.join(', ')}. The deal stays pending until the required replacement seated for that vacancy signs. An original seat of the same role does not clear it.</p>
      <Seats parties={left.body.parties} />
      <CopyBlock label={leftFile.name} code={leftFile.json} />
    </section>
  )
}

function BadSplit() {
  const found = example('offer', 'bad-split.json')
  const parsed = JSON.parse(found.json) as DealExample
  const percents = parsed.body.parties.map((seat) => seat.terms?.percent).join(' and ')
  return (
    <p className="mt-3 text-[16px] leading-7 text-t2">The offer file {found.name} has percents {percents}. The JSON schema accepts it, so the 0.4 offer page lists it with the valid examples. The reference host rejects that split.</p>
  )
}

export function StartPrompt() {
  return (
    <section className="mt-10">
      <h2 className="h-display text-2xl">One prompt</h2>
      <CopyBlock label="Three requests to start" code="Three requests to start: join, send the offer with its seats, then each required party accepts." />
    </section>
  )
}
