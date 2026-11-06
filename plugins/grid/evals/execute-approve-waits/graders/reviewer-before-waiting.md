---
type: regex
pattern: '^(?:(?![^\n]*"name":\s*"(?:Write|Edit|MultiEdit)"[^\n]*status: waiting)(?![^\n]*"name":\s*"Bash"[^\n]*(?:sed -i|\btee\b|>>?|\bpython3?\b|\bnode\b|\bperl\b)[^\n]*status: waiting)[^\n]*\n)*(?![^\n]*"name":\s*"(?:Write|Edit|MultiEdit)"[^\n]*status: waiting)(?![^\n]*"name":\s*"Bash"[^\n]*(?:sed -i|\btee\b|>>?|\bpython3?\b|\bnode\b|\bperl\b)[^\n]*status: waiting)[^\n]*"subagent_type":\s*"(?:grid:)?grid-reviewer"'
target: trace
---
