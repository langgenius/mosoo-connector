# Catalog Protocol

Use the runtime catalog as the source of truth. Generated references are a fast index; command execution details come from the CLI itself.

## Search

Run `mosoo search "<intent>" --json` to find candidate commands. Use `--limit` to control result count. Treat search output as candidates only.

## Full Catalog

Run `mosoo commands --json` to inspect the generated command catalog. Use `--include-hidden` only when hidden commands are relevant.

Key fields:

- `path`: command path to pass to `commands show` or execute after the CLI name.
- `shortcuts`: root-level commands that execute the same operation with preset flag values.
- `http`: HTTP method and path template.
- `http.default_hostname`: optional source-level host selected after explicit `--hostname` and `$MOSOO_HOST`; when present it is used before the single-host fallback from `hosts.yml`.
- `flags`: CLI flags, parameter location, type, required state, defaults, enum values, format, input modes, and help.
- `body`: request body requirement, media type, and optional `runtime_schema` preflight source, including its active-context prerequisites.
- `auth`: whether auth is required and which scopes are declared.
- `examples`: runnable examples with optional body shape, output hints, and follow-up commands.
- `output`: list path, default columns, response media type, pagination, and streaming hints; a streaming policy describes collection, terminal outcomes, and optional live projection.
- `flags[].context`: an optional account-scoped default with explicit flag, declared environment, then stored-value precedence.
- `sets_context`: a successful operation persists its declared parameter as the selected host's active context.
- `notes`, `prerequisites`, and `known_errors`: overlay-provided operation context that is not inferred from the API spec.

## Command Detail

Run `mosoo commands show <path...> --json` before executing an unfamiliar command. This is the source of truth for flags, body, auth, HTTP path, and output hints.

## Schema

Run `mosoo commands schema --json` to read the catalog schema version before parsing catalog JSON with durable tooling.

## Sensitive Flags

When a flag entry has `input_modes`, prefer safe modes over putting secrets directly in shell arguments.

- `flag`: pass the direct `--<flag>` value; keep this for compatibility or non-secret values.
- `env`: pass `--<flag>-env NAME` to read the value from an environment variable.
- `file`: pass `--<flag>-file path` to read the value from a file.
- `stdin`: pass `--<flag>-stdin` to read the value from stdin.
- Use only one input mode for the same flag.

## Request Bodies

- `--file path`: read a JSON body from a file.
- `--file -`: read a JSON body from stdin.
- `--set key.path=value`: build JSON with type inference for booleans, null, integers, and floats.
- `--set-str key.path=value`: build JSON while forcing the value to remain a string.

If `body.runtime_schema` is present, normal execution fetches that JSON Schema and validates the body before the target request. `--dry-run` does not fetch it.

## Output

Use `-o json` for machine-readable command output. Other supported formats are `table`, `yaml`, and `raw`. For collected streams, choose one mode: JSON or YAML for one document, `--stream` in the default output mode when `output.streaming.policy.live` is present, or raw for wire events.

On a non-zero exit with JSON or YAML output, read `error.code`, `error.message`, and `error.hint`; optional `error.http` contains only `status`. A configured pause exits zero and is represented by the field mapping in its stream collection policy.

## Auth

If command detail returns `auth.required=true`, run `mosoo auth status --hostname <host>` before execution. Use `http.default_hostname` when present unless the user provides `--hostname` or `$MOSOO_HOST`; if no matching host is logged in, stop and ask the user to authenticate.
For browser-based OAuth login, run `mosoo auth login --device-auth --hostname <host> --provider <provider>`. The browser opens by default in an interactive terminal; use `--no-browser` for manual login. `auth_type: bearer` in `hosts.yml` is expected after login because API requests use the issued bearer token.
