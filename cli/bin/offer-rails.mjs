#!/usr/bin/env node
import { main } from '../lib/main.mjs';

main(process.argv.slice(2)).catch((err) => {
  process.stderr.write(`${err && err.message ? err.message : err}\n`);
  process.exit(1);
});
