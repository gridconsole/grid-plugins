---
type: llm
weight: 1
---

Pass when a `grid job post --type found` call in the transcript has a body with kind `security` and an `at` that names `src/auth.go` and the line of the `log.Printf` call that writes the token (line 11), and the body says the session token is written to a log. Fail when no found record names the file and line, when its kind is not `security`, or when the hole is only mentioned in the verdict text.
