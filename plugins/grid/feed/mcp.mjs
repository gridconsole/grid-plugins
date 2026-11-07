#!/usr/bin/env node
// The grid-feed MCP server: stdio, JSON-RPC 2.0, one message per line. Tools: feed_post, feed_read, feed_state, feed_types
// (feed/lib.mjs holds what they do). The task is resolved inside a tool call, so initialize and tools/list need no Grid environment.
// Nothing but JSON-RPC goes to stdout. Environment of the session: GRID_PROJECTS or GRID_HOME, GRID_PROJECT, GRID_MEMBER,
// GRID_HOST, GRID_ROLE (default worker), GRID_WORKER.
import { TOOLS, callTool } from './lib.mjs';

const PROTOCOLS = ['2025-06-18', '2025-03-26', '2024-11-05'];
const NAME = 'grid-feed';
const VERSION = '0.1.0';

const reply = (id, result) => ({ jsonrpc: '2.0', id, result });
const error = (id, code, message) => ({ jsonrpc: '2.0', id, error: { code, message } });

// The response to one parsed message, or null when it is a notification.
function handle(msg) {
  if (msg === null || typeof msg !== 'object' || Array.isArray(msg) || typeof msg.method !== 'string') return error(msg?.id ?? null, -32600, 'Invalid request');
  if (msg.id === undefined) return null;
  const { id, params } = msg;
  switch (msg.method) {
    case 'initialize':
      return reply(id, {
        protocolVersion: PROTOCOLS.includes(params?.protocolVersion) ? params.protocolVersion : PROTOCOLS[0],
        capabilities: { tools: {} },
        serverInfo: { name: NAME, version: VERSION },
      });
    case 'ping': return reply(id, {});
    case 'tools/list': return reply(id, { tools: TOOLS });
    case 'tools/call': {
      if (typeof params?.name !== 'string' || !TOOLS.some((t) => t.name === params.name)) return error(id, -32602, `Unknown tool: ${params?.name}`);
      const result = callTool(params.name, params.arguments ?? {}, { env: process.env, cwd: process.cwd() });
      return reply(id, { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }], ...(result.ok ? {} : { isError: true }) });
    }
    default: return error(id, -32601, `Method not found: ${msg.method}`);
  }
}

const send = (m) => process.stdout.write(`${JSON.stringify(m)}\n`);

function line(text) {
  if (!text.trim()) return;
  let msg;
  try { msg = JSON.parse(text); } catch { send(error(null, -32700, 'Parse error')); return; }
  let out;
  try { out = handle(msg); } catch (e) { out = error(msg?.id ?? null, -32603, `Internal error: ${e?.message ?? e}`); }
  if (out) send(out);
}

let buf = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
  buf += chunk;
  let i;
  while ((i = buf.indexOf('\n')) >= 0) {
    const text = buf.slice(0, i);
    buf = buf.slice(i + 1);
    line(text);
  }
});
process.stdin.on('end', () => { line(buf); buf = ''; });
