# opencode-memory

[![Go Reference](https://pkg.go.dev/badge/github.com/zarathos364/opencode-memory.svg)](https://pkg.go.dev/github.com/zarathos364/opencode-memory)
[![License: BSD-3-Clause](https://img.shields.io/badge/license-BSD--3--Clause-blue.svg)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/zarathos364/opencode-memory)](go.mod)

A command-line memory store for agents and developers. Memories are plain
markdown files with a YAML header. They stay on disk, so they stay readable,
diffable and versionable. A full-text index next to them makes them searchable.

The stored files are the source of truth. The index is a derived view that can
always be rebuilt, so a damaged or drifted index never costs you data.

## Contents

- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Quickstart](#quickstart)
- [Output and behavior](#output-and-behavior)
- [Store location](#store-location)
- [Memory file format](#memory-file-format)
- [Search queries](#search-queries)
- [Sensitive memories](#sensitive-memories)
- [Consistency and concurrency](#consistency-and-concurrency)
- [MCP server](#mcp-server)
- [Problems and fixes](#problems-and-fixes)
- [Development](#development)
- [License](#license)

## Overview

`opencode-memory` gives an agent a durable, local long-term memory without a
database server or a service. Each memory is a single `.md` file you can open
in any editor and commit to git. A Bleve index sits next to the files and makes
`search` fast, while `reindex` can rebuild it from the files at any moment.

It serves one project and every project at once. By default the store lives
in the `opencode` directory of your user configuration, so a single store
covers every project you work on. A project that already holds a store keeps
it, and the tool finds it from any subdirectory. A stale configuration never
starts a second store by accident.

## Features

- **Memories as markdown.** Human-readable files with a small YAML header and a
  markdown body, written atomically so a reader never sees a half-written file.
- **Full-text search.** Title-weighted ranking over title and body, with support
  for explicit field queries.
- **Private memories.** A `sensitive` memory is stored on disk but never indexed,
  so it can never surface in search results.
- **Rebuildable index.** The index is always derived from the files. `reindex`
  restores it from scratch after any loss or drift.
- **Exclusive index lock.** The index directory is locked per process, and a
  second instance reports a clear conflict instead of corrupting state.
- **Scriptable output.** `--json` on `search` and `list` for machine-readable
  results.

## Installation

Install the latest release into your `GOBIN`:

```sh
go install github.com/zarathos364/opencode-memory/cmd/opencode-memory@latest
```

Or install from a local checkout:

```sh
make install
```

Verify the install:

```sh
opencode-memory --version
```

## Quickstart

This session stores a memory, finds it again, and deletes it.

```sh
$ opencode-memory add --title "Go module layout" --tags go,arch \
    "Keep cmd thin and put logic in internal packages."
01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47

$ opencode-memory search "module layout"
score=1.2837  id=01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47  Go module layout

$ opencode-memory list
2026-10-11T12:00:00Z  id=01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47  Go module layout

$ opencode-memory get 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
id: 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
title: Go module layout
created: 2026-10-11T12:00:00Z
tags: go, arch
sensitive: false

Keep cmd thin and put logic in internal packages.

$ opencode-memory delete 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
deleted 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
```

## Output and behavior

`opencode-memory COMMAND --help` lists each command's flags. What follows is
what the commands actually print, and the behavior the help does not describe.
Flags work before or after the positional arguments.

### add

`--title` and the memory text are required. Prints the new memory's id.

```sh
$ opencode-memory add --title "Go module layout" --tags go,arch \
    "Keep cmd thin and put logic in internal packages."
01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
```

Tags are trimmed, lowercased and de-duplicated before they are stored.

### search

Find memories by text. Hits are returned strongest first.

```sh
$ opencode-memory search "module layout"
score=1.2837  id=01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47  Go module layout
```

```sh
$ opencode-memory search --json "module layout"
[
  {
    "id": "01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47",
    "created_at": "2026-10-11T12:00:00.123456789Z",
    "title": "Go module layout",
    "tags": [
      "go",
      "arch"
    ],
    "sensitive": false,
    "content": "Keep cmd thin and put logic in internal packages.",
    "score": 1.2837
  }
]
```

### list

List memories, oldest first. Sensitive memories are excluded unless you ask
for them.

```sh
$ opencode-memory list
2026-10-11T12:00:00Z  id=01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47  Go module layout
```

### get

Print one memory with its header fields and body.

```sh
$ opencode-memory get 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
id: 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
title: Go module layout
created: 2026-10-11T12:00:00Z
tags: go, arch
sensitive: false

Keep cmd thin and put logic in internal packages.
```

### update

Rewrite a memory while keeping its id and creation time. Pass `--title`,
`--tags` or `--sensitive` to replace those fields, and extra positional text to
replace the body.

```sh
$ opencode-memory update 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47 \
    --tags go,testing "Keep cmd thin, put logic in internal packages, cover it with tests."
01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
```

The command needs at least one field or new text.

### delete

Delete one memory and its index entry.

```sh
$ opencode-memory delete 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
deleted 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
```

### reindex

Rebuild the index from the stored files. This is the single way back from a
lost, damaged or drifted index, because the files are the source of truth.

```sh
$ opencode-memory reindex
reindexed 3 memories
```

The count reports how many memories are searchable. It skips sensitive
memories.

## Store location

Every command accepts `--root DIR`. Without it, the tool resolves the store
in this order. It uses `OPENCODE_MEMORY_ROOT` when you set it. Otherwise it
walks up to the closest `.opencode` directory that holds a memory file. A
project folder that only carries the `.opencode` name is a setting, not a
store, so the walk up skips it. A store left empty by a probe is skipped too.

When no project store is found, the tool uses the `opencode` directory under `$XDG_CONFIG_HOME`, or
under `$HOME/.config` when that variable is not set.

The tool refuses an explicit root that does not exist, so a stale setting can
never start a second store by accident. Only the automatic fallback resolves
to a directory that does not exist yet. The tool creates it on first use.

Inside the root the layout is fixed:

```
<root>/memories/raw/<uuid>.md      # the memories themselves
<root>/memories/index/index.bleve  # the search index
```

## Memory file format

Each memory is a markdown file with a YAML header followed by the body:

```markdown
---
schema_version: 1
id: 01935c2a-7f4e-7c3b-9a10-2f6d8e5b1c47
created_at: "2026-10-11T12:00:00.123456789Z"
tags:
    - go
    - arch
sensitive: false
title: Go module layout
---

Keep cmd thin and put logic in internal packages.
```

- **`schema_version`** is the header version this build understands.
- **`id`** is a UUIDv7, so sorting identifiers yields creation order.
- **`created_at`** is UTC in RFC 3339 form.
- **`tags`** hold a list. An empty list is valid.
- **`sensitive`** tells whether the memory stays out of the index.
- **`title`** must be a single non-empty line.

The body follows the header. It must not be empty.

Files are written to a temporary name and renamed only once complete, so an
interrupted run leaves no broken memory behind.

## Search queries

A plain query matches against the title and the body, with the title weighted
higher. A query that already names a field goes to the index unchanged, so
you can search by a single field:

```
tags:go
title:layout
content:packages
```

Recognised fields are `title`, `content`, `tags`, `sensitive`, `created_at`
and `id`. Anything that does not name one of these fields is treated as plain
text.

## Sensitive memories

`--sensitive` stores a memory and keeps it out of the index. The file stays on
disk and stays reachable through `get`, `list --sensitive` and `update`, but it
never appears in `search` results. When you mark an existing memory as
sensitive, the next write deletes it from the index.

## Consistency and concurrency

The files on disk are the source of truth. The index is a derived view.
Writes apply the file first and the index second. An index failure leaves
a memory that still exists. `reindex` can make it searchable again.

Delete works the same way. The file goes first, so a failure leaves a stale index
entry, not an unrecoverable memory.

If the two drift apart, `search` and `list` print a warning to stderr and
continue, because the stored files are still readable:

```
warning: the search index is out of sync, run 'opencode-memory reindex'
```

A single process holds an exclusive lock on the index directory. A second
instance that opens the same store at the same time reports that another
instance already holds the store. This prevents corruption.

## MCP server

`opencode-memory serve` exposes the store to agents as MCP tools over stdio.
It serves one store per process and opens it for each tool call, so the CLI
keeps working while the server runs. Every tool returns JSON in the first
content part, and a failure sets `isError` with the message in the same place.

Configure it in `opencode.jsonc`, globally or per project:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "memory": {
        "type": "local",
        "command": ["opencode-memory", "serve"],
        "codemode": false
      }
    }
  },
  "permissions": [
    { "action": "memory_*", "resource": "*", "effect": "allow" }
  ]
}
```

The seven tools are `memory_add`, `memory_search`, `memory_list`,
`memory_get`, `memory_update`, `memory_delete` and `memory_reindex`. They take
the same fields as the matching commands. Copy `docs/MCP_INSTRUCTION.md`
into your `AGENTS.md` so the agent keeps its own memory. The
server shows up in `opencode mcp list` once configured.

## Problems and fixes

| Symptom                                        | Cause and fix                                                                        |
|------------------------------------------------|--------------------------------------------------------------------------------------|
| `another instance is using this store`         | Another process holds the index lock. Close it, or wait for it to finish.            |
| `the search index is out of sync` (warning)    | The index no longer matches the files. Run `opencode-memory reindex`.                |
| A memory is on disk but not found by `search`  | It is marked sensitive, or the index drifted. Check with `list --sensitive`, then `reindex`. |
| `root from ... is not usable`                  | An explicit `--root` or `OPENCODE_MEMORY_ROOT` points to a missing path.             |

## Development

Some targets need extra tools. Run `make install-tools` to install them.
It uses the versions pinned in `.github/env`. The `Makefile` holds every
target, and each one runs as `make TARGET`.

Before you push a change, run `make fmt`, `make lint`, `make vet`,
`make test` and `make vuln-check`.

## License

BSD 3-Clause. See [LICENSE](LICENSE).
