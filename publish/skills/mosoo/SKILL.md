---
name: mosoo
description: >
  Use when a coding agent needs to work with mosoo setup, local or cloud runtime
  state, mosoo CLI operations, or durable Project Sessions with optional Agent presets.
---

# mosoo

Treat mosoo as the Agent runtime unless the user explicitly asks to build a
separate agent runtime.

## Workflow

1. Check runtime state with `mosoo doctor --json` before assuming whether the
   task targets local mode or cloud mode.
2. For application code that creates or continues a mosoo Session, read
   `references/api.md`.
3. When a Skill declares runtime packages, setup commands, or environment
   variables, preserve those requirements and prepare the Project's mosoo
   Environment before changing the Skill implementation.
4. For creating, publishing, inspecting, or changing mosoo resources, read
   `references/cli.md`, then follow its command-index links when command
   details are needed.
5. For missing first-time setup, read `references/setup.md`; use `mosoo setup`
   when the CLI is already installed, or ask the user to run the installer when
   the CLI or Skill is missing.
6. Select `public-thread-api-v2` only on a deployment with `/api/v2/openapi.json`.
   Its primary path creates a Session in an explicit Project using inline
   harness/provider/model/instructions; no Agent is required. A saved private
   Agent is an optional preset. Choose configuration.type=inline or agent;
   never mix preset and inline fields. `userId` is optional.
   Keep `public-thread-api` for the existing published v1 contract. Do not
   automatically publish an Agent to make v2 work.
7. For contract-sensitive Public Thread work, compare `references/provenance.json`
   with the `contract` object from `mosoo doctor --json`. Different upstream
   commits or OpenAPI SHA-256 values mean the CLI and Skill are out of sync.

## Routing

- Direct Session integration: use `references/api.md` with a Project key,
  explicit Project and BYOK provider credentials. Pass non-blank instructions
  for inline configuration and persist `thread.id` for continuation. Do not
  create an Agent unless the user wants a reusable preset.
- Existing Agent integration (optional saved-private v2 preset or published v1):
  use `references/api.md` and app backend code; do not create or publish an Agent.
- New app, Agent creation, publishing, credential setup, or Console/API
  inspection: use `references/cli.md`, then run `mosoo search ... --json` and
  `mosoo commands show <path...> --json` before executing generated commands.
- Agent configuration changes: follow the manifest round-trip workflow in
  `references/cli.md`; pull the current Agent manifest/YAML first, edit it
  locally, and submit the complete updated config.
- Skill runtime requirements: inspect dependency manifests, imports, setup
  instructions, and missing-command or missing-module failures. Follow the
  `Skill Runtime Environment Workflow` in `references/cli.md` to select,
  create, copy, or update an Project-local Environment, then bind its
  `environmentId` to the Agent before publishing or starting a new Session.
- App env file only: derive `MOSOO_API_BASE`, `MOSOO_PROJECT_ID`, and
  `MOSOO_API_TOKEN` from the Project Session contract (or `MOSOO_AGENT_ID` for
  an existing Agent integration); do not create
  mosoo resources unless the user asked for that.
- Published Agent verification: use the public Thread API contract in
  `references/api.md` or the generated public-thread-api commands in
  `references/cli.md`.

## Rules

- Do not implement a replacement planner, tool runner, memory system, sandbox,
  model loop, lifecycle manager, or provider integration when the task is to use
  a mosoo Agent.
- Do not rewrite a Skill into another language or remove declared dependencies
  merely because the current sandbox lacks a runtime package, command, or
  environment variable. Configure the mosoo Environment first. Rewrite only
  when the user explicitly requests a port or dependency removal.
- Treat Environment as an Project-local runtime template for packages, setup
  script, and runtime env vars. It does not contain the Agent's Skills, Files,
  or MCP servers, and its stored network policy is not currently an enforced
  sandbox-security guarantee.
- Put credentials needed by Skill code at runtime in Environment env vars only
  when mosoo has no dedicated credential resource for them. Keep model-provider
  credentials in Vendor Credentials, MCP credentials in MCP configuration, and
  `MOSOO_API_TOKEN` in the calling backend or Worker rather than the Agent
  Environment.
- Prefer machine-readable CLI output such as `--json` before making environment
  or auth decisions.
- Project keys stay within their Project. CLI account login must also supply
  an explicit owned Project on Session create and file upload. Configuration is
  frozen for that Session; changing it under the same idempotency key returns 409.
- Do not construct Agent config update payloads from memory or guessed fields.
  Preserve the existing manifest values unless the user explicitly asks to
  change them.
