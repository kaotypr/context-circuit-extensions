# Token usage

Records how many tokens each dispatched subagent used, per plan, and totals
them after implementing, so a team can see what delegation costs.

## What it does

- **Before planning, implementing and review**, the agent is told to write one
  entry for each subagent it dispatches. An entry has the time, the role, and
  the token count the host returned with the subagent's result. Where the host
  returned none, the entry says `not-available`.
- **After implementing**, a script totals the plan's entries per role, writes
  `<plan>-summary.md` beside them, and reports a one-line summary with the
  implementation.

Entries look like this, one line each, in `<folder>/<plan>.log`:

```text
at=2026-11-11T05:30:00Z role=worker tokens=48210 from=host-result
at=2026-11-11T05:41:12Z role=x-verifier-checker tokens=not-available from=none
```

While an intent is being planned, entries go in `<intent>.log`; a direct
change uses `direct.log`.

## What the numbers mean

**The totals cover only the entries the agent recorded.** No hook sees a
subagent's result, so the agent writes each entry, and an entry it forgets is
missing. Counts come only from what the host returns with a result; the agent
never reads a host's session files or estimates a number. A `not-available`
entry is counted as an entry and adds no tokens. A line the script cannot read
is named in the summary, never dropped silently.

`after implement` runs once for each repository of a plan, so a plan that
spans two repositories gets the summary twice. Both runs total the same log,
so they agree.

## Settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `folder` | `.context-circuit/local/token-usage` | Where entries and summaries go, relative to the workspace root |

The default folder is ignored by Git, so the entries stay on the machine. Point
`folder` at a committed folder to share them. A folder that is absolute or
leaves the workspace is refused.

## What it needs

- Context Circuit CLI 2.6.0 or later.
- `sh` and `awk` on macOS and Linux, Windows PowerShell on Windows.
- No credentials. The script reads and writes only inside the workspace.

## Add it

```sh
sh .context-circuit/cli extension add --id token-usage \
  --url https://github.com/kaotypr/context-circuit-extensions.git \
  --ref token-usage/v1.0.0 --subpath token-usage
sh .context-circuit/cli extension allow --id token-usage
```

The summary is a script, so it runs only after each member allowed this version
on their own machine with `extension allow`. Until then the hook is skipped
and says so; the entries are still written.

To use another folder, add it to the entry in `extensions.yaml`:

```yaml
  - id: token-usage
    settings:
      folder: docs/token-usage
```
