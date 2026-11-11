# Context Circuit extensions

Official extensions for [Context Circuit](https://github.com/kaotypr/context-circuit-source)
workspaces. A workspace pins one at a tag and gets the same files on every
machine; nothing here runs until a workspace adds it.

| Extension | What it shows | Current tag |
| --- | --- | --- |
| [verifier](verifier/) | Changing what an agent does: a read-only role that checks an implementation against its plan, and may run during execution or in a repair loop where a person allows it | `verifier/v1.0.0` |
| [token-usage](token-usage/) | Adding steps around a stage: records the tokens each dispatched subagent used, per plan, and totals them after implementing | `token-usage/v1.0.0` |

Each extension's README says what it does, what it needs, and how to add it.

## Adding one

From a workspace root, pin the extension's tag and folder:

```sh
sh .context-circuit/cli extension add --id verifier \
  --url https://github.com/kaotypr/context-circuit-extensions.git \
  --ref verifier/v1.0.0 --subpath verifier
```

Then follow the extension's README: run `agent setup` for one that ships a
role, and `extension allow` on each machine for one that ships a script.
`check` validates what was added. All of them need Context Circuit CLI 2.6.0
or later.

## Versions

Each extension is versioned on its own. Its version is the `version` in its
`extension.yaml`, its changes are in its `CHANGELOG.md`, and each release is a
tag `<extension>/vX.Y.Z`. A workspace follows a new version with
`extension update --id ID --ref <extension>/vX.Y.Z`, only when a person asks.

## What every extension here keeps to

- It follows the extension contract of the Context Circuit release it
  requires, and passes `check`.
- It never pushes, opens a pull request, approves anything, or edits a plan
  record.
- A script runs only after `extension allow`, ships a Windows command beside
  its Unix one, and needs no credential.

## Tests

The tests check this repository only: every manifest through a real CLI, each
script against sample input, and the published text.

```sh
CC_CLI=/path/to/context-circuit-cli go test ./...
```

`CC_CLI` names a Context Circuit CLI binary, 2.6.0 or later. CI runs the same
tests on Linux, macOS and Windows against the release named in
`.github/workflows/test.yml`.

## License

[Apache-2.0](LICENSE)
