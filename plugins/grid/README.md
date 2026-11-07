# Grid plugin

Task folders, a planner per task, plan pages built from a component library.

Without the Grid app there is no Accept button: to let a session write past its plan, restart Claude Code with `GRID_PLAN_GUARD=off`.

The plugin grants its own Grid home (the data folder Grid uses for this machine) and its own skill scripts through the `grid-home-allow` hook, so nothing in settings.json is needed; a deny or ask rule and the other guards still win.

`remote/sync.json` lists what the Grid app copies to and from a machine reached over SSH (task folder paths each way, what never copies, and how each agent starts there); the app reads it. `remote/trust-claude.mjs` is the one script beside it: the app sends its text to the machine and runs it with `node` before the agent starts there, so the checkout is trusted and the session does not wait at Claude Code's trust question.
