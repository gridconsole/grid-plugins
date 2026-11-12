The absence of desktop configuration no longer suppresses output. Go resolves
~/.grid and Personal for this temporary workspace, prints context, and exports
the default home and projects path while unsetting GRID_TASK. This is the
single-home, desktop-independent behavior in docs/lanes/L10.md:50 and
docs/lanes/L10.md:51; the old discovery path is docs/old-plugin-map.md:31.
The legacy script is replayed as `grid hook sessionstart`. The consolidated
SessionStart contract is documented in docs/lanes/L10.md:21 and
docs/lanes/L10.md:49. Context includes the resolved project/task, the compact
agent list and the terminal approval path (docs/lanes/L10.md:23,
docs/lanes/L10.md:52, docs/lanes/L10.md:53).

The reviewed bytes include Go's JSON key order (`additionalContext` before
`hookEventName`) from internal/hook/hook.go:77 and internal/hook/hook.go:80.
No JSON normalization hides that difference. The shell file has a leading blank
line, quoted exports, a task export or unset, and mode 0600, as implemented in
internal/hook/hook.go:92 and internal/hook/hook.go:100 and described in
docs/lanes/L10.md:21. Exit status remains zero. Stderr is no longer empty: it holds
`grid hook: agent sync: open $OLD_PLUGIN/agent-templates: no such file or directory`.
The hook syncs agent templates from `$CLAUDE_PLUGIN_ROOT/agent-templates`
(internal/hook/hook.go:167-179, internal/settings/agentsync.go:23); the corpus rig
sets CLAUDE_PLUGIN_ROOT to the inert `$OLD_PLUGIN` token folder, which has no
agent-templates, so the failure is only logged and never reaches the session.
The installed plugin has that folder, so this is a gap in the rig, not a product bug.
