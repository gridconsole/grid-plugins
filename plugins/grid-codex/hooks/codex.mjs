// Shared by the Codex adapters. Each adapter reads one Codex hook payload, turns it into the input
// a shared Grid hook script expects (the Claude Code shape), runs that script and turns its verdict
// back into Codex hook output. The shared scripts sit in hooks/grid/ beside this file: Grid copies
// plugins/grid/hooks there when it stages this plugin for an account, so the gates are the same code
// for both agents. No dependencies. Like the shared hooks, an adapter never fails a call: any error
// means no output and exit 0, so Codex goes on as if the hook were not there.
//
// Codex 0.162 hook payloads carry the Claude Code field names (session_id, transcript_path, cwd,
// hook_event_name, tool_name, tool_input, tool_response, tool_use_id, stop_hook_active, agent_id,
// agent_type, agent_transcript_path) plus turn_id; see README.md for the sources. The one part that
// is not fully confirmed (the tool names and tool inputs Codex reports) is kept in claudeTool below.
import { spawn } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));

// The staged copy of plugins/grid/hooks.
export const SHARED = path.join(here, 'grid');

const str = (v) => (typeof v === 'string' ? v : '');
const obj = (v) => (v && typeof v === 'object' && !Array.isArray(v) ? v : {});

// stdin as a JSON object, or null when it is not one (same rules as the shared readInput).
export async function readPayload(stdin = process.stdin) {
  let text = '';
  try {
    stdin.setEncoding('utf8');
    const read = (async () => { for await (const chunk of stdin) text += chunk; })();
    await Promise.race([read, new Promise((resolve) => setTimeout(resolve, 3000).unref())]);
    const v = JSON.parse(text);
    return v && typeof v === 'object' && !Array.isArray(v) ? v : null;
  } catch { return null; }
}

// Run an adapter: `fn` gets the payload and returns { stdout } or nothing. Output is flushed before
// the exit, and the exit is always 0: a verdict travels as JSON on stdout, never as an exit code.
export async function main(fn) {
  let out = {};
  try {
    const payload = await readPayload();
    out = (payload && (await fn(payload))) ?? {};
  } catch { out = {}; }
  process.stdout.on('error', () => {});
  await new Promise((resolve) => (out.stdout ? process.stdout.write(out.stdout, () => resolve()) : resolve()));
  process.exit(0);
}

// The plugin data folder the shared scripts keep their per-session pointers in. Codex sets
// PLUGIN_DATA and, for plugins written for Claude Code, CLAUDE_PLUGIN_DATA. Without either the
// shared scripts get none and skip their pointers (pointerFile gives null), never a made-up folder.
export function pluginData(env = process.env) {
  return env.CLAUDE_PLUGIN_DATA || env.PLUGIN_DATA || '';
}

// The environment a shared script runs with: the hook's own, with the plugin root and data folder
// the shared scripts read.
export function sharedEnv(env = process.env) {
  const out = { ...env, CLAUDE_PLUGIN_ROOT: path.dirname(here) };
  const data = pluginData(env);
  if (data) out.CLAUDE_PLUGIN_DATA = data;
  else delete out.CLAUDE_PLUGIN_DATA;
  return out;
}

// Run a shared script with `input` on stdin: { code, stdout, stderr }. A script that cannot start,
// or runs past `timeoutMs`, gives code 0 and no output, which every adapter reads as "allow".
export function runShared(script, input, { env = process.env, shared = SHARED, timeoutMs = 9000 } = {}) {
  return new Promise((resolve) => {
    let stdout = '';
    let stderr = '';
    let done = false;
    const finish = (r) => { if (!done) { done = true; resolve(r); } };
    let child;
    try {
      child = spawn(process.execPath, [path.join(shared, script)], { env: sharedEnv(env), stdio: ['pipe', 'pipe', 'pipe'] });
    } catch { return finish({ code: 0, stdout: '', stderr: '' }); }
    const timer = setTimeout(() => { child.kill('SIGKILL'); finish({ code: 0, stdout: '', stderr: '' }); }, timeoutMs);
    child.stdout.setEncoding('utf8').on('data', (s) => { stdout += s; });
    child.stderr.setEncoding('utf8').on('data', (s) => { stderr += s; });
    child.on('error', () => { clearTimeout(timer); finish({ code: 0, stdout: '', stderr: '' }); });
    child.on('close', (code) => { clearTimeout(timer); finish({ code: code ?? 0, stdout, stderr }); });
    child.stdin.on('error', () => {});
    child.stdin.end(JSON.stringify(input));
  });
}

// The fields every event shares, copied as they are.
const COMMON = ['session_id', 'transcript_path', 'cwd', 'permission_mode', 'model', 'turn_id', 'agent_id', 'agent_type'];

export function common(payload, event) {
  const out = { hook_event_name: event };
  for (const k of COMMON) if (payload[k] !== undefined && payload[k] !== null) out[k] = payload[k];
  return out;
}

// Stop and SubagentStop: the payload is already in the shape stop-check reads.
export function stopInput(payload) {
  const event = payload.hook_event_name === 'SubagentStop' ? 'SubagentStop' : 'Stop';
  const out = common(payload, event);
  out.stop_hook_active = payload.stop_hook_active === true;
  for (const k of ['last_assistant_message', 'agent_transcript_path']) if (typeof payload[k] === 'string') out[k] = payload[k];
  return out;
}

// stop-check blocks with exit 2 and the reason on stderr. Codex takes a block as
// {"decision":"block","reason":...} on stdout, the reason going back to the model.
export function stopVerdict(result) {
  if (result.code !== 2 || !result.stderr.trim()) return undefined;
  return { stdout: `${JSON.stringify({ decision: 'block', reason: result.stderr.trim() })}\n` };
}

// PreToolUse and PostToolUse: the Codex tool as the tool the shared scripts know, or null for a
// tool they do not handle. UNCONFIRMED (see README.md): the names and inputs Codex reports for its
// shell and its subagent spawn tool. This is the only place that knows them.
//   - shell: "Bash" with { command: string } is what exec_command reports. The other names are the
//     model-facing ones, in case another shell handler reports its own; an argv list is joined with
//     spaces, which is all tasksInCommand needs (the task folder paths it names).
//   - subagent spawn: "spawn_agent" (alias "Agent") with the spawn arguments: message, items,
//     agent_type, model, reasoning_effort, fork_context. The message, or the text of its items, is
//     the prompt and agent_type the subagent type.
export const SHELL_TOOLS = ['Bash', 'shell', 'local_shell', 'container.exec', 'exec_command'];
export const SPAWN_TOOLS = ['spawn_agent', 'Agent'];

export function claudeTool(name, input) {
  const t = obj(input);
  if (SHELL_TOOLS.includes(name)) {
    const raw = t.command ?? t.cmd;
    const command = Array.isArray(raw) ? raw.filter((s) => typeof s === 'string').join(' ') : str(raw);
    if (!command) return null;
    const out = { command };
    if (t.run_in_background === true) out.run_in_background = true;
    return { tool_name: 'Bash', tool_input: out };
  }
  if (SPAWN_TOOLS.includes(name)) {
    const items = Array.isArray(t.items) ? t.items : [];
    const text = items.map((i) => str(obj(i).text)).filter(Boolean);
    const prompt = [str(t.message), ...text].filter(Boolean).join('\n');
    return { tool_name: 'Agent', tool_input: { prompt, subagent_type: str(t.agent_type) || 'default', description: '' } };
  }
  return null;
}

// A tool event in the shape the shared scripts read, or null for a tool they do not handle.
export function toolInput(payload) {
  const tool = claudeTool(str(payload.tool_name), payload.tool_input);
  if (!tool) return null;
  const event = payload.hook_event_name === 'PostToolUse' ? 'PostToolUse' : 'PreToolUse';
  const out = { ...common(payload, event), ...tool, tool_use_id: str(payload.tool_use_id) };
  if (event === 'PostToolUse' && payload.tool_response !== undefined) out.tool_response = payload.tool_response;
  return out;
}

// brief-lint denies with hookSpecificOutput.permissionDecision "deny". Codex reads the same object;
// only a deny is passed on (Codex does not act on "allow" or "ask" from a hook).
export function preToolVerdict(result) {
  let out;
  try { out = JSON.parse(result.stdout); } catch { return undefined; }
  const h = obj(obj(out).hookSpecificOutput);
  if (h.permissionDecision !== 'deny') return undefined;
  const reason = str(h.permissionDecisionReason) || 'Grid refused this call.';
  return { stdout: `${JSON.stringify({ hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: 'deny', permissionDecisionReason: reason } })}\n` };
}
