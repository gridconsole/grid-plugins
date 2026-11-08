// PreToolUse on Write|Edit|MultiEdit|NotebookEdit (and on Bash for the recall gate only, see recallGate): no session builds in the repo before a person
// accepts the plan. Grid's `plan accepted` line is the gate: the planner plans in normal mode, and
// this keeps its file edits out of the repo until then. Every rule runs in every permission mode.
// - A task is held while history.md has no `plan accepted` line of Grid's own shape ("- <time> · plan
//   accepted · ..."; a name a hook logged in the detail's place does not count), it is not closed,
//   paused or pausing, and its step is the plan approval (the first approve step whose name has "plan",
//   as gateEvent in core) or one before it, or task.md cannot be read for sure: missing, no step, an
//   odd key, a workflow item that is not a {step, kind} pair, a step named twice (lib's parser is
//   core's, kept equal by tests/hooks/frontmatter-parity.test.mjs). While any task of the agent is
//   held (the one cwd is in, and every one this agent wrote into, which its list never drops while
//   held), the agent writes only in the Grid projects folder. The hold is per agent: the main thread is held by
//   the tasks it wrote into itself, a subagent (agent_id) by the ones it wrote into itself and the main thread's,
//   so a worker started before the plan is accepted is held, while a subagent working on a task whose plan is
//   not accepted does not stop the other agents of the session.
// - A grid-planner writes only in the session's task folders, a task folder whose plan is not accepted
//   (to start a task), a project's or any task's artifacts folder (not a history.md or task.md in it;
//   a change asked on an artifact goes to a planner), the brain (a project's `knowledge/` folder or the
//   Grid home's, GRID_HOME/knowledge), or the projects folder while the session has no task.
// - A grid-curator writes only the brain and its task folder (with or without Grid's folders set); only it and a
//   grid-planner write the brain.
// - Any other subagent never writes the brain: it writes a proposal in its task's brain-proposals/.
// - In a task whose plan is not accepted, whatever its status, history.md and task.md (any letter
//   case) are not changed to add the `· plan accepted ·` or `· preview approved ·` marker anywhere in a
//   line (core still reads the first there, so this is stricter than the hold), a step past the approval or unknown, a workflow
//   without the approval, a closed or paused status, or a task.md that cannot be read for sure (unless
//   it could not be before), as Grid records the acceptance. Every way Claude Code could apply an edit
//   is tried (old_string as written or after CRLF to LF, new_string literal or with `$&` patterns); an
//   edit whose result cannot be told (no match, input that is not text) and a notebook edit of those
//   files are denied.
// - In such a task a top-level name with a character outside printable ASCII, and any name with a
//   colon or a part ending in a dot or space, is denied: on macOS or Windows it can open history.md or
//   task.md under another spelling. So is such a name, or tasks in other letters, for the folder in a
//   project's tasks folder's place, which could open a task unseen.
// - In any task, whatever the plan's state, history.md (any letter case, or a trailing dot or space, or a colon stream) is not
//   changed to add a `· preview approved ·` line: a person approves in Grid, and the brain trusts that line.
// - In any task, history.md is not changed to add a `· plan ready ·` line while plan.md has no brain recall line
//   (`Used: <path>` or `Recall: nothing found`, lib.mjs hasRecallLine): the plan reaches the gate with its recall cited.
// - In any task, a top-level name that reads as a gate event (plan accepted, preview approved, ship
//   approved; any case, spaces trimmed) is denied: history.md logs the names written.
// - A grid-reviewer or grid-checker writes only inside the projects folder, in any mode (its Verdict);
//   GRID_PLAN_GUARD=off turns this off too.
// Paths are expanded (~), resolved against cwd and through symlinks (realpath.native, a link that
// points nowhere followed from its own real folder) before they are compared.
// Not protected: a Bash write (beyond recallGate's text match); a repo inside the projects folder (GRID_PROJECTS or GRID_HOME/projects);
// an agent before it first writes into a task folder or works in one (the app starts it in the repo);
// a file changed between this check and the write. A held task left unaccepted keeps its agent out
// of the repo until the person accepts, closes or pauses it. GRID_PLAN_GUARD=off (any case) in the
// environment Claude Code starts with turns the guard off, for a plugin-only user who has no app to
// accept the plan; with no GRID_PROJECTS or GRID_HOME it does nothing.
import os from 'node:os';
import path from 'node:path';
import {
  main, readInput, str, gridFolders, taskAt, readText, agentTasks, heldTasks, agentName, planStage, heldStage, inactive, isHeld,
  gateLike, PLAN_ACCEPTED, PLAN_READY, hasRecallLine, realNative,
} from './lib.mjs';

const WRITES = new Set(['Write', 'Edit', 'MultiEdit', 'NotebookEdit']);
const GUARDED_FILES = new Set(['history.md', 'task.md']);
// The two roles that write the brain, matched on the exact agent_type (`grid-curator`, or with the plugin's `grid:` prefix),
// never on agentName's looser reading, so `other:grid-curator` or "Grid Curator" is not one.
const BRAIN_ROLES = new Set(['grid-curator', 'grid-planner']);
const brainRole = (raw) => { const t = str(raw).trim(); const r = t.startsWith('grid:') ? t.slice(5) : t; return BRAIN_ROLES.has(r) ? r : ''; };
const READ_ONLY = new Set(['grid-reviewer', 'grid-checker']);
const ODD_NAME = /[^\x20-\x7e]|:|[. ](?:\/|$)/;
const ODD_PART = /:|[. ](?:\/|$)/;
const NOT_ASCII = /[^\x20-\x7e]/;

const OFF = ' To turn this guard off, start Claude Code with GRID_PLAN_GUARD=off.';

main(async () => {
  const input = await readInput();
  const { home, projects, set } = gridFolders(process.env, str(input?.cwd));
  if (!input || !projects) return;
  if (str(process.env.GRID_PLAN_GUARD).trim().toLowerCase() === 'off') return;
  if (input.tool_name === 'Bash') return recallGate(input, projects);
  if (!WRITES.has(input.tool_name)) return;
  const toolInput = input.tool_input && typeof input.tool_input === 'object' ? input.tool_input : {};
  const given = str(input.tool_name === 'NotebookEdit' ? toolInput.notebook_path : toolInput.file_path);
  if (!given) return;
  const cwd = str(input.cwd);
  const target = realNative(path.resolve(cwd, expandHome(given)));
  const root = realNative(projects);
  const owner = taskAt(target, root);
  const taskText = owner ? readText(path.join(owner.dir, 'task.md')) : null;
  const historyText = owner ? readText(path.join(owner.dir, 'history.md')) : null;
  const unaccepted = owner && heldStage(planStage(taskText, historyText));
  const slot = path.relative(root, target).split(path.sep);
  if (!owner && inside(root, target) && slot.length > 2 && slot[1] !== 'tasks' && (ODD_NAME.test(slot[1]) || slot[1].toLowerCase() === 'tasks')) {
    return deny(`A folder beside a project's tasks folder keeps a plain name (printable ASCII, no colon, no dot or space at the end, not tasks in other letters): on macOS or Windows it can be the tasks folder under another spelling, so ${given} is not written.${OFF}`);
  }
  const top = owner ? owner.rel.split('/')[0] : '';
  if (owner && gateLike(top)) {
    return deny(`${given} is not written: a name at the top of a task folder is not plan accepted, preview approved or ship approved, as history.md would log it like a person's approval.${OFF}`);
  }
  // In every plan state: people approve a preview in Grid, never through an agent edit, and the brain trusts the line.
  // A line a hook logged and a line already in the file stay; only a new one is refused. What an edit would leave that cannot be told
  // is not refused here (Claude Code does not apply an edit that fits no reading).
  if (owner && /^history\.md(?::.*|[. ]*)$/i.test(owner.rel)) {
    const next = after(input.tool_name, toolInput, historyText);
    if (next && next.some((t) => addsMark(historyText, t, [PREVIEW_APPROVED]))) {
      return deny(`${given} is not changed to add a preview approved line: a person approves the preview in Grid, and Grid writes the line.${OFF}`);
    }
    // The plan goes to the gate only with its brain recall cited (the s41 incident: a planner skipped recall and nothing said so).
    if (next && next.some((t) => addsMark(historyText, t, [PLAN_READY])) && !hasRecallLine(readText(path.join(owner.dir, 'plan.md')))) {
      return deny(`${given} is not changed to add a plan ready line: plan.md has no brain recall line. Add it under ## How we approach it: one \`Used: <path>\` line for each note of the recall you used, or \`Recall: nothing found\` when no note bears on the task (the recall hits are in your context, or run \`grid-brain recall --task <id>\`). Then run plan-hash.mjs again and add the plan ready line.${OFF}`);
    }
  }
  if (unaccepted && (NOT_ASCII.test(top) || ODD_PART.test(owner.rel))) {
    return deny(`This task's plan is not accepted yet, so ${given} is not written: until then its files keep plain names (printable ASCII at the top of the task folder, no colon, no dot or space at the end).${OFF}`);
  }
  const forged = unaccepted ? leavesPlan(input.tool_name, toolInput, owner.rel.toLowerCase(), taskText, historyText) : '';
  if (forged === 'status') {
    return deny(`This task's plan is not accepted yet, so ${given} is not changed to close or pause the task: the person does that in Grid.${OFF}`);
  }
  if (forged === 'unreadable') {
    return deny(`This task's plan is not accepted yet, so ${given} is not changed to a task.md whose step or workflow cannot be read for sure: give it a step and a workflow of {step, kind} items with plain lower-case names, each named once.${OFF}`);
  }
  if (forged === 'unclear') {
    return deny(`This task's plan is not accepted yet: Grid records the acceptance, and what ${given} would hold after this edit cannot be told (an old_string not in the file, or input that is not text), so it is not changed. Read it and edit it again.${OFF}`);
  }
  if (forged) {
    return deny(`This task's plan is not accepted yet: Grid records the acceptance, so ${given} is not changed to say so or to move the task past its plan step.${OFF}`);
  }
  if (set && READ_ONLY.has(agentName(input.agent_type)) && !inside(root, target)) {
    return deny(`A ${agentName(input.agent_type)} writes only its Verdict inside a task folder; ${target} is outside it.`);
  }
  const agent = str(input.agent_type).trim() ? agentName(input.agent_type) : '';
  const role = brainRole(input.agent_type);
  if (agent && !role && brainFile(root, target, home)) {
    return deny(`Only a grid-curator (or a grid-planner) writes the brain; a ${agent} writes a proposal in its task's brain-proposals/ folder instead of ${given}.`);
  }
  const here = cwd ? taskAt(realNative(cwd), root)?.dir : undefined;
  const tasks = [...new Set([here, ...agentTasks(input, projects)].filter(Boolean))];
  if (tasks.some((d) => inside(d, target))) return;
  if (role === 'grid-curator') {
    // The curator writes the brain and its task folder, nothing else, with or without Grid's folders set.
    if (brainFile(root, target, home)) return;
    return deny(`A curator writes only the brain (a project's knowledge/ folder or the Grid home's) and its task folder. Write inside ${tasks.length ? tasks.join(' or ') : root}, not ${given}.${OFF}`);
  }
  if (set && role === 'grid-planner') {
    if (unaccepted || (!owner && !tasks.length && inside(root, target)) || artifact(root, target, owner) || brainFile(root, target, home)) return;
    return deny(`A planner writes only inside a task folder (its own, or one whose plan is not accepted yet), an artifacts folder or a brain folder. Write inside ${tasks.length ? tasks.join(' or ') : root}, not ${given}.${OFF}`);
  }
  const held = [...new Set([...tasks, ...heldTasks(input, projects)])].filter(isHeld);
  if (!held.length || inside(root, target)) return;
  return deny(`A task whose plan is not accepted yet writes only inside its task folder or the Grid projects folder. Write inside ${held.join(' or ')}, not ${given}. Writes elsewhere start once the person accepts the plan in Grid.${OFF}`);
});

// Bash: the plan's recall gate for an agent that writes history.md where no file hook sees it (Codex's apply_patch, a shell
// write). plan-hash.mjs writing the revision line of a held task's plan.md, or a shell command that puts a plan ready line in
// a held task's history.md, is refused while plan.md has no Recall or Used line: without the hash there is no plan ready line
// Grid takes. `plan-hash.mjs --print` writes nothing and is let through. A text match on the command, like host-guard's.
const PLAN_HASH = /plan-hash\.mjs['"]?\s+(?:'([^']+)'|"([^"]+)"|(\S+))/g;
const SHELL_READY = /(?:'([^']*history\.md)'|"([^"]*history\.md)"|((?:[^\s'"<>|;&]*\/)?history\.md))/g;
const RECALL_MISSING = 'plan.md has no brain recall line. Add it under ## How we approach it: one `Used: <path>` line for each note of the recall you use, or `Recall: nothing found` when no note bears on the task (the recall hits are in your context, or run `grid-brain recall --task <id>`).';

// Whether the command writes the file (a redirect into it, tee, sed -i, perl -i, a python open for writing), not only reads it: a
// grep for `plan ready` passes. The whole command is judged, with the file named anywhere in it. One exemption: every write is a
// tee whose arguments are all plain literal paths (no $, backtick, glob or quote) that are not the file, with no redirect into it
// and no xargs, so a read piped into `tee /tmp/out` is let through.
const REDIRECT_TO_FILE = />[>|]?\s*(?:'[^']*history\.md'|"[^"]*history\.md"|[^\s'"<>|;&]*history\.md)/;
const OTHER_WRITE = /\b(?:sed|perl)\b[^|;&]*\s-[a-zA-Z]*i|\bpython3?\b[\s\S]*?\s-c\b[\s\S]*?(?:\bopen\([\s\S]*['"][wax]\+?b?['"]|\.write_text\()/;
const LITERAL_PATH = /^[A-Za-z0-9_.\/~+@%,:=-]+$/;
function writesTo(command, file, cwd) {
  const hasTee = /\btee\b/.test(command);
  if (REDIRECT_TO_FILE.test(command) || OTHER_WRITE.test(command)) return true;
  if (!hasTee) return false;
  if (/\bxargs\b/.test(command) || /\b(?:cp|mv|dd|install|ln)\b|\bof=/.test(command)) return true;
  if (/\$/.test(command.replace(/'[^']*'/g, ''))) return true;
  for (const m of command.matchAll(/\btee\b([^|;&\n<>]*)/g)) {
    for (const w of m[1].trim().split(/\s+/).filter(Boolean)) {
      if (w.startsWith('-')) continue;
      if (!LITERAL_PATH.test(w)) return true;
      if (realNative(path.resolve(cwd, expandHome(w))) === file) return true;
    }
  }
  return false;
}

// A tee word with a glob whose last part matches history.md is a candidate file too (tee dir/hist*.md).
const globCandidates = (command) => [...command.matchAll(/\btee\b([^|;&\n<>]*)/g)].flatMap((m) => m[1].trim().split(/\s+/))
  .filter((w) => /[*?[]/.test(w) && new RegExp(`^${path.basename(w).replace(/[.+^${}()|\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.')}$`).test('history.md'))
  .map((w) => path.join(path.dirname(w), 'history.md'));

function recallGate(input, projects) {
  const command = str(input.tool_input?.command);
  const cwd = str(input.cwd);
  const root = realNative(projects);
  const held = (file) => {
    const owner = taskAt(realNative(path.resolve(cwd, expandHome(file))), root);
    if (!owner) return null;
    const taskText = readText(path.join(owner.dir, 'task.md'));
    if (inactive(taskText)) return null;
    const stage = planStage(taskText, readText(path.join(owner.dir, 'history.md')));
    return heldStage(stage) && !hasRecallLine(readText(path.join(owner.dir, 'plan.md'))) ? owner : null;
  };
  if (!/(?:^|\s)--print(?:\s|$)/.test(command)) {
    for (const m of command.matchAll(PLAN_HASH)) {
      const file = m[1] ?? m[2] ?? m[3];
      if (path.basename(file) === 'plan.md' && held(file)) return deny(`plan-hash.mjs is not run on ${file}: ${RECALL_MISSING} Then run it again.${OFF}`);
    }
  }
  if (command.includes('plan ready')) {
    const files = [...[...command.matchAll(SHELL_READY), ...command.replace(/["']/g, '').matchAll(SHELL_READY)].map((m) => m[1] ?? m[2] ?? m[3]), ...globCandidates(command)];
    for (const file of files) {
      if (writesTo(command, realNative(path.resolve(cwd, expandHome(file))), cwd) && held(file)) return deny(`This command is not run: it adds a plan ready line to ${file}, and ${RECALL_MISSING}${OFF}`);
    }
  }
  return undefined;
}

const deny = (reason) => ({ stdout: JSON.stringify({ hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: 'deny', permissionDecisionReason: reason } }) });

// Whether file is root or under it, both resolved through symlinks.
function inside(root, file) {
  const rel = path.relative(realNative(root), file);
  return rel === '' || (rel !== '..' && !rel.startsWith('..' + path.sep) && !path.isAbsolute(rel));
}

// A file under a project's artifacts folder or a task's, other than a history.md or task.md.
function artifact(root, target, owner) {
  if (!inside(root, target) || GUARDED_FILES.has(path.basename(target).toLowerCase())) return false;
  if (owner) return owner.rel.startsWith('artifacts/');
  const slot = path.relative(root, target).split(path.sep);
  return slot.length > 2 && slot[1] === 'artifacts';
}

// A file in the brain: a project's knowledge folder (<projects>/<project>/knowledge/) or the Grid home's (GRID_HOME/knowledge/).
// A project's brain/ folder is the Notes tab's, not the brain.
// Any letter case, and the folder's own name as a file: on macOS or Windows `Knowledge/` is the same folder.
function brainFile(root, target, gridHome) {
  const slot = path.relative(root, target).split(path.sep);
  if (inside(root, target) && slot.length >= 2 && slot[1].toLowerCase() === 'knowledge') return true;
  const home = str(gridHome).trim();
  if (!home) return false;
  const homeReal = realNative(path.resolve(expandHome(home)));
  // target is already resolved, so a knowledge link that points elsewhere lands outside the Grid home's knowledge.
  return inside(homeReal, target) && path.relative(homeReal, target).split(path.sep)[0].toLowerCase() === 'knowledge';
}

// The approval markers an agent never writes: Grid's own plan accepted and preview approved lines (the brain trusts the second).
const PREVIEW_APPROVED = '· preview approved ·';
const MARKERS = [PLAN_ACCEPTED, PREVIEW_APPROVED];

// The lines with an approval marker anywhere in them, as core/lifecycle.mjs reads history.md.
const marked = (text, markers = MARKERS) => str(text).split(/\r?\n/).filter((l) => markers.some((m) => l.includes(m)));

// Whether next has a marked line that text does not, each line of text matched once.
function addsMark(text, next, markers = MARKERS) {
  const left = marked(text, markers);
  return marked(next, markers).some((l) => {
    const i = left.indexOf(l);
    if (i < 0) return true;
    left.splice(i, 1);
    return false;
  });
}

// A leading ~ is the home folder; Claude Code passes the path as the model wrote it.
const expandHome = (p) => (p === '~' || /^~[\\/]/.test(p) ? path.join(os.homedir(), p.slice(1)) : p);

// Every text the file could hold after the call, or null when it cannot be told. Claude Code matches
// old_string in the text it read (line ends made LF) and may read `$&` in new_string as a pattern, so
// each start and each reading gives a candidate; a reading where an edit does not fit gives none.
function after(tool, toolInput, text) {
  if (tool === 'Write') return typeof toolInput.content === 'string' ? [toolInput.content] : null;
  const edits = tool === 'MultiEdit' ? toolInput.edits : [toolInput];
  if (!Array.isArray(edits) || !edits.length) return null;
  const bad = (e) => !e || typeof e !== 'object' || typeof e.old_string !== 'string' || typeof e.new_string !== 'string'
    || (e.replace_all !== undefined && typeof e.replace_all !== 'boolean');
  if (edits.some(bad)) return null;
  const out = [];
  for (const start of new Set([text ?? '', (text ?? '').replace(/\r\n/g, '\n')])) {
    for (const literal of [true, false]) {
      let cur = start;
      for (const e of edits) if (cur !== null) cur = applyEdit(cur, e, literal);
      if (cur !== null) out.push(cur);
    }
  }
  return out.length ? out : null;
}

// One edit, or null when it does not fit: an empty old_string only fits a file with no text yet.
function applyEdit(text, e, literal) {
  if (e.old_string === '') return text === '' ? e.new_string : null;
  if (!text.includes(e.old_string)) return null;
  if (!literal) return e.replace_all ? text.replaceAll(e.old_string, e.new_string) : text.replace(e.old_string, e.new_string);
  return e.replace_all ? text.split(e.old_string).join(e.new_string) : text.replace(e.old_string, () => e.new_string);
}

// A write to history.md or task.md of a task whose plan is not accepted that would leave the plan:
// 'accepted' for the accepted line or a step or workflow that ends the hold, 'status' for a status
// that lifts it, 'unreadable' for a task.md that could be read before (or was missing) and no longer
// can, 'unclear' when the text after it cannot be told, '' when it is fine. A new task.md
// whose workflow has no plan approval is fine: the task has no plan to accept.
function leavesPlan(tool, toolInput, file, taskText, historyText) {
  if (!GUARDED_FILES.has(file)) return '';
  if (tool === 'NotebookEdit') return 'unclear';
  const isTask = file === 'task.md';
  const texts = after(tool, toolInput, isTask ? taskText : historyText);
  if (!texts) return 'unclear';
  const wasUnreadable = planStage(taskText, historyText) === 'unreadable';
  for (const next of texts) {
    if (!isTask && addsMark(historyText, next)) return 'accepted';
    const stage = isTask ? planStage(next, historyText) : planStage(taskText, next);
    if (!heldStage(stage) && !(isTask && !taskText && stage === '')) return 'accepted';
    if (isTask && stage === 'unreadable' && !wasUnreadable) return 'unreadable';
    if (isTask && inactive(next) && !inactive(taskText)) return 'status';
  }
  return '';
}
