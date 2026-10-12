import { digestCommand, killCommand, listCommand, loginCommand, scanCommand, statusCommand, undoCommand } from './commands.mjs';
import { SESSION_ONLY, sessionOnlyResult } from './session.mjs';

const VERSION = '0.1.0';

function tool(name, description, properties, required = []) {
  return {
    name,
    description,
    inputSchema: { type: 'object', properties, required, additionalProperties: false },
  };
}

const str = (description) => ({ type: 'string', description });
const bool = (description) => ({ type: 'boolean', description });

export function toolDefs() {
  return [
    tool('login', 'Device-code login, or an agent-only account when agent is true. Stores a project key locally.', {
      agent: bool('Create an agent-only account instead of a device-code login.'),
      role: str('publisher or maker, used with agent.'),
      name: str('Account name, used with agent.'),
      email: str('Account email, used with agent.'),
      domain: str('Publisher domain, used with agent.'),
    }),
    tool('scan', 'Local readiness scan. Without confirm, nothing is uploaded.', {
      path: str('Directory to scan. Ignored when url is set.'),
      url: str('One https URL, or http on localhost. Links inside the page are not fetched.'),
      confirm: bool('Upload the listing card after printing it.'),
    }),
    tool('list', 'Submit the scoped draft offer and print the status the server returns. Prints listed when the server lists it. Prints in review, check with the server review_reason, the message, and the next step when a check fails. Prints mode and mode_preselected from the server and does not set a mode.', {}),
    tool('status', 'Show the stored project key scope and the offer or listing state.', {}),
    tool('digest', 'Show what went live for this project key, with an undo token.', {}),
    tool('undo', 'Undo the latest digest item for this project key.', {}),
    tool('kill', 'Turn the kill switch on for this project.', {}),
    ...SESSION_ONLY.map((item) => tool(item.tool, `Session-only. ${item.label}. Returns a sign-in link and does not call the API.`, {
      placement_id: str('Optional placement id. Ignored; restore stays on the dashboard.'),
    })),
  ];
}

function capture(io) {
  let text = '';
  return {
    ...io,
    stdout(chunk) {
      text += chunk;
      io.stderr(chunk);
    },
    text() {
      return text;
    },
  };
}

async function callTool(name, args, io) {
  const session = SESSION_ONLY.find((item) => item.tool === name);
  if (session) {
    const result = sessionOnlyResult(session.label, io.env);
    return { isError: false, text: JSON.stringify(result, null, 2) };
  }
  const out = capture(io);
  let result;
  if (name === 'login') {
    result = await loginCommand({
      agent: args.agent === true,
      role: args.role,
      name: args.name,
      email: args.email,
      domain: args.domain,
    }, out);
  } else if (name === 'scan') {
    const confirm = args.confirm === true ? true : false;
    result = await scanCommand(args.url || args.path || '.', out, { confirm });
  } else if (name === 'list') result = await listCommand(out);
  else if (name === 'status') result = await statusCommand(out);
  else if (name === 'digest') result = await digestCommand(out);
  else if (name === 'undo') result = await undoCommand(out);
  else if (name === 'kill') result = await killCommand(out);
  else return { isError: true, text: `unknown tool ${name}` };
  return { isError: result.exitCode !== 0, text: out.text() };
}

export async function handleMcpMessage(raw, io) {
  let req;
  try {
    req = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!req || req.id === undefined || req.id === null) return null;
  const id = req.id;
  const ok = (result) => ({ jsonrpc: '2.0', id, result });
  const fail = (code, message) => ({ jsonrpc: '2.0', id, error: { code, message } });
  try {
    if (req.method === 'initialize') {
      return ok({
        protocolVersion: '2025-06-18',
        capabilities: { tools: {} },
        serverInfo: { name: 'offer-rails', version: VERSION },
        instructions: 'restore_placement does not call the API. It returns a sign in again link to the dashboard. The other session-only tools do the same.',
      });
    }
    if (req.method === 'ping') return ok({});
    if (req.method === 'tools/list') return ok({ tools: toolDefs() });
    if (req.method === 'tools/call') {
      const params = req.params || {};
      const called = await callTool(params.name, params.arguments || {}, io);
      return ok({
        content: [{ type: 'text', text: called.text }],
        isError: called.isError,
      });
    }
    return fail(-32601, 'method not found');
  } catch (err) {
    return fail(-32603, err && err.message ? err.message : 'failed');
  }
}

export function startMcp(input, output, io) {
  let buf = Buffer.alloc(0);
  let mode = null;
  const pending = [];
  let draining = false;

  const write = (response) => {
    if (!response) return;
    const encoded = JSON.stringify(response);
    if (mode === 'length') {
      output.write(`Content-Length: ${Buffer.byteLength(encoded)}\r\n\r\n${encoded}`);
      return;
    }
    output.write(`${encoded}\n`);
  };

  const drain = async () => {
    if (draining) return;
    draining = true;
    while (pending.length) {
      const line = pending.shift();
      write(await handleMcpMessage(line, io));
    }
    draining = false;
  };

  const take = () => {
    while (buf.length) {
      if (mode === null) mode = buf[0] === 0x7b ? 'line' : 'length';
      if (mode === 'line') {
        const nl = buf.indexOf(0x0a);
        if (nl < 0) return;
        const line = buf.subarray(0, nl).toString('utf8').replace(/\r$/, '');
        buf = buf.subarray(nl + 1);
        if (line.trim()) pending.push(line);
        continue;
      }
      const sep = buf.indexOf('\r\n\r\n');
      if (sep < 0) return;
      const header = buf.subarray(0, sep).toString('utf8');
      const match = /content-length:\s*(\d+)/i.exec(header);
      if (!match) {
        buf = Buffer.alloc(0);
        return;
      }
      const len = Number(match[1]);
      const start = sep + 4;
      if (buf.length < start + len) return;
      pending.push(buf.subarray(start, start + len).toString('utf8'));
      buf = buf.subarray(start + len);
    }
  };

  input.on('data', (chunk) => {
    buf = Buffer.concat([buf, Buffer.from(chunk)]);
    take();
    drain().catch((err) => {
      io.stderr(`${err && err.message ? err.message : err}\n`);
    });
  });
}

export async function mcpCommand(io) {
  startMcp(process.stdin, process.stdout, io);
  await new Promise((resolve) => {
    process.stdin.on('end', resolve);
    process.stdin.on('close', resolve);
  });
  return { exitCode: 0 };
}
