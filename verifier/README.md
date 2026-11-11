# Verifier

A read-only role that checks an implementation against its plan and returns a
verdict, its findings, and what it could not check. It changes nothing.

## What it does

On its own, the verifier runs only when someone asks for it: it is a role you
can dispatch, nothing more. A workspace can let it run during execution by
allowing one of the two overrides it declares:

| Override | What changes |
| --- | --- |
| `verify-during-execution` | After each implementation, the checker runs once and its result is reported beside the implementation. Nothing is fixed because of it. |
| `repair-loop` | After each implementation, the checker runs; on findings a worker fixes them and the checker runs again, until it is clean or the round limit is reached. Up to 3 rounds. |

Only a person allows an override, and only for their workspace.

## What it needs

- Context Circuit CLI 2.6.0 or later.
- For the repair loop, a host that can start a subagent: the checker must not
  be the session that wrote the code.
- No scripts, settings or credentials.

## Add it

```sh
sh .context-circuit/cli extension add --id verifier \
  --url https://github.com/kaotypr/context-circuit-extensions.git \
  --ref verifier/v1.0.0 --subpath verifier
sh .context-circuit/cli agent setup
```

`agent setup` installs the role as `x-verifier-checker` for every host. Reload
the host's agents if it does not see the new role in the current session.

Dispatch it on request:

```sh
sh .context-circuit/cli agent dispatch --host HOST --role x-verifier-checker --plan PLAN --path WORKTREE
```

To run it during execution, allow one override:

```sh
sh .context-circuit/cli extension override --id verifier --allow verify-during-execution
sh .context-circuit/cli extension override --id verifier --allow repair-loop --rounds 2
```

`--rounds` may lower the limit and never raise it. `--revoke` takes an
allowance back.

## What it returns

- **Verdict**: `clean` or `findings`.
- **Findings**: file and line, what the plan expected, and what the diff does.
  A finding about the plan's outcome itself is marked for the person.
- **Not checked**: what it could not verify, and why.
