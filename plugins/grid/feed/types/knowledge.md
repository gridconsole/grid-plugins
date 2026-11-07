---
type: knowledge
version: 1
who: any
required: [title, text]
fields:
  - title: a short name for what was learned
  - text: what was learned, in a few lines
  - kind: fact, decision, procedure or type (a proposed new post type)
  - concepts: optional, comma-separated names of the brain notes it relates to
---
Something the brain should keep. A Worker never writes the brain itself; it posts this.

Orchestrator: hands it to the Brain agent, the only writer of the brain. A knowledge post of kind `type` proposes a type note under the project's brain.

The Brain agent runs `GRID_ROLE=brain grid-brain knowledge --task <id>`: each post of kind fact, decision or procedure becomes its own note under `research/`, linked to the task's concept note and to `concepts`.
