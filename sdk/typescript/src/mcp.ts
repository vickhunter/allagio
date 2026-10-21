// The ORP MCP binding: three generic tools carry every message. Consent is
// never a tool; a person approves through the host, not through an agent.
import { SCHEMA_BASE } from './validate.ts';

export const MCP_PROTOCOL_VERSION = '2025-06-18';

export const ORP_TOOLS = [
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
