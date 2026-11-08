---
type: llm
focus:
  source: file
  path: ws/projects/notes-app/tasks/T-01-setup/proposals.md
---
The repo's .github/workflows/deploy.yml deploys a production web site (https://app.notes.example, netlify) and an api (flyctl, notes-api); fly.toml says the api is served at https://api.notes.example.
PASS if proposals.md holds a separate `kind: environment` entry for the web production site and another for the api production site (each with its own address), and each such entry's source names deploy.yml or fly.toml.
FAIL if they are merged into one entry, if either is missing, or if their source is only a session.
