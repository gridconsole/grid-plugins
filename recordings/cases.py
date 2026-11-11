"""Inputs and fixture variants. Paths are tokens expanded only in temporary copies."""
from pathlib import Path
import json

ROOT = Path(__file__).resolve().parents[1]
T = '$GRID_HOME/projects/demo/tasks/T-1-demo'
B = T + '/subagents/01-build.md'
R = '$GRID_HOME/../workspace/demo'
D = '$GRID_HOME/plugin-data'
TASK = (ROOT / 'fixtures/projects/demo/tasks/T-1-demo/task.md').read_text()
BRIEF = (ROOT / 'fixtures/projects/demo/tasks/T-1-demo/subagents/01-build.md').read_text()
HISTORY = (ROOT / 'fixtures/projects/demo/tasks/T-1-demo/history.md').read_text()
ACCEPTED = HISTORY + '- 2025-01-02T03:04 · plan accepted · rev 1 · by member\n'
CASES = []


def event(kind='PreToolUse', tool=None, ti=None, cwd=T, **kw):
    out = dict(hook_event_name=kind, cwd=cwd, session_id='session-demo', **kw)
    if tool:
        out.update(tool_name=tool, tool_input=ti or {})
    return out


def add(hook, name, proves, input=None, **kw):
    CASES.append(dict(hook=hook, name=name, proves=proves,
                      input=event() if input is None else input, **kw))


def write(file, content='demo\n', **kw):
    return event(tool='Write', ti=dict(file_path=file, content=content), **kw)


def edit_brief(old, new):
    return {B: BRIEF.replace(old, new)}


add('session-check', 'supported', 'Plugin data suppresses the version warning.')
add('session-check', 'missing-data', 'Missing plugin data emits a version warning.', env={'CLAUDE_PLUGIN_DATA': None}, contains='2.1.78')
add('session-check', 'empty-data', 'Empty plugin data also warns.', env={'CLAUDE_PLUGIN_DATA': ''}, contains='2.1.78')
for name, inp in [('session', event('SessionStart')), ('empty-object', {}), ('invalid-json', '{')]:
    add('agent-routing', name, 'Routing is independent of stdin and lists shipped agents.', inp, contains='grid:grid-builder')
config = json.dumps({'current': 'demo', 'workspaces': [{'name': 'demo', 'folder': R, 'data': '$GRID_HOME'}]})
config_file = '$GRID_HOME/../user/$APP_CONFIG/app.gridconsole.desktop/config.json'
add('grid-env', 'explicit', 'Explicit Grid variables prevent discovery and env-file writes.', event('SessionStart', cwd=R))
add('grid-env', 'discovered', 'Desktop config selects a project and writes shell exports.', event('SessionStart', cwd=R), env={'GRID_HOME': None, 'GRID_PROJECTS': None}, files={config_file: config}, contains='Grid is set up', changed=['claude.env'])
add('grid-env', 'no-config', 'No desktop configuration gives no output.', event('SessionStart', cwd=R), env={'GRID_HOME': None, 'GRID_PROJECTS': None})

add('plan-guard', 'task-write', 'A held task may write its own plan.', write(T + '/plan.md'))
add('plan-guard', 'accepted-repo', 'Plan acceptance releases repository writes.', write(R + '/demo.txt'), files={T + '/history.md': ACCEPTED})
add('plan-guard', 'guard-off', 'The explicit off switch releases repository writes.', write(R + '/demo.txt'), env={'GRID_PLAN_GUARD': 'OFF'})
add('plan-guard', 'forged-acceptance', 'An agent cannot insert a plan acceptance.', write(T + '/history.md', ACCEPTED), contains='deny')
add('plan-guard', 'forged-preview', 'Preview approval cannot be forged even after plan acceptance.', write(T + '/history.md', ACCEPTED + '- now · preview approved · demo\n'), files={T + '/history.md': ACCEPTED}, contains='deny')
add('plan-guard', 'missing-recall', 'Plan ready requires a recall citation.', write(T + '/history.md', HISTORY + '- now · plan ready · rev 1\n'), files={T + '/plan.md': '# Plan, rev 1\n'}, contains='deny')
add('plan-guard', 'recall-present', 'Plan ready passes when the plan cites recall.', write(T + '/history.md', HISTORY + '- now · plan ready · rev 1\n'))
add('plan-guard', 'brain-builder', 'Builders must propose rather than write brain notes.', write('$GRID_HOME/knowledge/demo.md', agent_type='grid:grid-builder'), contains='deny')
add('plan-guard', 'brain-curator', 'Curators can write brain notes.', write('$GRID_HOME/knowledge/demo.md', agent_type='grid:grid-curator'))
add('plan-guard', 'gate-filename', 'Approval-like filenames are forbidden.', write(T + '/plan accepted'), contains='deny')
add('plan-guard', 'unmatched-edit', 'An edit with an unknown result cannot change task metadata.', event(tool='Edit', ti={'file_path': T + '/task.md', 'old_string': 'missing text', 'new_string': 'status: closed'}), contains='deny')
add('plan-guard', 'bash-recall', 'Bash cannot write plan ready without recall.', event(tool='Bash', ti={'command': "echo 'plan ready' >> history.md"}), files={T + '/plan.md': '# Plan\n'}, contains='deny')
add('plan-guard', 'invalid-json', 'Malformed input fails open.', '{')

add('host-guard', 'ordinary-write', 'Ordinary task files pass.', write(T + '/plan.md'))
add('host-guard', 'host-write', 'Host approval files are denied even with plan guard off.', write('$GRID_HOME/../.host/approve'), env={'GRID_PLAN_GUARD': 'off'}, contains='deny')
add('host-guard', 'bash-uppercase', 'Bash host-name matching is case insensitive.', event(tool='Bash', ti={'command': 'cat ../.HOST/approve'}), contains='deny')
add('host-guard', 'similar-name', 'A longer dot-host filename does not match the host folder.', event(tool='Bash', ti={'command': 'cat .host-example'}))

brief_input = event(tool='Agent', ti={'subagent_type': 'grid:grid-builder', 'prompt': 'Use ' + B, 'description': 'Build demo'})
add('brief-lint', 'complete', 'A complete brief passes and records dispatch.', brief_input, changed=['.agents'])
add('brief-lint', 'missing-field', 'An empty verify field denies dispatch.', brief_input, files=edit_brief('verify: Check the demo output', 'verify:'), contains='verify')
add('brief-lint', 'missing-section', 'A missing Outcome heading denies dispatch.', brief_input, files=edit_brief('## Outcome\n', ''), contains='Outcome')
add('brief-lint', 'no-frontmatter', 'A readable brief without frontmatter denies dispatch.', brief_input, files={B: '# Brief\n'}, contains='no frontmatter')
add('brief-lint', 'missing-file', 'A nonexistent brief fails open and records an unbound dispatch.', brief_input, files={B: None}, changed=['.agents'])
add('brief-lint', 'no-brief', 'A prompt without a brief may start an agent.', event(tool='Task', ti={'prompt': 'Inspect the demo', 'subagent_type': 'grid:grid-scout'}), changed=['.agents'])
add('brief-lint', 'invalid-json', 'Malformed input is ignored.', '{')

for name, inp, extra in [
    ('inside-read', event(tool='Read', ti={'file_path': B}, cwd=R), {'contains': 'allow'}),
    ('permission', event('PermissionRequest', 'Write', {'file_path': B}, cwd=R), {'contains': 'addDirectories'}),
    ('outside', event(tool='Read', ti={'file_path': R + '/demo.md'}, cwd=R), {}),
    ('relative', event(tool='Read', ti={'file_path': 'plan.md'}, cwd=R), {}),
    ('broad-home', event(tool='Read', ti={'file_path': B}), {}),
    ('glob-traversal', event(tool='Glob', ti={'path': T, 'pattern': '../*.md'}, cwd=R), {}),
    ('own-script', event(tool='Bash', ti={'command': 'node $OLD_PLUGIN/skills/plan/scripts/plan-hash.mjs ' + T + '/plan.md --print'}, cwd=R), {'contains': 'allow'}),
    ('shell-chain', event(tool='Bash', ti={'command': 'node $OLD_PLUGIN/skills/plan/scripts/plan-hash.mjs ' + T + '/plan.md; echo demo'}, cwd=R), {}),
    ('script-outside', event(tool='Bash', ti={'command': 'node $OLD_PLUGIN/skills/plan/scripts/plan-hash.mjs ' + R + '/plan.md'}, cwd=R), {}),
    ('symlink-escape', event(tool='Read', ti={'file_path': '$GRID_HOME/escape/demo.md'}, cwd=R), {'links': {'$GRID_HOME/escape': R}}),
]:
    add('grid-home-allow', name, {'inside-read': 'An absolute read inside Grid home is allowed.', 'permission': 'PermissionRequest adds the home as a session directory.'}.get(name, name.replace('-', ' ').capitalize() + ' receives no automatic permission.' if not extra.get('contains') else 'A single plugin script with safe arguments is allowed.'), inp, **extra)

post_write = event('PostToolUse', 'Write', {'file_path': T + '/notes.md'})
add('history-log', 'file-write', 'A file write appends history and remembers the task.', post_write, changed=['history.md', '.tasks'])
add('history-log', 'agent-edit', 'An old worker alias is logged as grid-builder with its own task pointer.', event('PostToolUse', 'Edit', {'file_path': B}, agent_type='grid:grid-worker', agent_id='agent-demo'), changed=['history.md', '.tasks-agent-demo'])
add('history-log', 'history-self', 'History edits remember the task without recursively logging.', event('PostToolUse', 'Write', {'file_path': T + '/history.md'}), changed=['.tasks'])
add('history-log', 'missing-history', 'The hook does not create a missing history file.', post_write, files={T + '/history.md': None}, changed=['.tasks'])
add('history-log', 'gate-detail', 'An approval-like file name is quoted in history.', event('PostToolUse', 'Write', {'file_path': T + '/plan accepted'}), changed=['history.md'])
add('history-log', 'duplicate', 'Consecutive identical file events are deduplicated.', post_write, pre=[{'hook': 'history-log', 'input': post_write}])
bash = event('PreToolUse', 'Bash', {'command': 'write demo notes'}, tool_use_id='tool-demo')
add('history-log', 'bash-before', 'Bash before saves file size and clock signatures.', bash, changed=['.bash-tool-demo'])
for kind in ['PostToolUse', 'PostToolUseFailure']:
    add('history-log', 'bash-' + ('success' if kind == 'PostToolUse' else 'failure'), 'Bash completion logs created, edited and removed files even on failure.', dict(bash, hook_event_name=kind), pre=[{'hook': 'history-log', 'input': bash}], after_pre={T + '/notes.md': 'new demo\n', T + '/intent.md': 'changed demo intent\n', B: None}, changed=['history.md', '.bash-tool-demo'])
add('history-log', 'background', 'Background Bash calls do not take a premature snapshot.', event(tool='Bash', ti={'command': 'write notes', 'run_in_background': True}, tool_use_id='tool-demo'))
add('history-log', 'subagent-stop', 'SubagentStop records the canonical agent name.', event('SubagentStop', agent_type='grid:grid-worker'), changed=['history.md'])
add('history-log', 'invalid-json', 'Malformed input has no effects.', '{')

stop = event('Stop')
add('stop-check', 'complete', 'A waiting task with summary and next may stop.', stop)
add('stop-check', 'empty-summary', 'Empty task summary blocks Stop with exit 2.', stop, files={T + '/task.md': TASK.replace('summary: Build the demo example', 'summary:')}, exit=2, contains_stderr='"summary" is empty')
add('stop-check', 'empty-next', 'Empty task next blocks Stop with exit 2.', stop, files={T + '/task.md': TASK.replace('next: Await plan acceptance', 'next:')}, exit=2, contains_stderr='"next" is empty')
add('stop-check', 'finished-no-outcome', 'A finished brief needs an Outcome.', stop, files=edit_brief('status: briefed', 'status: done'), exit=2, contains_stderr='Outcome')
add('stop-check', 'already-pushed', 'The stop recursion flag bypasses a second block.', dict(stop, stop_hook_active=True), files=edit_brief('status: briefed', 'status: done'))
transcript = json.dumps({'type': 'user', 'message': {'content': 'Use ' + B}}) + '\n'
substop = event('SubagentStop', agent_type='grid:grid-builder', agent_id='agent-demo', agent_transcript_path='$GRID_HOME/transcript.jsonl')
add('stop-check', 'subagent-missing', 'A builder must fill its own brief Outcome.', substop, files={'$GRID_HOME/transcript.jsonl': transcript}, exit=2, contains_stderr='Outcome')
add('stop-check', 'mark-blocked', 'A repeated stop with no Outcome marks the brief blocked.', dict(substop, stop_hook_active=True), files={'$GRID_HOME/transcript.jsonl': transcript}, changed=['01-build.md'])
add('stop-check', 'mark-done', 'A completed Outcome marks the brief done.', substop, files={'$GRID_HOME/transcript.jsonl': transcript, **edit_brief('## Outcome\n', '## Outcome\nBuilt and checked the demo.\n')}, changed=['01-build.md'])
add('stop-check', 'scout', 'A scout is not required to write an Outcome.', dict(substop, agent_type='grid:grid-scout'), files={'$GRID_HOME/transcript.jsonl': transcript})
add('stop-check', 'reviewer', 'A reviewer must write Verdict even on a builder brief.', dict(substop, agent_type='grid:grid-reviewer'), files={'$GRID_HOME/transcript.jsonl': transcript}, exit=2, contains_stderr='Verdict')
add('stop-check', 'invalid-json', 'Malformed Stop input fails open.', '{')

attach = event('UserPromptSubmit', prompt='/grid:resume T-1', cwd=R)
add('session-attach', 'resume', 'A resume prompt creates the session record and attachment pointer.', attach, changed=['session-demo.json', '.attached'])
add('session-attach', 'grid-task', 'GRID_TASK attaches a session started by Grid.', event('SessionStart', cwd=R), env={'GRID_TASK': 'T-1'}, changed=['session-demo.json'])
add('session-attach', 'file-read', 'Reading a task file does not attach a session.', event('PreToolUse', 'Read', {'file_path': B}, cwd=R))
for name, inp in [('stop', event('Stop', cwd=R)), ('end', event('SessionEnd', cwd=R)), ('subagent', event('SubagentStop', cwd=R))]:
    add('session-record', name, 'An attached session ' + ('ends on SessionEnd.' if name == 'end' else 'is not refreshed inside the ten second throttle.'), inp, pre=[{'hook': 'session-attach', 'input': attach}], **({'changed': ['session-demo.json']} if name == 'end' else {}))
add('session-record', 'unattached', 'An unattached session creates no record.', event('SessionEnd'))

add('usage-log', 'start', 'SessionStart records source and transcript path.', event('SessionStart', source='startup', transcript_path='$GRID_HOME/transcript.jsonl'), changed=['.usage'])
add('usage-log', 'agent-totals', 'Agent totals are whitelisted without copying prompt or result text.', event('PostToolUse', 'Agent', {'prompt': 'Use ' + B, 'subagent_type': 'grid:grid-worker'}, tool_use_id='tool-demo', tool_response={'agentId': 'agent-demo', 'totalTokens': 42, 'totalDurationMs': 25, 'usage': {'input_tokens': 30, 'output_tokens': 12}, 'status': 'done', 'result': 'placeholder result text'}), changed=['.usage'])
add('usage-log', 'subagent', 'SubagentStop stores the canonical agent and transcript.', substop, changed=['.usage'])
add('usage-log', 'wrong-tool', 'Non-Agent tool usage is ignored.', post_write)

read_note = event('PostToolUse', 'Read', {'file_path': '$GRID_HOME/projects/demo/knowledge/demo.md'})
add('brain-read', 'project-note', 'A project brain read appends history and a deduplication marker.', read_note, changed=['history.md', '.brain-read'])
add('brain-read', 'workspace-note', 'A shared brain note uses the workspace prefix.', event('PostToolUse', 'Read', {'file_path': '$GRID_HOME/knowledge/demo.md'}, agent_type='grid:grid-planner'), changed=['history.md'])
add('brain-read', 'other-project', 'A different project brain note is ignored.', event('PostToolUse', 'Read', {'file_path': '$GRID_HOME/projects/other/knowledge/demo.md'}))
add('brain-read', 'duplicate', 'Repeated note reads are suppressed.', read_note, pre=[{'hook': 'brain-read', 'input': read_note}])
recall = event('UserPromptSubmit', prompt='/grid:plan T-1')
add('brain-recall', 'no-hits', 'An empty brain emits recall guidance and records zero hits.', recall, contains='Recall: nothing found', changed=['history.md', '.recalled'])
add('brain-recall', 'accepted', 'An accepted plan needs no planning recall.', recall, files={T + '/history.md': ACCEPTED})
add('brain-recall', 'duplicate', 'Recall is suppressed for the same task, agent and plan revision.', recall, pre=[{'hook': 'brain-recall', 'input': recall}], pre_stdout=True)
add('brain-recall', 'planner-dispatch', 'Planner dispatch records the named task.', event(tool='Agent', ti={'subagent_type': 'grid:grid-planner', 'prompt': 'Plan T-1'}), changed=['.planner'])

work = event('UserPromptSubmit', prompt='Build another demo example', cwd=R)
for name, inp in [('direct-work', work), ('direct-empty', {}), ('direct-invalid', '{')]:
    add('terminal-task', name, 'Direct execution only defines exports; it has no stdin entrypoint.', inp)
add('terminal-task', 'create', 'The exported terminalTask creates the next task from a work prompt.', work, call='terminalTask', env={'GRID_BRIDGE': None}, contains='T-02-build-another-demo-example', changed=['T-02-build-another-demo-example'])
add('terminal-task', 'short-prompt', 'The exported helper rejects a short acknowledgement.', event('UserPromptSubmit', prompt='yes', cwd=R), call='terminalTask', env={'GRID_BRIDGE': None}, contains='null')
add('terminal-task', 'unattended', 'SDK work prompts do not create tasks.', work, call='terminalTask', env={'GRID_BRIDGE': None, 'CLAUDE_CODE_SESSION_ATTENDED': '0'}, contains='null')

worker_env = {'GRID_FEED_TASK': T, 'GRID_JOB': 'J-1', 'GRID_WORKER': 'worker-demo'}
for hook, kind in [('heartbeat', 'PostToolUse'), ('worker-exit', 'SessionEnd')]:
    inp = event(kind, 'Read', {'file_path': B}, reason='clear')
    add(hook, 'not-worker', 'A session without worker environment writes no feed event.', inp)
    add(hook, 'post', 'A worker writes its local feed event without a server.', inp, env=worker_env, dirs=[T + '/feed'], changed=['member@example~worker-demo.jsonl'])
    add(hook, 'repeat', 'A repeated event is throttled or deduplicated.', inp, env=worker_env, dirs=[T + '/feed'], pre=[{'hook': hook, 'input': inp}])
    add(hook, 'missing-feed-folder', 'A missing feed directory prevents lock acquisition and silently skips posting.', inp, env=worker_env)
    add(hook, 'missing-task', 'An unresolved worker task fails open.', inp, env=dict(worker_env, GRID_FEED_TASK='T-999'))

for hook, kind in [('needs-you', 'Notification'), ('permission-forward', 'PermissionRequest')]:
    add(hook, 'bridge-off', 'With the bridge disabled no request or decision is made.', event(kind, message='Demo notification'))
    add(hook, 'no-bridge', 'Missing desktop bridge data falls through silently.', event(kind), env={'GRID_BRIDGE': None})
    add(hook, 'invalid-json', 'Malformed event input is ignored.', '{')

pure = {
    'parseFrontmatter': [('scalars', ['---\ntitle: "Demo" # comment\ncount: 2\n---\nBody\n']), ('lists', [TASK]), ('odd-key', ['---\n__proto__: demo\n---\n']), ('missing', ['plain text']), ('crlf', ['---\r\ntitle: Demo\r\n---\r\nBody'])],
    'historyLine': [('detail', ['2025-01-02T03:04', 'file written', 'plan.md', 'claude']), ('empty-detail', ['2025-01-02T03:04', 'created', '', 'member']), ('via', ['2025-01-02T03:04', 'file edited', 'plan.md', 'grid-builder', 'Bash']), ('literal-text', ['clock', 'event', 'a\nb · c', 'member'])],
    'agentName': [('qualified', ['grid:grid-builder']), ('legacy-alias', ['grid:grid-worker']), ('punctuation', [' Grid Builder! ']), ('missing', [None])],
    'gateLike': [('approval', [' PLAN   Accepted ']), ('preview', ['preview approved']), ('substring', ['file plan accepted.md']), ('non-string', [None])],
    'workflowSteps': [('valid', [TASK]), ('duplicate', [TASK.replace('step: build,', 'step: plan,')]), ('scalar', ['---\nworkflow: build\n---\n']), ('odd-key', [TASK.replace('id: T-1', 'constructor: bad')])],
    'planStage': [('gate', [TASK, HISTORY]), ('accepted', [TASK, ACCEPTED]), ('missing', [None, '']), ('before', [TASK.replace('step: plan-review\n', 'step: plan\n'), '']), ('past', [TASK.replace('step: plan-review\n', 'step: build\n'), '']), ('unreadable', ['---\nstep: build\n---\n', '']), ('fake-acceptance', [TASK, '- now · file written · plan accepted · by claude\n'])],
}
for fn, cases in pure.items():
    for name, args in cases:
        add('lib-' + fn, name, fn + ' preserves the ' + name.replace('-', ' ') + ' behavior.', {'args': args}, call=fn)
