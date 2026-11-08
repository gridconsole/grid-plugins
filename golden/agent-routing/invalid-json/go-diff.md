The old agent-routing script ignored stdin and printed its agent catalog
(docs/old-plugin-map.md:125). The Go SessionStart handler parses the event and
silently returns on malformed JSON (docs/lanes/L10.md:21 and
docs/lanes/L10.md:56). Expected stdout, stderr and files.diff are empty; exit is
zero. No fixture files change.
