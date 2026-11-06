// PreToolUse on Agent|Task: refuse to start a subagent on a brief that is missing what
// grid-brief-writer writes (the frontmatter keys and the nine sections, BRIEF_KEYS and
// BRIEF_SECTIONS in lib.mjs). The brief is found in the prompt by briefInPrompt, which only
// reads a file inside a real task folder of the projects folder (projectsRoot). Every other case
// allows silently: no brief named, a brief outside it, no GRID_PROJECTS or GRID_HOME, bad input, a
// file that is gone or cannot be read. Only a brief that was read and found incomplete (no
// frontmatter block included) denies.
// The same sections are required of every role: the writer leaves Outcome and Verdict empty
// but present, so a reviewer's brief needs no exception here (REVIEWER_ROLES matters to the
// checks that read a finished brief).
// A call that is not denied is also remembered per session (rememberDispatch), so SubagentStop can
// name its brief when the subagent's own transcript is not on disk.
import { main, readInput, str, frontmatter, readText, briefInPrompt, rememberDispatch, projectsRoot, BRIEF_KEYS, BRIEF_SECTIONS } from './lib.mjs';

main(async () => {
  const input = await readInput();
  const projects = projectsRoot(process.env, str(input?.cwd));
  if (!input || !projects) return;
  if (!['Agent', 'Task'].includes(input.tool_name)) return;
  const toolInput = input.tool_input && typeof input.tool_input === 'object' ? input.tool_input : {};
  const found = briefInPrompt(toolInput.prompt, str(input.cwd), projects);
  const text = found ? readText(found.file) : null;
  const problems = text === null ? [] : lint(text);
  if (!problems.length) {
    try { rememberDispatch(input, found); } catch { /* the call goes ahead without a note */ }
    return;
  }
  const reason = `The brief subagents/${found.brief}.md is not complete: ${problems.join('; ')}. `
    + 'Have grid-brief-writer write or fix the brief, then start the subagent again.';
  return { stdout: JSON.stringify({ hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: 'deny', permissionDecisionReason: reason } }) };
});

// What is missing from the brief text, one phrase per part.
function lint(text) {
  const fm = frontmatter(text);
  if (!fm) return ['it has no frontmatter block'];
  const out = [];
  for (const k of BRIEF_KEYS) if (!fm.data[k]) out.push('frontmatter field `' + k + '` is missing or empty');
  const headings = new Set(fm.body.split(/\r?\n/).map((l) => /^##\s+(.+?)\s*$/.exec(l)?.[1]).filter(Boolean));
  for (const s of BRIEF_SECTIONS) if (!headings.has(s)) out.push('section `## ' + s + '` is missing');
  return out;
}
