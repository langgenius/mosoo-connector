# CLI Reference

Generated from Lathe's mosoo CLI Skill output during `make build`.

## Runtime State

Run:

```sh
mosoo doctor --json
```

Use the result to decide whether the current task targets local mosoo runtime or
mosoo cloud runtime before running API commands.

## Command Selection

Use generated CLI commands for mosoo resource operations, and use
`references/api.md` for application code that creates or continues a Session.
Do not invent a wrapper command when the generated catalog already exposes the
operation.

High-frequency root commands such as `mosoo ls`, `mosoo run`, `mosoo add-key`,
and `mosoo create-agent` are Lathe-generated `shortcuts` for canonical generated
operations. Treat them as generated command entries, and confirm their exact
flags and body shape with `mosoo commands show <shortcut> --json`.

For a new Project, Agent creation, publishing, credential setup, or Console/API
inspection, search the generated catalog first. For app environment files only,
derive `MOSOO_API_BASE`, `MOSOO_PROJECT_ID`, and `MOSOO_API_TOKEN` from the
Project Session contract. `MOSOO_AGENT_ID` is needed only for an Agent integration.

Use this reference when a user asks you to operate `mosoo`, inspect its API commands, or find the right generated command for an API task.

## Choose the Public API version

`public-thread-api` keeps the published-Agent v1 contract and required `userId`.
`public-thread-api-v2` creates a durable Session directly in a Project; no Agent
is required. Choose an explicit inline configuration or an optional saved
private Agent preset. `userId` is optional. Verify that the selected deployment advertises
`/api/v2/openapi.json` and confirm the features available on that target before
using v2. Do not publish solely to invoke a saved Agent.

```sh
mosoo commands show public-thread-api-v2 threads create --json
mosoo public-thread-api-v2 files upload --project-id <project-id> --file ./brief.txt -o json
mosoo public-thread-api-v2 threads create --project-id <project-id> --file session.json --idempotency-key <stable-create-key> -o json
mosoo public-thread-api-v2 events send --thread-id <thread-id> --file events.json --idempotency-key <stable-turn-key> -o json
mosoo public-thread-api-v2 events wait --thread-id <thread-id> --final-output
mosoo public-thread-api-v2 threads usage --thread-id <thread-id> --limit 100 -o json
```

Use the same target on every command. `session.json` must contain an explicit
configuration; use the uploaded `file.id` in resources only when a file is needed:

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
  "resources": [{ "type": "file", "file_id": "<file-id>" }]
}
```

The `inline` mode requires non-blank harness, provider, model and instructions.
For a saved private preset use only
`"configuration": { "type": "agent", "agent_id": "<agent-id>" }`.
Never add inline overrides to a preset. Omit input for an idle Session; omit
userId to keep it null. The Session freezes its admitted configuration, and
follow-ups use the returned `thread.id`. Inline execution creates no hidden Agent.
Reusing a create idempotency key with changed configuration returns 409.

A Project key can only use its own Project. CLI account login also requires an
explicit owned `--project-id` for Session creation and Project uploads. The
compatibility `threads create --agent-id` and `files upload --agent-id` forms
remain available; creation on that Agent route still permits an omitted body.
Do not combine `--project-id` and `--agent-id`; Project presets belong in the body.

The initial v2 release ([#582](https://github.com/langgenius/mosoo/issues/582))
uses your own model provider account (BYOK): configure its credentials in the
Project, then call the Session API. Saving an Agent is optional. Platform model supply,
top-ups, and commercial usage billing are separate work in
[#636](https://github.com/langgenius/mosoo/issues/636) and do not block #582.
Usage records and provider cost estimates remain in scope. The existing
file, wait and transcript recipes below also work with the v2 module prefix.

Usage is paginated with `--after <nextCursor>`. `null` means unreported, and
`reportedCostUsd` is an estimate, not settled billing. Do not combine provider
cache buckets without inspecting `usageContract`. Run `mosoo auth login` for the target after upgrading to authorize v2, or
configure the Project key for that explicit v2 hostname. An API invocation
never restores a credential removed by logout.
`doctor --json` reports both versioned OpenAPI hashes in `contract`.

## Common Workflow Recipes

Use this section as the entry point for end-to-end mosoo CLI tasks. It defines
workflow order and handoff values only; keep detailed command flags and request
shapes in the owning workflow sections below.

For a direct Session integration, select the Project, configure BYOK provider
credentials, then follow `Choose the Public API version` above. Pass inline
configuration, optionally upload files, and persist the returned Thread ID for
continuation. Saving or publishing an Agent is not a prerequisite.

For an existing backend or Worker integration with a published Agent:

1. Resolve runtime and hosts with `Runtime State` and `Host Context`.
2. Provision or select the Project and Agent with `Agent Project Provisioning Workflow`.
3. If a bound Skill has runtime dependencies, prepare and bind an Project-local
   Environment with `Skill Runtime Environment Workflow` before publishing or
   starting a new Session.
4. Prepare backend environment values with `Public API Tokens`.
5. Upload files only when the thread needs attachments; use `Public Thread File Upload Workflow`, then reference the returned `fileId` when creating or continuing the thread.
6. Create or continue the thread, wait for completion, and inspect output with `Public Thread Wait, Final Output, And Transcript Workflow`.
7. Edit Agent configuration only through `Agent Manifest Workflow`.

Carry these handoff values between workflow sections: `projectId`, `agentId`,
`environmentId`, `threadId`, `fileId`, env file path, and manifest file path. If
any value is missing, return to the section that produces it instead of
guessing.

## Public API Tokens

`MOSOO_API_TOKEN` is a server-side credential for application backends or
Workers that create or continue Sessions through the Public API. Do not expose it in
browser or frontend code.

Create Project API keys (`msp_...`) under the selected Project. One account can own
multiple Projects and each Project can have multiple keys. Mosoo enforces the
Project boundary for Session execution, Agent configuration, and files. A Project key
cannot manage API keys, account settings, or other Projects. Your backend still
owns application-level tenant and user mapping.

Use `mosoo auth login` for account access through browser authorization. The
resulting `mcli_...` credential can manage your Projects and their keys. It is a
CLI login credential and must not be exported into application environments.

```sh
mosoo auth login
mosoo console-rest access create --set projectId=<project-id> --set label="backend" -o json
mosoo console-rest access list --project-id <project-id> -o json
```

Legacy `mst_...` and `grt_pat_...` credentials stop working at the Project key
upgrade. Sign in again for CLI access; create replacement Project keys for
application backends. Revocation blocks subsequent requests without cancelling
existing tasks.

Use `mosoo agent env export` or `mosoo agent env write --file <path>` with
`--api-token` or `MOSOO_API_TOKEN` set to a Project API key. Keep raw token values
in backend secret storage or environment files; terminal output is redacted.

## Skill Runtime Environment Workflow

Use this workflow when a Skill declares Python, Node.js, system, or other
runtime dependencies; includes setup instructions; requires runtime environment
variables; or fails because a module, package, command, or env var is missing.
Do not port the Skill to another language or remove the dependency merely to fit
the current sandbox.

A mosoo Environment is an Project-local runtime template. Runtime installs its
declared packages, runs its setup script before the Agent process starts, and
injects its env vars. An Agent selects the Environment by `environmentId`, and
each new Session freezes the selected Environment revision. Environment does
not contain Skills, Files, or MCP servers. The current mosoo product stores
network-policy intent, but Runtime does not enforce that policy yet; do not
present it as a security boundary.

Follow these steps:

1. Inspect the Skill package and its dependency manifests, imports, setup
   instructions, and documented environment-variable names. Treat the Skill's
   declared implementation and dependencies as requirements unless the user
   explicitly asks for a port or dependency removal.
2. Resolve the Project and inspect its available runtime templates:

```sh
mosoo console environments project-environment-list --project-id <project-id> -o json
mosoo console environments environment --project-id <project-id> --environment-id <environment-id> -o json
```

3. Reuse a suitable Project-local Environment, or create/copy one when an
   independent template is needed. To update an Environment, fetch it first and
   preserve every unchanged package, env-var name, setup, and policy field; the
   update is a complete configuration update.

```sh
mosoo commands show console environments create-environment --json
mosoo console environments create-environment --file environment-create.json -o json
mosoo commands show console environments update-environment --json
mosoo console environments update-environment --file environment-update.json -o json
```

Declare repeatable packages in `packages` and use `setupScript` only for setup
that cannot be expressed as package declarations. Pin versions when the Skill
requires them. A failing setup script prevents the Session from starting.

4. Keep secret values out of committed JSON. Create the Environment with
   `envVars: []`, then write each Skill runtime secret through the dedicated
   mutation:

```sh
mosoo console environments set-environment-variable-value \
  --input-project-id <project-id> \
  --input-environment-id <environment-id> \
  --input-key <secret-name> \
  --input-value "$SECRET_VALUE" \
  -o json
```

Environment env vars are for values the Skill process must read inside the
sandbox. Prefer mosoo's dedicated Vendor Credential or MCP Credential resource
when one exists. Keep `MOSOO_API_TOKEN` in the calling backend or Worker; do not
inject it into the Agent Environment.

5. Set the Environment as the Project default when it should be preselected for new
   Agents, or bind it explicitly to the target Agent through the manifest
   round-trip workflow:

```sh
mosoo console environments set-project-default-environment --input-project-id <project-id> --input-environment-id <environment-id> -o json
mosoo agent manifest probe --project-id <project-id> --agent-id <agent-id> --out agent.yaml
# Set environment.environmentId in agent.yaml and preserve all other fields.
mosoo agent manifest apply --project-id <project-id> --agent-id <agent-id> --file agent.yaml --dry-run
mosoo agent manifest apply --project-id <project-id> --agent-id <agent-id> --file agent.yaml
```

6. Publish if needed, start a new Session, and run the Skill's smallest useful
   smoke test. Environment edits affect future Sessions only; do not use an
   already-started Session to verify a new revision.

Carry these handoff values: `projectId`, `agentId`, `environmentId`, Environment
JSON path, Agent manifest path, required package list, setup requirements, and
required env-var names. Never record secret values in these handoff artifacts.

## Agent Project Provisioning Workflow

Use this workflow when a task starts from Project and Agent setup instead of an
already published Agent. It is a product workflow assembled from generated
commands plus the env and Public Thread workflows below.

First create or reuse the Project, create the Agent, and publish it. Run the
generated commands in order and save each returned `projectId` and `agentId` before
moving to the next step:

```sh
mosoo console projects project-list --organization-id <organization-id> -o json
mosoo console projects create-project --input-organization-id <organization-id> --input-name <project-name> -o json
mosoo add-key --input-project-id <project-id> --input-vendor-id openai --input-name OpenAI --input-api-key-env OPENAI_API_KEY -o json
mosoo create-agent --file create-agent.json -o json
mosoo console agents publish-agent --input-project-id <project-id> --input-agent-id <agent-id> -o json
```

After publish, continue through the related workflow sections instead of
duplicating their commands here:

1. Write backend or Worker env values with `Public API Tokens`.
2. Upload attachments only when the first thread requires files; use `Public Thread File Upload Workflow`.
3. Run a smoke test by creating a Public Thread and waiting for final output with `Public Thread Wait, Final Output, And Transcript Workflow`.
4. If Agent configuration needs a follow-up change, round-trip it through `Agent Manifest Workflow`.

Use `mosoo commands show <path...> --json` before each generated command to
confirm body shape and required flags. Prefer `--file` for large Agent create
bodies. If a step fails or times out, inspect state with `console projects project-list`,
`console agents accessible-agent-list`, or `console agents agent` before
retrying; do not recreate resources until the current remote state is known.

## Public Thread File Upload Workflow

For v2 Public Thread file uploads, upload each file to the Project endpoint before
creating or continuing a Thread. Save `response.file.id`, then reference it as
`resources[].file_id` in the Thread create body or a `user_message` event body:

```sh
mosoo public-thread-api-v2 files upload --project-id <project-id> --file <path> -o json
mosoo public-thread-api-v2 threads create --project-id <project-id> --file session.json -o json
```

The upload uses `multipart/form-data` with exactly one `file` field and returns
a ready draft file. A create body references it like this:

```json
{
  "configuration": {
    "type": "inline",
    "harness": "openai-runtime",
    "provider": "openai",
    "model": "<model-id>",
    "instructions": "Summarize the supplied attachment."
  },
  "input": {
    "content": [{ "type": "text", "text": "Summarize the attachment." }],
    "type": "user.message"
  },
  "resources": [{ "type": "file", "file_id": "<file-id>" }]
}
```

The same `resources` shape is available on a follow-up `user_message` event.
mosoo claims the draft file into that Thread before queueing the Run. There is
no public create-upload, PUT, complete, or post-create attach command. Use
`mosoo commands show public-thread-api-v2 files upload --json` before uploading to
confirm the generated flags and host selection.
The v1 and v2 compatibility uploads use `--agent-id` with their Agent route.

## Public Thread Wait, Final Output, And Transcript Workflow

Project create requires an explicit configuration; userId is optional. Omitting
input creates an idle Session with no Run to wait for. The compatibility v1
Agent route still requires a body with a non-blank userId.

```sh
mosoo public-thread-api-v2 threads create --project-id <project-id> --file session.json --wait -o json
mosoo public-thread-api-v2 threads create --project-id <project-id> --file session.json --final-output
mosoo public-thread-api-v2 events wait --thread-id <thread-id> --final-output
mosoo public-thread-api-v2 threads transcript --thread-id <thread-id>
```

## Workflow

1. Search for candidates with `mosoo search "<intent>" --json`; use `--limit` when needed. Search is only candidate discovery.
2. Inspect the exact command with `mosoo commands show <path...> --json` before executing an unfamiliar command.
3. If the command detail has `auth.required=true`, run `mosoo doctor --json` and check the resolved target/auth state before execution. If credentials are missing, run `mosoo auth login` for the resolved target.
4. Execute only after flags, body, auth, HTTP path, and output hints are clear from `commands show`.

## Contract Provenance

`mosoo doctor --json` reports the pinned Mosoo `upstreamCommit` and normalized
Public Thread OpenAPI `sha256` under `contract`. The bundled Skill carries the
same record in `references/provenance.json`. If those values differ, update the
older CLI or Skill before copying a Public Thread request shape from it.

## Setup And Login

For first-time mosoo Cloud usage, do not ask the user for a hostname:

```sh
mosoo setup
mosoo auth login
```

`mosoo setup` stores the cloud service root. `mosoo auth login` defaults to
mosoo Cloud when no config exists and stores one credential for both the console
API (`/api`) and Public Thread API (`/api/v1`) hosts derived from the root.

For self-hosted or local runtimes, configure the root target first:

```sh
mosoo setup self-host --base-url https://mosoo.example.com
mosoo setup local
```

Use `--hostname <surface-host>` only as an advanced one-off override for an
exact API surface.

## Host Context

Use `mosoo doctor --json` first when the target is not explicit. It reports the
resolved target, base URL, and per-surface hosts. Console GraphQL and console
REST commands use the `/api` surface. Public Thread API commands use the
`/api/v1` surface.

Use `mosoo setup`, `mosoo setup local`, or `mosoo setup self-host` to persist a
target. Use `--target local`, `--target cloud`, or `--target custom --base-url
<service-root>` for per-command target selection. Use `--hostname
<surface-host>` or `MOSOO_HOST` only when overriding one exact surface host.

For runnable examples covering `--target`, `--base-url`, `--hostname`, and
`MOSOO_HOST`, read `references/cli/host-context.md`.

## General Commands

- `mosoo commands --json`: full generated command catalog.
- `mosoo commands --include-hidden --json`: include hidden generated commands.
- `mosoo commands show <path...> --json`: source of truth for one command.
- `mosoo commands schema --json`: catalog schema version for parser compatibility.
- `mosoo search "<intent>" --json`: ranked candidate commands.

## Agent Manifest Workflow

Prefer the product workflow commands for editable Agent manifest YAML:

```sh
mosoo agent manifest probe --project-id <project-id> --agent-id <agent-id> --out agent.yaml
mosoo agent manifest diff --project-id <project-id> --agent-id <agent-id> --file agent.yaml
mosoo agent manifest apply --project-id <project-id> --agent-id <agent-id> --file agent.yaml --dry-run
mosoo agent manifest apply --project-id <project-id> --agent-id <agent-id> --file agent.yaml
```

`probe` reads the current remote manifest and writes YAML for editing or version
control. It also has a `pull` alias. `diff` performs a local field-level diff
between the local YAML target state and the current remote state.

`apply` always fetches the current remote manifest before writing, treats the
local YAML as the intended patch, preserves remote fields omitted from the YAML,
and then calls the raw `updateAgentConfig` operation. Use `--dry-run` first to
show the field-level changes without writing.

When changing prompts, models, providers, tools, runtime, or environment
settings, do not reconstruct an update payload from memory or guessed defaults.
Round-trip the current manifest, edit only the requested fields, and preserve
unchanged values such as `environmentId`, runtime, provider, model, skill IDs,
MCP server IDs, and `providerOptions`.

The raw generated `console agents manifest` and `console agents update-config`
commands are hidden from normal discovery. Use them only for low-level API
inspection with `mosoo commands --include-hidden --json`.

## References

- Read `references/cli/catalog.md` for the command discovery protocol and catalog field meanings.
- Read `references/cli/modules/console.md` for the `console` module command index.
- Read `references/cli/modules/console-rest.md` for the `console-rest` module command index.
- Read `references/cli/modules/public-thread-api.md` for the `public-thread-api` module command index.

## Rules

- Do not guess flags or request body shape from command names.
- Do not execute directly from search results; confirm with `commands show` first.
- Prefer `-o json` for machine-readable command output unless the user asks for human-readable output.
- Use `--file`, `--set`, or `--set-str` for JSON request bodies according to `commands show` body requirements.
