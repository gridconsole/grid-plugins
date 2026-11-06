// SessionStart, PostToolUse on Agent|Task and SubagentStop: note what the hooks can see of a
// session's token use, one JSON line per event, in CLAUDE_PLUGIN_DATA/sessions/<session_id>.usage.
// core/usage.mjs reads that file (and the transcripts it points at) for GET /api/usage. No hook
// input carries a cost, so this records transcript paths and the Agent tool's own totals.
//   SessionStart  { event, time, source, transcript }
//   PostToolUse   { event, time, tool_use_id, type, brief, agent_id, response, response_keys }
//   SubagentStop  { event, time, agent_id, agent_type, transcript }
// `response` keeps only the numbers and short words of the Agent tool_response that the reader
// uses; `response_keys` is its top-level key names (keys only), which is how a version change in
// the tool_response shows. Prompt, description and result text are never copied. Paths are model-
// or version-supplied, so they are stored as plain strings and the reader decides whether to open
// them. A hook that cannot record exits 0 and the session goes on (main swallows every error).
import { main, readInput, str, agentName, briefInPrompt, appendUsage, trimUsage, safeNum, safeWord, projectsRoot } from './lib.mjs';

const USAGE_KEYS = ['input_tokens', 'output_tokens', 'cache_read_input_tokens', 'cache_creation_input_tokens'];

main(async () => {
  const input = await readInput();
  if (!input) return;
  const entry = record(input);
  if (!entry) return;
  if (entry.event === 'SessionStart') trimUsage(input);
  appendUsage(input, entry);
});

const filePath = (v) => (typeof v === 'string' && v.length <= 1000 && !/[\u0000-\u001f]/.test(v) ? v : null);

function record(input) {
  const time = new Date().toISOString();
  switch (input.hook_event_name) {
    case 'SessionStart':
      return { event: 'SessionStart', time, source: safeWord(input.source), transcript: filePath(input.transcript_path) };
    case 'SubagentStop':
      return { event: 'SubagentStop', time, agent_id: safeWord(input.agent_id), agent_type: agentName(input.agent_type), transcript: filePath(input.agent_transcript_path) };
    case 'PostToolUse': {
      if (!['Agent', 'Task'].includes(input.tool_name)) return null;
      const toolInput = obj(input.tool_input);
      const res = obj(input.tool_response);
      const found = briefInPrompt(toolInput.prompt, str(input.cwd), projectsRoot(process.env, str(input.cwd)));
      return {
        event: 'PostToolUse',
        time,
        tool_use_id: safeWord(input.tool_use_id),
        type: agentName(toolInput.subagent_type || 'general-purpose'),
        brief: found?.brief ?? null,
        agent_id: safeWord(res.agentId),
        response: response(res),
        response_keys: Object.keys(res).map(safeWord).filter(Boolean).slice(0, 40),
      };
    }
    default:
      return null;
  }
}

const obj = (v) => (v && typeof v === 'object' && !Array.isArray(v) ? v : {});

// The whitelisted part of an Agent tool_response; a field that is absent or of another type is left out.
function response(res) {
  const out = {};
  const status = safeWord(res.status);
  const model = safeWord(res.resolvedModel);
  if (status) out.status = status;
  if (model) out.resolvedModel = model;
  for (const k of ['totalTokens', 'totalDurationMs']) if (safeNum(res[k]) !== null) out[k] = res[k];
  const usage = {};
  for (const k of USAGE_KEYS) if (safeNum(obj(res.usage)[k]) !== null) usage[k] = res.usage[k];
  if (Object.keys(usage).length) out.usage = usage;
  return out;
}
