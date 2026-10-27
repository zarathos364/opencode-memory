---
name: opencode-memory
description: Use to save, consult or change memories on request, to recover a fact, or after the user agrees a decision.
---

# Memory

The server binary is `opencode-memory`. The tool prefix comes from the name
you give the server in the configuration. With a server called `memory`, the
tools are `memory_search`, `memory_add` and so on.

## Tools

- `memory_search` — search by text, best matches first. Takes `query` and an
  optional `limit`. Every meaningful word must match; a word the index drops,
  such as `the`, does not count. A result that matched only part of the query is
  marked `weak`, and an empty list means nothing matched. Returns `id`,
  `created_at`, `title`, `tags`, `sensitive`, `score` and `weak`. It does
  **not** return the body.
- `memory_list` — lists memories, oldest first. Without `sensitive: true`,
  sensitive ones stay out. It does not return the body either.
- `memory_get` — returns one memory with the whole body. It takes the `id` and
  is the only way to read a stored body.
- `memory_add` — stores a new record. `title` and `content` are required.
  `tags` and `sensitive` are optional.
- `memory_update` — changes an existing record. It takes the `id` and a patch:
  `content` replaces the body, `append` adds text at the end, and `title`,
  `tags` and `sensitive` replace the fields. One call takes one body source.
- `memory_delete` — removes a memory. It takes the `id`.
- `memory_reindex` — rebuilds the index from the stored files.

## Before you store

Your instructions decide when a memory is wanted. The gate below decides
whether the write happens. All of these must hold before `memory_add` or
`memory_update`:

- The fact is still true and useful at the start of the next session.
- The answer is not already in the code, `--help`, the Makefile or `AGENTS.md`.
- It holds no secret, token, key or personal data.
- You ran `memory_search` before writing.
- A near match on the same fact becomes an update, not a second memory.
- The title states the fact. `search` and `list` show titles and no body.

## Rules

- Never edit `memories/raw/*.md` by hand. A hand edit of the text leaves the
  index stale. The drift check only counts files, so it cannot see the edit and
  the tools print no warning.
- Read the body with `memory_get` before `memory_delete`. A title alone can
  name the wrong memory.
- A memory marked `sensitive` never appears in search. `get` returns its body.
  `list` with `sensitive: true` returns its summary, and `add` and `update`
  return the record they just changed.
- `sensitive` hides a record from search. It is not encryption. Never store a
  secret in a sensitive memory.
- Without the `memory_*` tools, use the CLI: `opencode-memory search --json`,
  `add`, `get --json`, `update --append` or `update --content-from`. `add`
  needs `--title`. `get` and `update` need the id. `--help` lists the flags.
  If the CLI is not installed either, say so. Do not simulate a memory.
- Say what you stored in one line, with the id.
