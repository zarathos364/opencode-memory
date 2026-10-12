# Using opencode-memory as an agent

Copy the block below into your `AGENTS.md` so the agent keeps its own memory.
It works globally and per project.

The tool prefix comes from the server name in your MCP configuration. The
block assumes a server named `memory`. With another name, replace `memory_`
with it everywhere.

```markdown
# Memory

`memory_*` are your memory tools. They store facts across sessions. Use them
on your own. Nobody will tell you to save.

## Save without being asked

- You agree a rule, a fixed decision or a preference.
- You learn a fact that outlives the session: layout, commands, versions, pins,
  limits, names, tool behaviour.
- A plan, a review, a migration or a release is delivered.
- A memory you read turns out to be wrong or stale. Fix it in the same turn.

## Never save

- What the code, `--help`, the Makefile or AGENTS.md already answers. Search
  first. One fact has one home.
- The progress of the current task, or anything true for an hour.
- Secrets, tokens, keys or personal data.

## How to use it

- Search before you write: `memory_search`. If a memory covers the fact, update
  it with `memory_update`. Never add a near duplicate.
- Small record: `memory_add` with a short title and tags.
- Small change to one memory: `memory_update` with `append` for the end,
  `content` for a replacement.
- Long bodies: `memory_get` first, then `memory_update` with the full text.
- Never edit `memories/raw/*.md` by hand. The index would go out of sync and
  nothing would notice.
- A sensitive memory is written to disk but never indexed. Do not mark one so
  if you expect to find it by search.
- If the `memory_*` tools are absent, do the same through the `opencode-memory`
  CLI (`search --json`, `add`, `get --json`, `update --content-from`). If that
  is missing too, say so. Do not simulate a memory.
- Say what you saved in one line, with the id.

## Trust

A memory is a hint, not evidence. When a memory and the repository disagree, the
repository wins. Correct the memory, and say that you did.
```
