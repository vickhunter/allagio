// Run the in-memory reference host, for the conformance suite or to try an
// agent against. The /orp-test harness is on: never expose this publicly.
//
//   PORT=8787 node examples/host.ts
//   ORP_SIGN=1 PORT=8787 node examples/host.ts     sign every host message (Level 3)
import { startHost } from '../src/index.ts';

const { origin, close } = await startHost({ harness: true, sign: process.env.ORP_SIGN === '1', port: Number(process.env.PORT ?? 8787) });
console.log(`ORP reference host on ${origin}  (discovery: ${origin}/.well-known/orp.json)`);
for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.on(signal, () => void close().then(() => process.exit(0)));
}
