// A writing assistant that offers a deal mid-task. It drafts a post, asks the
// host which deals fit (no account), pitches the best one to its person, joins
// with nothing personal once they say yes, and after their one-tap approval
// puts the tracking link and the disclosure into the post.
//
//   node examples/writer.ts                                 demo host in this process
//   ORP_HOST=https://hub.example node examples/writer.ts    stops at the approval url
import { OrpClient, startHost, suggest, termsHash, type MessageByType, type Terms } from '../src/index.ts';

const DRAFT = process.env.ORP_DRAFT ?? 'Five note-taking apps I use to run a one-person software business, and how I keep my notes in sync.';

async function write(origin: string, approve?: (escalation: string) => Promise<void>) {
  console.log(`draft: ${DRAFT}\n`);
  const { items } = await suggest(origin, { text: DRAFT, platform: 'web', space_kind: 'web.article', want: ['earn'] });
  const best = items.find((item) => item.offer);
  if (!best?.offer) {
    console.log('no deal fits this post, so the assistant says nothing');
    return undefined;
  }
  console.log(`assistant: ${best.pitch}`);
  console.log('person: yes\n');

  const { client, joined } = await OrpClient.join(origin, { agent: { name: 'writer-assistant', version: '0.3.0' } });
  console.log(`joined as ${client.party} with no email or signup; the person can claim it later at ${joined.claim}`);
  let deal = (await client.send(client.build('accept', { offer: best.offer.id, terms_hash: best.offer.terms_hash }))) as MessageByType['deal'];
  if (deal.body.state === 'awaiting') {
    console.log(`assistant: Tap to approve: ${deal.body.waiting?.url}`);
    if (!approve) return undefined;
    await approve(deal.body.waiting!.escalation!);
    deal = await client.deal(deal.deal!);
  }
  const link = deal.body.tracking?.[0]?.link;
  if (deal.body.state !== 'active' || !link) {
    console.log(`the deal is ${deal.body.state}`);
    return undefined;
  }
  const label = best.offer.terms.disclosure?.label ?? 'Partner link';
  console.log(`\npublished:\n  ${DRAFT}\n  ${label}: ${link}\n`);
  return { client, deal: deal.deal!, link };
}

if (process.env.ORP_HOST) {
  await write(process.env.ORP_HOST);
} else {
  const { host, origin, close } = await startHost({ harness: true });
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

    const published = await write(origin, async (escalation) => {
      console.log('person: (taps Approve with a passkey)');
      host.decideEscalation(escalation, 'approved', 'passkey');
    });
    if (published) {
      const readers = [
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36',
        'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148',
        'Twitterbot/1.0',
      ];
      for (const agent of readers) await fetch(published.link, { redirect: 'manual', headers: { 'user-agent': agent } });
      const outcome = await published.client.outcome(published.deal);
      console.log(`readers so far, aggregate only: ${JSON.stringify(outcome.body.rows[0]?.clicks)}`);
    }
  } finally {
    await close();
  }
}
