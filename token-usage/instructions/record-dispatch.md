# Record each dispatch

For this stage, write one entry each time a subagent you dispatched returns.

## Where

The folder is this extension's `folder` setting, relative to the workspace
root, or `.context-circuit/local/token-usage` when the setting is absent.
Create it if it is missing. Never write outside the workspace.

Append the entry to `<folder>/<record>.log`, where `<record>` is:

- the plan's ID while implementing or reviewing a plan;
- the intent's ID while planning it, or `standalone` for a standalone plan;
- `direct` for a direct change.

## What

One line per dispatch, in this form:

```text
at=2026-11-11T05:30:00Z role=worker tokens=48210 from=host-result
```

- `at`: when the subagent returned, as an ISO 8601 UTC timestamp.
- `role`: the role you passed to `agent dispatch`, such as `worker` or
  `x-verifier-checker`.
- `tokens`: the total tokens the host returned with the subagent's result, as a
  whole number with no separators.
- `from`: `host-result` when that number came with the result.

Where the host returned no token count, write `tokens=not-available from=none`.

## Limits

- Copy only the number the host gave you with the result. Never estimate, add
  up messages yourself, or read the host's session files, logs, or settings to
  find one.
- Write the entry even when the subagent failed.
- These entries are local bookkeeping. They never go into a plan record, a
  commit, or a pull request.
