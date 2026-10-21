// The ORP MCP binding. orp_suggest and orp_join need no credentials, so any
// writing agent can offer a deal mid-task. Three generic tools carry every
// message. Consent is never a tool; a person approves through the host.
import { SCHEMA_BASE } from './validate.ts';

export const MCP_PROTOCOL_VERSION = '2025-06-18';

export const ORP_TOOLS = [
  {
    name: 'orp_suggest',
    title: 'Suggest deals for what you are writing',
    description:
      'Call this while you write a post, thread, newsletter or video description for someone. Send the draft or a short summary; get up to five deals they could take now (affiliate, sponsored, cross-sell, swaps, link exchanges, reposts), each with a pitch to show them as is, such as "You can monetize this post ... Want to test it?". No account needed. Leave out names, emails and anything personal; the host does not keep the text.',
    inputSchema: {
      type: 'object',
      additionalProperties: false,
      required: ['text'],
      properties: {
        text: { type: 'string', minLength: 1, maxLength: 4000, description: 'The draft, or a short summary of it.' },
        platform: { type: 'string', description: 'Where it will be published, such as web, x, newsletter, youtube, linkedin.' },
        space_kind: { type: 'string', description: 'Such as web.article, social.post, newsletter.issue.' },
        want: { type: 'array', items: { type: 'string', enum: ['earn', 'grow'] }, description: 'earn: get paid. grow: get readers. Both when absent.' },
        kinds: { type: 'array', items: { type: 'string' } },
        lang: { type: 'string' },
        limit: { type: 'integer', minimum: 1, maximum: 5 },
      },
    },
    annotations: { readOnlyHint: true, openWorldHint: true },
  },
  {
    name: 'orp_join',
    title: 'Join to take a deal',
    description:
      'Get a pseudonymous party for your person with no email, signup or website. Call it once, when they say yes to a suggestion. This MCP session then acts as that party. Every step that binds them waits for their one-tap approval at a url you show them. Give them the claim url; keep the token out of posts and messages.',
    inputSchema: { type: 'object', additionalProperties: false, properties: { agent: { type: 'object', properties: { name: { type: 'string' }, version: { type: 'string' } }, required: ['name'] } } },
    annotations: { readOnlyHint: false, destructiveHint: false, idempotentHint: false, openWorldHint: true },
  },
  {
    name: 'orp_send',
    title: 'Send an ORP message',
    description:
      'Send one Offer Rails Protocol 0.3 message (discover, ask, offer, counter, accept, decline, execute, receipt, revoke, dispute, reputation, hello) and get the reply message. Escalations come back as a deal in state awaiting with a url for the person; never try to approve them yourself.',
    inputSchema: {
      type: 'object',
      additionalProperties: false,
      required: ['message'],
      properties: {
        message: { type: 'object', description: `An ORP 0.3 message, see ${SCHEMA_BASE}message.schema.json` },
      },
    },
    annotations: { destructiveHint: false, idempotentHint: true, openWorldHint: true },
  },
  {
    name: 'orp_inbox',
    title: 'Read the ORP inbox',
    description: 'Read messages for this party, oldest first. Pass the returned next value as after to continue.',
    inputSchema: { type: 'object', additionalProperties: false, properties: { after: { type: 'string', maxLength: 200 } } },
    annotations: { readOnlyHint: true },
  },
  {
    name: 'orp_deal',
    title: 'Read one ORP deal',
    description: "Get the current snapshot of a deal this party is in, including its tracking links. The tracking link works anywhere: paste it in email, a post, a video description or a blog.",
    inputSchema: { type: 'object', additionalProperties: false, required: ['deal'], properties: { deal: { type: 'string', pattern: '^[A-Za-z0-9_-]{3,64}$' } } },
    annotations: { readOnlyHint: true },
  },
] as const;

export type OrpToolName = (typeof ORP_TOOLS)[number]['name'];

export function orpCapability(discoveryUrl: string) {
  return { orp: { versions: ['0.3'], discovery: discoveryUrl } };
}
