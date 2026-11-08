#!/usr/bin/env python3
"""Execute reference JavaScript, never a reimplementation of a hook."""
import argparse
import difflib
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

from cases import CASES, ROOT


def main():
    os.umask(0o022)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Regenerate and compare without replacing output')
    parser.add_argument('--source', default=os.environ.get('OLD_PLUGIN', '/srv/gridwork/ref/old-plugin'))
    parser.add_argument('--output', default='golden', help='Destination inside this checkout')
    args = parser.parse_args()
    source = Path(args.source).resolve()
    output = (ROOT / args.output).resolve()
    if not output.is_relative_to(ROOT) or output == ROOT:
        parser.error('output must be a subdirectory of the checkout')
    if output.exists() and not (output / 'INDEX.md').exists() and any(output.iterdir()):
        parser.error('refusing to replace a non-recording directory')
    node = shutil.which('node')
    if not node:
        parser.error('node is required')
    hooks = {p.stem for p in (source / 'hooks').glob('*.mjs')} - {'lib'}
    covered = {c['hook'] for c in CASES if not c['hook'].startswith('lib-')}
    if hooks != covered:
        raise RuntimeError(f'hook coverage differs: {sorted(hooks ^ covered)}')
    for hook in hooks:
        assert sum(c['hook'] == hook for c in CASES) >= 3, hook
    assert len({(c['hook'], c['name']) for c in CASES}) == len(CASES)
    with tempfile.TemporaryDirectory(prefix='.record-', dir=ROOT) as scratch:
        scratch = Path(scratch)
        destination = scratch / 'golden'
        destination.mkdir()
        for case in CASES:
            record(case, source, node, scratch, destination)
        index = index_text(source, node)
        (destination / 'INDEX.md').write_text(index)
        if args.check:
            def contents(folder):
                return {str(p.relative_to(folder)): p.read_bytes() for p in folder.rglob('*') if p.is_file()}
            actual, expected = contents(destination), contents(output)
            differences = sorted(k for k in actual.keys() | expected.keys() if actual.get(k) != expected.get(k))
            if differences:
                raise RuntimeError('recordings differ: ' + ', '.join(differences))
        else:
            if output.exists():
                shutil.rmtree(output)
            output.parent.mkdir(parents=True, exist_ok=True)
            shutil.move(destination, output)
    verb = 'Verified' if args.check else 'Recorded'
    print(f'{verb} {len(CASES)} cases across {len(hooks)} scripts and six library functions into {output.relative_to(ROOT)}.')


def record(case, source, node, scratch, destination):
    # All mutable files, including HOME and the fake checkout, stay in this lane.
    box = scratch / 'case'
    box.mkdir()
    home = box / 'demo'
    workspace = box / 'workspace/demo'
    user = box / 'user'
    shutil.copytree(ROOT / 'fixtures', home)
    workspace.mkdir(parents=True)
    user.mkdir()
    replacements = {'$GRID_HOME': str(home), '$OLD_PLUGIN': str(source), '$RECORDER': str(ROOT / 'recordings')}

    def expand(value):
        if isinstance(value, str):
            for token, actual in replacements.items():
                value = value.replace(token, actual)
            return value
        if isinstance(value, list):
            return [expand(v) for v in value]
        if isinstance(value, dict):
            return {expand(k): expand(v) for k, v in value.items()}
        return value

    ids = {}

    def normalize(raw):
        text = raw.decode('utf-8') if isinstance(raw, bytes) else raw
        for token, actual in sorted(replacements.items(), key=lambda item: -len(item[1])):
            text = text.replace(actual, token)
        text = text.replace(str(box), '$GRID_HOME/..')
        text = re.sub(r'\b\d{4}-\d\d-\d\dT\d\d:\d\d(?::\d\d(?:\.\d+)?Z?)?', '<TIME>', text)
        # taskFiles uses size:mtimeMs:ctimeMs; retain size and normalize only clocks.
        text = re.sub(r'(?<=:)\d{13}(?:\.\d+)?(?=[:"\s])', '<TIME>', text)
        # Feed IDs are random ULIDs. Keep identity and references, with stable first-seen labels.
        def ident(match):
            value = match[0]
            if value not in ids:
                ids[value] = f'<ID-{len(ids) + 1}>'
            return ids[value]
        return re.sub(r'\b[0-9A-HJKMNP-TV-Z]{26}\b', ident, text)

    def safe_path(path):
        at = Path(expand(path))
        if not at.resolve().is_relative_to(box):
            raise RuntimeError('fixture modification escapes the temporary case')
        return at

    def patch(files):
        for path, content in files.items():
            at = safe_path(path)
            if content is None:
                at.unlink(missing_ok=True)
            else:
                at.parent.mkdir(parents=True, exist_ok=True)
                at.write_text(expand(content))

    for file in home.rglob('*'):
        if file.is_file():
            file.write_text(expand(file.read_text()))
    for directory in case.get('dirs', []):
        safe_path(directory).mkdir(parents=True, exist_ok=True)
    patch(case.get('files', {}))
    for path, target in case.get('links', {}).items():
        at = safe_path(path)
        at.parent.mkdir(parents=True, exist_ok=True)
        at.symlink_to(safe_path(target))
    env = dict(HOME=str(user), USER='member', LOGNAME='member', USERNAME='member',
               PATH='/usr/bin:/bin', LANG='C.UTF-8', LC_ALL='C.UTF-8', TZ='UTC',
               TMPDIR=str(box), XDG_CONFIG_HOME=str(user / '.config'),
               CLAUDE_CONFIG_DIR=str(user / '.claude'), GRID_HOME=str(home),
               GRID_PROJECTS=str(home / 'projects'), CLAUDE_PLUGIN_DATA=str(home / 'plugin-data'),
               CLAUDE_PLUGIN_ROOT=str(source), CLAUDE_ENV_FILE=str(box / 'claude.env'),
               GRID_BRIDGE='off', GRID_MEMBER='member', GRID_HOST='example.com', GRID_ROLE='builder',
               CLAUDE_CODE_ENTRYPOINT='cli', CLAUDE_CODE_SESSION_ATTENDED='1',
               NODE_OPTIONS='--import=' + str(ROOT / 'recordings/clock.mjs'))
    for key, value in case.get('env', {}).items():
        if value is None:
            env.pop(key, None)
        else:
            env[key] = expand(value)

    def invoke(hook, value, call=None):
        raw = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False, indent=2) + '\n'
        script = source / 'hooks' / (('lib' if hook.startswith('lib-') else hook) + '.mjs')
        command = [node, str(script)]
        if call:
            command = [node, str(ROOT / 'recordings/invoke.mjs'), str(script), call]
        result = subprocess.run(command, input=expand(raw).encode(), stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, cwd=workspace, env=env, timeout=10)
        return raw, result

    for step in case.get('pre', []):
        _, pre = invoke(step['hook'], step['input'])
        assert pre.returncode == 0 and not pre.stderr, (case['name'], 'prelude failed')
        assert not pre.stdout or case.get('pre_stdout'), (case['name'], 'unexpected prelude output')
    patch(case.get('after_pre', {}))

    def snapshot():
        result = {}
        for at in sorted(box.rglob('*')):
            label = str(at.relative_to(box))
            if at.is_symlink():
                result[label] = ('link', os.readlink(at).encode(), at.lstat().st_mode & 0o777)
            elif at.is_file():
                result[label] = ('file', at.read_bytes(), at.stat().st_mode & 0o777)
        return result

    before = snapshot()
    raw_input, result = invoke(case['hook'], case['input'], case.get('call'))
    after = snapshot()
    label = case['hook'] + '/' + case['name']
    assert result.returncode == case.get('exit', 0), (label, result.returncode, result.stderr)
    if 'contains' in case:
        assert case['contains'].encode() in result.stdout, (label, result.stdout)
    else:
        assert not result.stdout or case.get('call'), (label, 'unexpected stdout', result.stdout)
    if 'contains_stderr' in case:
        assert case['contains_stderr'].encode() in result.stderr, (label, result.stderr)
    else:
        assert not result.stderr, (label, result.stderr)
    changes = [name for name in sorted(before.keys() | after.keys()) if before.get(name) != after.get(name)]
    for expected in case.get('changed', []):
        assert any(expected in name for name in changes), (label, 'missing change', expected, changes)
    if not case.get('changed'):
        assert not changes, (label, 'unexpected file changes', changes)
    chunks = []
    for name in changes:
        old, new = before.get(name), after.get(name)
        chunks.append(f'diff --grid a/{name} b/{name}\n')
        if old is None:
            chunks.append(f'new {new[0]} mode {new[2]:04o}\n')
        elif new is None:
            chunks.append(f'deleted {old[0]} mode {old[2]:04o}\n')
        elif old[0::2] != new[0::2]:
            chunks.append(f'mode/type {old[0]} {old[2]:04o} -> {new[0]} {new[2]:04o}\n')
        lines = difflib.unified_diff(normalize(old[1]).splitlines(keepends=True) if old else [],
                                     normalize(new[1]).splitlines(keepends=True) if new else [],
                                     fromfile='a/' + name if old else '/dev/null',
                                     tofile='b/' + name if new else '/dev/null')
        for line in lines:
            chunks.append(line if line.endswith('\n') else line + '\n\\ No newline at end of file\n')
        if old and new and old[1] != new[1] and normalize(old[1]) == normalize(new[1]):
            chunks.append('Only normalized values changed.\n')
    target = destination / case['hook'] / case['name']
    target.mkdir(parents=True)
    artifacts = {'input.json': raw_input, 'env.txt': ''.join(f'{k}={normalize(v)}\n' for k, v in sorted(env.items())),
                 'stdout': normalize(result.stdout), 'stderr': normalize(result.stderr),
                 'exit': str(result.returncode) + '\n', 'files.diff': ''.join(chunks)}
    for name, content in artifacts.items():
        (target / name).write_bytes(content.encode())
    shutil.rmtree(box)


def index_text(source, node):
    lines = ['# Old JavaScript hook recordings', '',
             'Generated by `./record.sh`; see `recordings/README.md` for replay, setup and normalization.', '',
             'Every case has input.json, env.txt, stdout, stderr, exit and files.diff. Empty output files mean zero bytes.', '',
             '## Cases', '']
    lines.extend(f"- `{c['hook']}/{c['name']}`: {c['proves']}" for c in CASES)
    lines += ['', '## Limits and unrecorded behavior', '',
              '- No hook script is omitted. terminal-task has no direct stdin entrypoint; its extra helper cases use the exported terminalTask function.',
              '- needs-you: successful network notification delivery is not recorded; the live Grid server and authenticated desktop bridge are absent. Disabled and missing bridge paths are recorded.',
              '- permission-forward: remote allow, deny, timeout and authenticated bridge responses are not recorded; they require a server or bridge. Local fallback paths are recorded.',
              '- grid-env: desktop config discovery is recorded from a synthetic config; launching or communicating with a desktop application is not recorded.',
              '- heartbeat and worker-exit: local feed writes are recorded; orchestration and remote workers are outside this corpus.',
              '- These are observations of the old implementation, including its limitations; they do not decide which behavior the Go design retains.', '',
              '## Reference provenance', '',
              'Runtime: `' + subprocess.check_output([node, '--version'], text=True).strip() + '` on Linux.', '',
              'SHA-256 of reference files read by these cases (paths relative to the old plugin):', '']
    # Include transitive dependencies and agent descriptions without copying their source.
    for folder in ['hooks', 'agents', 'brain', 'feed', 'skills/task/scripts', 'skills/plan/scripts']:
        for file in sorted((source / folder).rglob('*')):
            if file.is_file():
                lines.append(f'- `{file.relative_to(source)}`: `{hashlib.sha256(file.read_bytes()).hexdigest()}`')
    return '\n'.join(lines) + '\n'


if __name__ == '__main__':
    main()
