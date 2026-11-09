The recorded script appended to history.md in place. Since FP1 (2792be12),
AppendHistory writes the new history.md to a temporary file and renames it into
place, so a hard link planted on the old file can never be rewritten through.
Appenders therefore serialize on the task lock, not on the history.md they
replace: AppendHistory creates and locks `.task.lock` in the task folder
(internal/task/history.go:117), the same lock the journal writers, trash, the
feed and the orchestrator already take there, first in the lock order
(docs/delivery.md:125). The lock is an empty file, mode 0600; the task-folder
readers skip dot names (internal/hook/history.go:173), and the FP1R4 section of
docs/lanes/FP1.md lists the readers checked.

The rename keeps history.md's mode (internal/task/create.go:400), so the
recorded diff has no mode change. Exit status, stdout and stderr are unchanged.
