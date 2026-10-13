# Using opencode-memory as an agent

Two pieces, both shipped in this repository. Configure the MCP server under
the name `memory` so the tools are `memory_add`, `memory_search` and so on.
The skill at `.opencode/skills/opencode-memory/` documents the tools. Install
it with:

```sh
mkdir -p ~/.config/opencode/skills
cp -r .opencode/skills/opencode-memory ~/.config/opencode/skills/
```

The skill alone makes the agent save only on request: its description stays in
context and the body loads when the task matches. Proactive saving needs rules
that are always in context. Paste the block below into your `AGENTS.md` for
that.

The tool prefix comes from the server name in your MCP configuration. The block
assumes a server named `memory`. With another name, replace `memory_` with
that name plus `_` everywhere.

```markdown
# Memory

`memory_*` are your memory tools. They carry facts across sessions. Use them
on your own. Nobody will tell you to save.

## Save without being asked

- You agree a rule, a fixed decision or a preference.
- You learn a fact that outlives the session: layout, commands, versions,
  pins, limits, component and tool names, tool behaviour.
- A plan, a review, a migration or a release is delivered.
- A memory you read turns out to be wrong or stale. Fix it in the same turn.

## Never save

- What the code, `--help`, the Makefile or AGENTS.md already answers. Check the
  code first. Then search the memory store. One fact has one home.
- The progress of the current task, or anything true for an hour.
- Secrets, tokens, keys or personal data.

## How to use

- Load the skill `opencode-memory` before the first memory operation: it
  documents the tools and their arguments.
- Search the store before you answer a question about this project. A stored
  fact may already answer it.
- Use the tools when a trigger above fires or when the user asks to remember,
  save or note something.
- Use them also to recover a saved fact and to fix a memory that is wrong or
  stale.
- Pass the gate in the skill before `memory_add` or `memory_update`.
- If the skill does not load, do not write memories until it is installed.
- If no memory tool and no CLI are available, say so. Never pretend a memory
  was saved.
- Never edit `memories/raw/*.md` by hand.
- A memory is a hint, not evidence. When it and the repository disagree, the
  repository wins. Correct the memory and say that you did.
- Say what you stored in one line, with the id.
```
