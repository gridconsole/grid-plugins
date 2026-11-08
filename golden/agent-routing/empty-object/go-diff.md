The old script printed its agent catalog without reading stdin. Go prints the
combined context and now writes claude.env. The fixture's temporary workspace
resolves to Personal; its legacy scalar `folder` is not a per-machine Go project
path (internal/project/project.go:6). See docs/lanes/L10.md:50 and docs/old-plugin-map.md:125.
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
docs/lanes/L10.md:21. Exit status remains zero and stderr remains empty.
