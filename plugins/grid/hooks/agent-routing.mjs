// SessionStart: tell the main session to delegate to grid's own agents instead of general-purpose. Installing the
// plugin only lists the agents; without this line a machine with no personal CLAUDE.md routing falls back to
// general-purpose. The list is read from agents/*.md so it follows the agents shipped. Never blocks.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { main } from './lib.mjs';

const agentsDir = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'agents');

// The first sentence of an agent's frontmatter description, or '' when it has none.
function firstSentence(text) {
  const m = /^---\n([\s\S]*?)\n---/.exec(text);
  const line = m && /^description:\s*(.*)$/m.exec(m[1]);
  if (!line) return '';
  const d = line[1].trim().replace(/^["']|["']$/g, '');
  const end = d.search(/\.(\s|$)/);
  return end === -1 ? d : d.slice(0, end);
}

function routing(dir = agentsDir) {
  let files = [];
  try { files = fs.readdirSync(dir).filter((f) => f.endsWith('.md')).sort(); } catch { return ''; }
  if (files.length === 0) return '';
  const rows = files.map((f) => {
    const what = firstSentence(fs.readFileSync(path.join(dir, f), 'utf8'));
    return `- grid:${f.slice(0, -3)}${what ? `: ${what}` : ''}`;
  });
  return [
    'Subagents: for every delegated job in this session, whether or not it belongs to a Grid task, use the Grid plugin\'s agents',
    '(subagent_type "grid:<name>"). Do not use general-purpose, Explore or Plan for a job one of them covers; their',
    '"Grid task" wording does not limit them to task folders. Pick by the job:',
    '- find where something is in the code, how a flow runs, a codebase map: grid:grid-scout',
    '- facts from docs, source or the web: grid:grid-researcher',
    '- write or change code, docs or files: grid:grid-builder',
    '- run a build, test, lint or read CI logs: grid:grid-runner',
    '- review code or finished work: grid:grid-reviewer',
    '- a design spec before UI is built: grid:grid-designer',
    '- check a page in a browser without changing it: grid:grid-checker; browser work that changes state: grid:grid-operator',
    '- plan a task: grid:grid-planner; write a brief: grid:grid-brief-writer; curate brain proposals: grid:grid-curator',
    'With no brief, put the job in the prompt. Use general-purpose only for a job that fits none of these.',
    'All Grid agents:',
    ...rows,
  ].join('\n');
}

main(() => {
  const context = routing();
  if (!context) return;
  return { stdout: `${JSON.stringify({ hookSpecificOutput: { hookEventName: 'SessionStart', additionalContext: context } })}\n` };
});
