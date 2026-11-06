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
  return `Delegate work by role to the Grid plugin's agents (subagent_type "grid:<name>"), not general-purpose. Use general-purpose only for a one-off that fits none of them.\n${rows.join('\n')}`;
}

main(() => {
  const context = routing();
  if (!context) return;
  return { stdout: `${JSON.stringify({ hookSpecificOutput: { hookEventName: 'SessionStart', additionalContext: context } })}\n` };
});
