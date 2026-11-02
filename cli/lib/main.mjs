import readline from 'node:readline';
import { digestCommand, killCommand, listCommand, loginCommand, scanCommand, sessionCommand, statusCommand, undoCommand } from './commands.mjs';
import { mcpCommand } from './mcp.mjs';
import { SESSION_ONLY } from './session.mjs';

export function defaultIO() {
  return {
    env: process.env,
    cwd: process.cwd(),
    fetch: globalThis.fetch.bind(globalThis),
    stdout(chunk) { process.stdout.write(chunk); },
    stderr(chunk) { process.stderr.write(chunk); },
    sleep(ms) { return new Promise((resolve) => setTimeout(resolve, ms)); },
    readLine() {
      return new Promise((resolve) => {
        let settled = false;
        const rl = readline.createInterface({ input: process.stdin, terminal: false });
        const done = (value) => {
          if (settled) return;
          settled = true;
          rl.close();
          resolve(value);
        };
        rl.once('line', (line) => done(line));
        rl.once('close', () => done(''));
      });
    },
  };
}

export function parseArgs(argv) {
  const positionals = [];
  const flags = {};
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg.startsWith('--')) {
      const key = arg.slice(2);
      const next = argv[i + 1];
      if (next === undefined || next.startsWith('--')) flags[key] = true;
      else {
        flags[key] = next;
        i += 1;
      }
    } else positionals.push(arg);
  }
  return { positionals, flags };
}

export async function run(argv, io = defaultIO()) {
  const { positionals, flags } = parseArgs(argv);
  const command = positionals[0] || 'help';
  const session = SESSION_ONLY.find((item) => item.command === command);
  if (session) return sessionCommand(session.label, io);
  if (command === 'login') return loginCommand(flags, io);
  if (command === 'scan') return scanCommand(positionals[1] || '.', io);
  if (command === 'list') return listCommand(io);
  if (command === 'status') return statusCommand(io);
  if (command === 'digest') return digestCommand(io);
  if (command === 'undo') return undoCommand(io);
  if (command === 'kill') return killCommand(io);
  if (command === 'mcp') return mcpCommand(io);
  io.stdout(`allagio commands: login, scan [path|url], list, status, digest, undo, kill, mcp\n`);
  io.stdout(`Session-only commands print a dashboard sign-in link: ${SESSION_ONLY.map((item) => item.command).join(', ')}\n`);
  return { exitCode: command === 'help' || flags.help ? 0 : 1 };
}

export async function main(argv) {
  const result = await run(argv);
  process.exit(result.exitCode);
}
