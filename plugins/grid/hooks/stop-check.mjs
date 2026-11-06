// Stop and SubagentStop: block once (exit 2, the reason on stderr goes to Claude) when the work
// is not written down. stop_hook_active true means Claude is already continuing because of a stop
// hook, so it is allowed through.
//   - Stop: for each task folder the main thread wrote into itself (or has as its cwd; a subagent's writes
//     are its own, see rememberTask in lib): task.md has a
//     summary and a next, and a brief in subagents/ with status done or blocked has an Outcome.
//     briefed and running briefs are left alone: they are not finished yet.
//     Also the approve gate: when the task's current step is an approve step and its finished worker
//     brief has an Outcome and no Verdict, and this session made no reviewer Agent call, Claude is
//     told to start grid-reviewer on that brief. The execute skill sometimes reports and stops
//     instead (status may still be running, or waiting). It blocks once per session per brief, so a
//     reviewer that cannot be started does not trap the session. The task folder is found only from
//     the main thread's own writes into it or from cwd.
//     Also the carry-on gate: a running or reworking task whose current step is a run or check step
//     nobody has to answer (no open question, no blocked brief, not the finished last step, not a task
//     just created) is told to carry on with it. stop_hook_active ends the push, so one push per chain.
//   - SubagentStop: the Outcome of the brief that subagent was given (found in its transcript, or else
//     in the Agent call that started it: briefFor).
//     A reviewer writes Verdict, never Outcome, so when the subagent is a reviewer (agent_type,
//     whatever the brief's role: it usually reviews a worker's brief) its Verdict is checked.
//     Otherwise a brief whose role is a reviewer role has its Outcome empty by design and the
//     Verdict is checked instead. A scout has no Write or Edit tool, so it is not asked for an
//     Outcome either (the dispatcher's own write is still checked by Stop). A missing agent_type
//     falls back to the brief's role alone.
import fs from 'node:fs';
import path from 'node:path';
import { main, readInput, str, taskAt, frontmatter, isEmpty, section, readText, agentTasks, briefFor, agentName, unreviewedApproval, carryOnStep, reviewerCalled, firstGate, projectsRoot, REVIEWER_ROLES, NON_WRITERS } from './lib.mjs';

main(async () => {
  const input = await readInput();
  const projects = projectsRoot(process.env, str(input?.cwd));
  if (!input || !projects || input.stop_hook_active === true) return;
  const problems = input.hook_event_name === 'SubagentStop' ? subagentProblems(input, projects) : stopProblems(input, projects);
  if (!problems.length) return;
  return { code: 2, stderr: `Not finished yet:\n${problems.map((p) => `- ${p}`).join('\n')}\nFix this, then stop again.\n` };
});

// A file as the message names it: from the folder above the projects folder, so projects/<project>/... on the default setup.
const label = (file, projects) => path.relative(path.dirname(path.resolve(projects)), file).split(path.sep).join('/');
const noOutcome = (file, projects) => `${label(file, projects)}: the "## Outcome" section is empty. Write what was done, what was checked and what is left.`;
const noVerdict = (file, projects) => `${label(file, projects)}: the "## Verdict" section is empty. Write the verdict and what must change, if anything.`;
const noReview = (gate, projects) => `${label(gate.file, projects)}: step "${gate.step}" is an approve step and this finished brief has an Outcome but no Verdict. Start grid-reviewer on this brief now; set "status: waiting" only after its Verdict is written.`;

function stopProblems(input, projects) {
  const dirs = [...new Set([taskAt(str(input.cwd), projects)?.dir, ...agentTasks(input, projects)].filter(Boolean))];
  const out = [];
  for (const dir of dirs) {
    const task = path.join(dir, 'task.md');
    const fm = frontmatter(readText(task) ?? '');
    if (fm) {
      for (const [field, hint] of [['summary', 'one line on what the task is'], ['next', 'one line on what happens next']]) {
        if (isEmpty(fm.data[field])) out.push(`${label(task, projects)}: "${field}" is empty. Write ${hint}.`);
      }
    }
    let briefs = [];
    try { briefs = fs.readdirSync(path.join(dir, 'subagents')).filter((f) => f.endsWith('.md')).sort(); } catch { /* none */ }
    for (const name of briefs) {
      const file = path.join(dir, 'subagents', name);
      const b = frontmatter(readText(file) ?? '');
      if (b && ['done', 'blocked'].includes(b.data.status) && !section(b.body, 'Outcome').trim()) out.push(noOutcome(file, projects));
    }
    const gate = unreviewedApproval(dir, projects);
    if (gate && !reviewerCalled(input) && firstGate(input, gate.file)) out.push(noReview(gate, projects));
    const next = carryOnStep(dir);
    if (next) out.push(`${label(task, projects)}: step ${next} is current and needs no one: record its result if its brief is done, else run it.`);
  }
  return out;
}

function subagentProblems(input, projects) {
  const found = briefFor(input, projects);
  if (!found) return [];
  const file = path.join(found.dir, 'subagents', `${found.brief}.md`);
  const b = frontmatter(readText(file) ?? '');
  if (!b) return [];
  const verdict = () => (section(b.body, 'Verdict').trim() ? [] : [noVerdict(file, projects)]);
  if (str(input.agent_type).trim() && REVIEWER_ROLES.has(agentName(input.agent_type))) return verdict();
  if (NON_WRITERS.has(agentName(input.agent_type))) return [];
  if (section(b.body, 'Outcome').trim()) return [];
  return REVIEWER_ROLES.has(str(b.data.role).toLowerCase()) ? verdict() : [noOutcome(file, projects)];
}
