---
type: regex
pattern: '"name"\s*:\s*"(?:Write|Edit|MultiEdit|NotebookEdit)"\s*,\s*"input"\s*:\s*\{\s*"(?:file_path|notebook_path)"\s*:\s*"(?:[^"\\]|\\.)*(?<!/proposals\.md)"'
match: not_contains
target: trace
---
