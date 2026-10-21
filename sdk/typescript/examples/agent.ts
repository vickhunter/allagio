// A tiny publisher agent. Its owner set limits once, on the host. Inside them
// the agent finds a standing offer, accepts it, and prints the tracking link.
// The owner pastes that link anywhere a person can click it: an email, an X
// post, a blog. No platform adapter is involved.
//
//   node examples/agent.ts                                    demo host in this process
//   ORP_HOST=https://hub.example ORP_TOKEN=... node examples/agent.ts
import { OrpClient, startHost, termsHash, type Constraints, type MessageByType, type Terms } from '../src/index.ts';

const QUERY = process.env.ORP_QUERY ?? 'note taking app for solo founders';

async function run(origin: string, token: string): Promise<{ deal?: string; links: string[]; client: OrpClient }> {
  const client = await OrpClient.connect(origin, token, { name: 'orp-example-agent', version: '0.3.0' });
  console.log(`connected as ${client.party}`);

  const results = (await client.send(client.build('discover', { want: 'offers', query: QUERY, filters: { kinds: ['affiliate'] }, limit: 5 }))) as MessageByType['results'];
  const best = results.body.items.find((item) => item.offer);
  if (!best?.offer) {
    console.log(`no offers fit (${results.body.empty_reason ?? 'empty'})`);
    return { links: [], client };
  }
  console.log(`best fit: "${best.offer.terms.title}" (fit ${best.fit.score}, reputation ${best.trust?.reputation ?? 'none yet'})`);

  const deal = (await client.send(client.build('accept', { offer: best.offer.id, terms_hash: best.offer.terms_hash }))) as MessageByType['deal'];
  if (deal.body.state === 'awaiting') {
    console.log(`outside the owner's limits (${deal.body.waiting?.reasons?.join(', ')}); the owner decides at ${deal.body.waiting?.url}`);
    return { deal: deal.deal, links: [], client };
  }
  const links = (deal.body.tracking ?? []).map((t) => t.link);
  for (const t of deal.body.tracking ?? []) {
    console.log(`deal ${deal.deal} is ${deal.body.state}. Paste this anywhere (email, X, a blog, video or podcast notes):\n  ${t.link}${t.code ? `  or the code ${t.code}` : ''}`);
  }
  return { deal: deal.deal, links, client };
}

if (process.env.ORP_HOST) {
  if (!process.env.ORP_TOKEN) throw new Error('set ORP_TOKEN to your agent token on that host');
  await run(process.env.ORP_HOST, process.env.ORP_TOKEN);
} else {
  const { host, origin, close } = await startHost();
  try {
    const terms: Terms = {
      title: 'Acme Notes: 30% of first-year revenue',
      kind: 'affiliate',
      categories: ['software.productivity'],
      disclosure: { label: 'Affiliate link', lang: 'en' },
      obligations: [
        { id: 'place', by: 'acceptor', action: 'place', content: { link: true }, destination: 'https://acmenotes.example/pricing', proof: ['link'] },
        { id: 'commission', by: 'offerer', action: 'pay', per: 'conversion', rate_bps: 3000, window: 'P30D', rail: 'stripe' },
      ],
    };
    const maker = host.createParty({ constraints: { steps: { list: 'auto' }, budget: { max_rate_bps: 3000 } } });
    const makerClient = await OrpClient.connect(origin, maker.token, { name: 'demo-maker' });
    await makerClient.send(makerClient.build('offer', { terms, terms_hash: termsHash(terms) }));

    const limits: Constraints = { steps: { accept: 'auto' }, kinds: ['affiliate'], categories: { allow: ['software'], deny: ['gambling'] } };
    const publisher = host.createParty({ constraints: limits });
    const { deal, links, client } = await run(origin, publisher.token);

    if (deal && links[0]) {
      const surfaces = {
        'an email (reader clicks in their mail app)': 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148',
        'X (the card preview bot fetches it first)': 'Twitterbot/1.0',
        'X (a reader clicks the post)': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15',
        'a blog (a reader clicks the post)': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36',
      };
      for (const [surface, agent] of Object.entries(surfaces)) {
        const response = await fetch(links[0], { redirect: 'manual', headers: { 'user-agent': agent } });
        console.log(`click from ${surface}: ${response.status} -> ${response.headers.get('location')}`);
      }
      const outcome = await client.outcome(deal);
      console.log(`aggregate outcome, no personal data: ${JSON.stringify(outcome.body.rows[0]?.clicks)}`);
    }
  } finally {
    await close();
  }
}
