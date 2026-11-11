# Research

- FACT: retry.go gives up after 3 attempts (retry.go:4).
- UNVERIFIED: the notes say the upload queue is flushed every 30 seconds by FlushQueue; no FlushQueue or timer exists in repo/ (searched, only repo/upload.go:3 mentions flush, to say it does not).
