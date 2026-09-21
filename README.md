# mosoo-connector

**AI agent CLI and coding-agent Skill for [mosoo](https://mosoo.ai).**

The generated `mosoo` CLI creates durable Sessions directly from Project credentials and harness/model/instructions, with saved Agents available as optional presets. It also configures, publishes, and inspects Agents through the [mosoo API and control plane](https://github.com/langgenius/mosoo). Use it from a terminal or install the bundled Skill so coding agents can discover the same commands. It supports mosoo Cloud, local development, and self-hosted targets.

This repository is mosoo client tooling, not an Agent runtime or a general-purpose OpenAPI generator.

## Quick Start

```sh
go install github.com/langgenius/mosoo-connector/cmd/mosoo@latest
mosoo setup
mosoo auth login
mosoo doctor --json
mosoo ls -o json
```

## Development Build

```sh
make build
```

This checks out the full Mosoo commit recorded in `specs/mosoo-contract.json`
under `.cache/mosoo`, exports OpenAPI / GraphQL
specs, renders `specs/sources.yaml` and `overlays/*.yaml`, runs Lathe code generation, and
builds `bin/mosoo`. Generated CLI command indexes are rendered into
`publish/skills/mosoo/references/cli/`; the CLI guide at
`publish/skills/mosoo/references/cli.md` is rendered from Lathe Skill include
resources under `publish/skills/mosoo/lathe-include/`; and the top-level mosoo
Skill entrypoint lives at `publish/skills/mosoo/SKILL.md`.

Lathe is managed by this repository. `make build` first compiles the pinned Lathe CLI from
`go.mod` into `.cache/bin/lathe`, then uses that local binary for code generation.

The same contract record is embedded into the CLI and bundled Skill. Query the
CLI copy with `mosoo doctor --json`; read the Skill copy at
`references/provenance.json`. Both include the full upstream Mosoo SHA and a
SHA-256 over canonical JSON (sorted object keys) for the Public Thread OpenAPI.

To refresh the contract, resolve and review a full Mosoo commit SHA, then run:

```sh
make build MOSOO_REF=<40-character-mosoo-commit>
make catalog-test
```

Commit the lock, source overlays, and all generated changes together. Pull
request CI and tag release run `make contract-gate`, which repeats that build
from the recorded commit and fails on a generated diff. Runtime/OpenAPI changes
land in `langgenius/mosoo` first; connector maintainers then refresh this lock,
generated CLI, and Skill before a connector release. Downstream docs should
consume the same upstream commit and normalized digest before publication.

Builds inject deterministic CLI version metadata from Git into Lathe's standard
`Version`, `Commit`, and `Date` fields:

```text
VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short=12 HEAD)
BUILD_DATE=$(git show -s --format=%cI HEAD)
```

Override `VERSION`, `COMMIT`, or `BUILD_DATE` for release builds when the
release pipeline has already computed those values.

Override the API host base baked into per-module defaults:

```sh
make build MOSOO_HOST_BASE=https://api.example.com
```

## Other Install Options

Install a specific release:

```sh
go install github.com/langgenius/mosoo-connector/cmd/mosoo@vX.Y.Z
```

`go install` downloads the module source and compiles it locally. It does not
run `make build`, so release tags include the generated CLI manifest and Go
command sources needed by `cmd/mosoo`. Make sure `go env GOBIN`, or
`$(go env GOPATH)/bin` when `GOBIN` is empty, is on `PATH`.

Install from a local checkout:

```sh
make install
```

By default, installation uses `go env GOBIN`, or `$(go env GOPATH)/bin` when
`GOBIN` is empty. Override the destination with `BINDIR`:

```sh
make install BINDIR="$HOME/.bin"
```

`make install` runs `make verify-install`, which checks that the installed
binary's `mosoo --version` output exactly matches the build metadata.

### Homebrew

After a release's generated Homebrew formula pull request is merged, tap this
repository and install the formula:

```sh
brew tap langgenius/mosoo-connector https://github.com/langgenius/mosoo-connector
brew install langgenius/mosoo-connector/mosoo
```

Release automation opens a GoReleaser pull request to add or update the
formula. Merged formula revisions install the matching prebuilt `mosoo` archive
from GitHub Releases.

## Installer

The source for the mosoo installer lives at `publish/installers/install.sh`.
The current stable public entrypoint is:

```sh
curl -fsSL https://install.mosoo.ai/install.sh | bash
```

The installer is interactive by default and asks for `y` or `n` before high-impact
steps. Use `--yes` for automation and `--dry-run` to preview the plan.
After a successful Skill copy, it retires the former
`$HOME/.codex/skills/mosoo` target when that path differs from the active
`$CODEX_HOME/skills/mosoo` target, preventing two stale Mosoo Skills from being
discovered at once.

## Non-production contract smoke

The manually dispatched `Non-production Public Thread smoke` workflow reads its
deployment URL, Agent ID, user ID, and environment label from the protected
`public-thread-smoke-non-production` GitHub environment and its token from an
environment secret. The script refuses `prod`/`production`, known production
Mosoo hosts, non-HTTPS targets, and environment labels that do not explicitly
identify a development, staging, preview, test, QA, sandbox, or non-production
deployment. It submits the minimal `{ "userId": "..." }` create shape, verifies
`thread.id`, and deletes the smoke Thread afterward.

## Published Skill layout

The publishable mosoo Skill is rooted at `publish/skills/mosoo`.

```text
publish/skills/mosoo/
|-- SKILL.md
`-- references/
    |-- setup.md
    |-- cli.md
    |-- api.md
    |-- provenance.json
    `-- cli/
        |-- catalog.md
        `-- modules/
```

`references/cli.md`, `references/cli/catalog.md`, and
`references/cli/modules/*.md` are generated from Lathe's CLI Skill output during
`make build`. To change the guide text in `references/cli.md`, edit the matching
Lathe include file under `lathe-include/`. Treat the module files as CLI command
indexes, not as the top-level mosoo Skill.

## Command layout

`cli.command_path` is `namespaced`: every generated command lives under its source module
(`console`, `console-rest`, `public-thread-api`, or `public-thread-api-v2`). Root-level flat mounting is not used
because the CLI ships multiple API surfaces.

Help text, examples, and error hints for generated commands come from `overlays/*.yaml`
(regenerated by `scripts/render-overlays.ts` during `make build`). The generated
catalog is the complete control-plane surface; overlays are usability polish, and
the mosoo Skill reference explains high-frequency workflows.

High-frequency root commands such as `mosoo ls`, `mosoo run`, `mosoo add-key`,
and `mosoo create-agent` are Lathe overlay `shortcuts` for generated operations.
They execute the same generated command specs as their canonical paths and are
reported in the generated command catalog.

## Hostnames and auth

Four API modules share one deployment but use different URL bases:

| CLI module | Default hostname (from `MOSOO_HOST_BASE`) | Example paths |
|------------|-------------------------------------------|---------------|
| `console`, `console-rest` | `{base}/api` | `/graphql`, `/access-tokens`, `/files` |
| `public-thread-api` | `{base}/api/v1` | Published-Agent compatibility API; `userId` required |
| `public-thread-api-v2` | `{base}/api/v2` | Project Session creation, optional Agent presets and `userId`, persisted `/threads/{id}/usage` |

Generated specs carry baked fallback hostnames from codegen (`MOSOO_HOST_BASE`).
Normal mosoo commands resolve a target before using those fallbacks. Override
any command with `--hostname` or `$MOSOO_HOST`.

## Direct Project Sessions (v2)

Use a deployment that advertises `GET /api/v2/openapi.json` and confirm its
available features before changing a production integration. The CLI keeps
the existing v1 commands unchanged and exposes v2 explicitly. Configure your
model provider credentials in the Project (BYOK), then save this as `session.json`:

```json
{
  "configuration": {
    "type": "inline",
    "harness": "openai-runtime",
    "provider": "openai",
    "model": "<model-id>",
    "instructions": "Analyze the supplied material and save the results."
  },
  "input": {
    "type": "user.message",
    "content": [{ "type": "text", "text": "Summarize the attachment." }]
  },
  "resources": [{ "type": "file", "file_id": "<uploaded-file-id>" }]
}
```

```sh
mosoo --target custom --base-url <service-origin> public-thread-api-v2 files upload --project-id <project-id> --file ./brief.txt -o json
mosoo --target custom --base-url <service-origin> public-thread-api-v2 threads create --project-id <project-id> --file session.json --idempotency-key <stable-key> -o json
mosoo --target custom --base-url <service-origin> public-thread-api-v2 events send --thread-id <thread-id> --file events.json --idempotency-key <new-turn-key> -o json
mosoo --target custom --base-url <service-origin> public-thread-api-v2 events wait --thread-id <thread-id> --final-output
mosoo --target custom --base-url <service-origin> public-thread-api-v2 threads usage --thread-id <thread-id> --limit 100 -o json
```

Use the uploaded `file.id` in `resources`; omit resources when no file is needed.
Project creation requires `configuration`. Choose `type: "inline"` with harness,
provider, model and non-blank instructions, or `type: "agent"` with `agent_id`
for an optional saved private preset. These modes cannot be mixed. Input and
`userId` are optional; omitted input creates an idle Session, omitted userId is
`null`. Inline execution creates no Agent. The admitted configuration stays
fixed; continue through the same returned `thread.id`.

A Project key cannot cross Projects. Account login also requires an explicit
owned `--project-id` on these operations. Reusing an idempotency key with a
changed configuration returns 409. The compatibility `threads create --agent-id`
and `files upload --agent-id` forms remain available; `--agent-id` and
`--project-id` cannot be combined. Platform model supply, top-ups and commercial
billing are separate work in [#636](https://github.com/langgenius/mosoo/issues/636).

All existing file upload, wait and transcript helpers work under both modules.
Usage preserves `null` for unreported values; `reportedCostUsd` is a runtime
estimate, not an invoice. Pass `nextCursor` as `--after` to paginate. Keep the
same idempotency key for a retry with an unchanged body, and a new key for a new
turn. Run `mosoo auth login` for the chosen target once after upgrading to add the
v2 credential. Configure Project keys against the explicit v2 hostname.
API commands never recreate credentials removed by logout.

The optional v2 budget extension is unreleased. Confirm that the target's
`/api/v2/openapi.json` includes `maxCostUsd` and a deployment budget policy
is configured before using it. Supply `maxCostUsd` as a top-level JSON number
in a complete `--file` body, or via `--set maxCostUsd=<usd-amount>` alongside
the initial `input` or a send request's `user_message` event. Choose a positive
USD amount with at most six decimal places, within the deployment maximum.
It applies only to that turn; omission uses a configured deployment default
when available. An explicit cap without policy returns `409 readiness_blocked`.
The CLI supplies no default amount and does not fund inference.

Budgeted responses expose `run.budget` (`capUsd`, `estimatedCostUsd`, `state`).
The estimate can exceed the cap for an in-flight request; reaching the
threshold blocks new model requests, and unknown usage fails closed. Native
provider protocols are unchanged. Budget failures keep available outputs
readable through file/event commands and `events wait` returns a failure;
they do not promise a successful checkpoint or completed `--final-output`.
See the [CLI budget recipe](publish/skills/mosoo/references/cli.md#per-turn-model-budget-unreleased)
for request examples and budget states.

## Target resolution

Generated API commands resolve a default target before falling back to baked-in hostnames.
Explicit hostname overrides always win:

```text
--hostname
  -> MOSOO_HOST
  -> --target / --base-url
  -> MOSOO_TARGET / MOSOO_BASE_URL
  -> project config .mosoo/config.json
  -> global config in the OS config dir (or $MOSOO_CONFIG_DIR/config.json)
  -> default mosoo Cloud target
```

Generated API commands default to mosoo Cloud when no target config exists:

```json
{
  "target": "cloud",
  "baseUrl": "https://cloud.mosoo.ai"
}
```

First-time cloud users should use the zero-config setup and login path:

```sh
mosoo setup
mosoo auth login
```

`mosoo setup` stores the root target (`https://cloud.mosoo.ai`) in config. The CLI
derives the console API (`/api`) and Public API (`/api/v1`) hosts internally.
`mosoo auth login` defaults to mosoo Cloud when no config exists, validates the
token against the console API, and stores the same credential for both derived
API hosts.

Self-hosted and local targets use explicit setup subcommands:

```sh
mosoo setup self-host --base-url https://mosoo.example.com
mosoo setup custom --api-url https://mosoo.example.com/api
mosoo setup local
```

Cloud and custom targets are also supported with explicit command flags:

```sh
mosoo doctor --json --target cloud
mosoo console user viewer --target custom --base-url https://example.com
```

Check the resolved target and readiness:

```sh
mosoo doctor --json
```

The JSON output is versioned with `schemaVersion` and groups machine-readable
readiness data under `target`, `auth`, `install`, `checks`, and `failures`.
Failure entries include stable `code` and `action` fields so automation can
branch without parsing human messages.

For local development targets, the installer can sign in through the local development
backdoor with an `@mosoo.ai` email, authorize a CLI device flow, and write the
account login credentials for both hostname bases. This only works against a loopback mosoo
API with the development backdoor enabled.

Run `mosoo auth login` for browser authorization. The resulting `mcli_` credential
grants account access, including operations across your Projects. `--hostname`
remains available for one-off host selection. Non-interactive installers can use
an existing account credential through `MOSOO_CLI_TOKEN`.

Application backends use Project API keys (`msp_`). Each key permits Agent
configuration, execution, and files within one Project. It cannot create or delete
Projects, manage API keys, or access another Project. After account login, create
or list keys with:

```sh
mosoo console-rest access create --set projectId=<project-id> --set label="backend" -o json
mosoo console-rest access list --project-id <project-id> -o json
```

Legacy `mst_` and `grt_pat_` credentials no longer work after the Project key
upgrade. Run `mosoo auth login` again for CLI access. For deployed integrations,
create a key under the matching Project and replace the old secret. Revoking a
Project key blocks later requests while existing tasks continue.

## Common commands

```sh
mosoo console user viewer
mosoo ls --project-limit 20 --agent-limit 20 --credential-limit 20 -o json
mosoo add-key --input-project-id <project-id> --input-vendor-id openai --input-name OpenAI --input-api-key-env OPENAI_API_KEY -o json
mosoo create-agent --file agent-create.json -o json
mosoo console agents publish --input-project-id <project-id> --input-agent-id <agent-id> -o json
mosoo run --input-project-id <project-id> --input-agent-id <agent-id> --input-prompt "Summarize this repository" -o json
mosoo console sessions events --project-id <project-id> --session-id <session-id> --limit 100 -o json
mosoo search "run agent" --json
mosoo commands show run --json
```

Use `commands show` before executing an unfamiliar generated command so flags,
body shape, auth, and output format are explicit.

### Public Thread file uploads

The Public API uses one multipart upload before a Thread references the file.
Upload a local file to the Agent endpoint and save `file.id` from the response:

```sh
mosoo public-thread-api files upload \
  --agent-id <agent-id> \
  --file ./brief.txt \
  -o json
```

Then put that ID in `resources[].file_id` when creating a Thread or sending a
follow-up `user_message` event:

```json
{
  "userId": "demo-user-001",
  "input": {
    "content": [{ "type": "text", "text": "Summarize the attachment." }],
    "type": "user.message"
  },
  "resources": [{ "type": "file", "file_id": "<file-id>" }]
}
```

There is no public create-upload, PUT, complete, or post-create attach step.
`GET /threads/{threadId}/files` lists claimed attachments and artifacts;
metadata, content download, and deletion are exposed under `/files/{fileId}`.
