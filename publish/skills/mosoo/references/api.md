# mosoo Public Thread API

Use this reference when application backend code creates a durable Project
Session, optionally using a saved private Agent preset. Existing v1 Agent
integrations remain supported. For creating, publishing, or changing mosoo resources, use the
generated CLI workflow in `references/cli.md` instead.

## Version selection

The examples below use direct Project Sessions. Select a deployment with
`GET /api/v2/openapi.json` and set
`MOSOO_API_BASE` to that service's `/api/v2` base. Confirm the target's available
features before changing a production integration.

v2 creates a Session at `POST /projects/{projectId}/threads`. The body requires
`configuration`: choose `type: "inline"` with harness, provider, model and
non-blank instructions, or `type: "agent"` with only `agent_id` for a saved
private preset. Mixed configuration is rejected. Inline execution creates no
Agent. Input, resources and `userId` are optional; omitted identity remains
`null`, while blank/null identity is invalid. No input creates an idle Session.

The returned `thread.id` is the durable handle for continuation, reads and files.
Its admitted configuration stays fixed; a new configuration requires a new
Session. A changed request under the same idempotency key returns 409.
`GET /threads/{threadId}/usage?limit=100&after=<cursor>` reads persisted usage.
Project provider credentials must be configured (BYOK). Platform model supply,
top-ups and commercial billing are tracked separately in
[#636](https://github.com/langgenius/mosoo/issues/636).

The v2 Agent compatibility route `POST /agents/{agentId}/threads` still accepts
an omitted body and freezes the latest saved private Agent configuration.
v1 keeps its existing admission/live selection and required non-blank userId
contract at `/api/v1/agents/{agentId}/threads`; this change does not publish an
Agent or change v1 selection.

The optional v2 budget extension is unreleased. Check the target's
`/api/v2/openapi.json` and deployment budget policy before sending top-level
`maxCostUsd` on create with `input`, or on send with a `user_message` event.
It is a positive USD number with at most six decimal places, bounded by the
deployment maximum, and applies only to that turn. Omission uses the configured
default if one exists; the client must not invent a default amount. An explicit
cap without a configured policy returns `409 readiness_blocked`.

Budgeted Runs expose `budget: { capUsd, estimatedCostUsd, state }`; state is
`available`, `settling`, `budget_exhausted`, or `budget_usage_unavailable`.
In-flight usage can exceed the estimate cap. The threshold blocks new model
requests, unknown usage fails closed, and native provider protocols remain
unchanged. With unavailable usage, the estimate covers only established usage.
Budget failures retain their failed outcome and available outputs, without
guaranteeing a successful checkpoint. Inspect files/events instead of treating
partial work as completed output. No funded inference is implied.

Usage returns `usage` and `nextCursor`. Null metrics are unknown, not zero.
`reportedCostUsd` is a runtime estimate; `usageContract` explains provider
cache/token conventions. Retrying a request keeps its idempotency key; a new
turn gets a new key. After an expired recovery window, new execution is
blocked with `readiness_blocked`; existing history and committed files remain
readable. Never replace a missing workspace with a fabricated continuation.

## Documentation sources

- Human documentation: `https://mosoo.ai/docs/`
- Machine-oriented integration guide: `https://mosoo.ai/docs/coding-agents/`
- Complete documentation index: `https://mosoo.ai/docs/llms.txt`
- Published OpenAPI document: `https://mosoo.ai/docs/openapi/mosoo-openapi.en.generated.json`

The target's OpenAPI document at `GET /api/v2/openapi.json` (or v1 equivalent) is the wire
contract. This guide explains the integration workflow and intentionally does
not duplicate every generated schema field.

## Boundary

Your application owns its UI, backend routes, user authentication, business
data, correlation IDs, API token storage, and persisted `thread.id` values. Its
trusted backend may supply an opaque `userId` when creating a v2 Session; v1
requires it. mosoo owns the Session runtime, provider and tool configuration, sandbox
execution, Thread lifecycle, and public events.

Do not expose `MOSOO_API_TOKEN` in frontend or browser code. Do not send model
provider credentials, channel, Skill or MCP configuration through the Public
Thread API. The inline configuration accepts only the documented harness,
provider, model and instructions fields.

## Configuration

Application backends need these values:

```sh
MOSOO_API_BASE=https://cloud.mosoo.ai/api/v2
MOSOO_PROJECT_ID=<project-id>
MOSOO_API_TOKEN=<project-api-key>
```

Authenticate every request with:

```http
Authorization: Bearer <MOSOO_API_TOKEN>
```

Create a Project API key (`msp_...`) under the selected Project. It can access
Session execution and files in that Project; it cannot manage
keys or access another Project. Keep account credentials from `mosoo auth
login` (`mcli_...`) in the CLI credential store and supply an explicit owned
Project when creating Sessions or uploading Project files. Legacy `mst_...` and
`grt_pat_...` tokens must be replaced after the Project key upgrade.

Use `Idempotency-Key` on Thread creation and event submission. Keep keys stable
for retries of the same method, route, and body; use a new key when the body
changes.

## Minimal Thread workflow

Create a Thread and queue its first Run:

```sh
curl -X POST "$MOSOO_API_BASE/projects/$MOSOO_PROJECT_ID/threads" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ticket-182-create" \
  -d '{
    "configuration": {
      "type": "inline",
      "harness": "openai-runtime",
      "provider": "openai",
      "model": "<model-id>",
      "instructions": "Analyze the supplied material and save the results."
    },
    "userId": "customer-123",
    "input": {
      "type": "user.message",
      "content": [
        { "type": "text", "text": "Triage this escalation." }
      ]
    }
  }'
```

Persist `response.thread.id` and use it for later Thread operations.

Submit an ordered event batch to a Thread:

```sh
curl -X POST "$MOSOO_API_BASE/threads/$MOSOO_THREAD_ID/events" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ticket-182-follow-up-1" \
  -d '{
    "events": [
      {
        "type": "user_message",
        "requestId": "ticket-182-message-1",
        "text": "List the three highest-priority actions."
      }
    ]
  }'
```

The `events` array must contain at least one event. Supported variants are
`user_message`, `permission_decision`, and `user_interrupt`; a batch may contain
more than one event.

Read or stream the public event projection:

```sh
curl "$MOSOO_API_BASE/threads/$MOSOO_THREAD_ID/events?limit=100" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN"

curl -N "$MOSOO_API_BASE/threads/$MOSOO_THREAD_ID/events/stream?limit=100" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN"
```

Treat event IDs as stable and reconcile after reconnecting through the snapshot
endpoint. Runtime transcripts, private diagnostics, and provider-native state
are not part of the public event contract. Group public events by `runId` when
reconstructing output, and after completion prefer `run.finalOutput.text` when
it is present.

## File workflow

The public write flow has one upload step. Upload exactly one multipart field
named `file` to the Project endpoint before creating or continuing a Thread:

```sh
curl -X POST "$MOSOO_API_BASE/projects/$MOSOO_PROJECT_ID/files" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN" \
  -F "file=@brief.txt"
```

The response contains a ready draft at `response.file`. Save
`response.file.id`, then reference it in `resources` when creating a Thread:

```json
{
  "configuration": {
    "type": "inline",
    "harness": "openai-runtime",
    "provider": "openai",
    "model": "<model-id>",
    "instructions": "Summarize the supplied attachment."
  },
  "userId": "customer-123",
  "input": {
    "type": "user.message",
    "content": [
      { "type": "text", "text": "Summarize the attachment." }
    ]
  },
  "resources": [
    { "type": "file", "file_id": "<file-id>" }
  ]
}
```

For a follow-up, put the same resource shape on the `user_message` event:

```json
{
  "events": [
    {
      "type": "user_message",
      "text": "Compare this attachment with the earlier result.",
      "resources": [
        { "type": "file", "file_id": "<file-id>" }
      ]
    }
  ]
}
```

mosoo validates and claims referenced drafts into the Thread before queueing the
Run. Drafts cannot be claimed across Project or caller boundaries. The public
multipart endpoint accepts one file up to 67108864 bytes and returns a ready
draft.

There is no public create-upload, PUT-content, complete-upload, or
`POST /threads/{threadId}/files` step. Do not use Console REST upload-session
commands as the Public Thread upload workflow.

List the files claimed into a Thread:

```sh
curl "$MOSOO_API_BASE/threads/$MOSOO_THREAD_ID/files" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN"
```

Retrieve metadata or download bytes:

```sh
curl "$MOSOO_API_BASE/files/$MOSOO_FILE_ID" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN"

curl "$MOSOO_API_BASE/files/$MOSOO_FILE_ID/content" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN" \
  --output brief.txt
```

`DELETE /files/{fileId}` deletes a visible pre-Thread draft or Thread file.
`DELETE /threads/{threadId}/files/{fileId}` removes a file through its Thread.
Archived, rescheduling, and terminal Threads remain readable but reject file
mutation.

## Routes

All paths are relative to `MOSOO_API_BASE` (`/api/v2`). Project routes and usage
are v2-only; Agent routes remain available for compatibility in both versions.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/projects/{projectId}/threads` | Create a Session from inline configuration or an optional Agent preset |
| `POST` | `/projects/{projectId}/files` | Upload one ready Project draft file |
| `GET` | `/threads/{threadId}/usage` | Read paginated persisted usage |
| `POST` | `/agents/{agentId}/files` | Upload one ready draft file as multipart field `file` |
| `GET` | `/files/{fileId}` | Retrieve public file metadata |
| `DELETE` | `/files/{fileId}` | Delete a draft or Thread file |
| `GET` | `/files/{fileId}/content` | Download ready file bytes |
| `GET` | `/agents/{agentId}/threads` | List Threads for an Agent endpoint and caller |
| `POST` | `/agents/{agentId}/threads` | Create a Thread, optionally queueing its first Run |
| `GET` | `/threads/{threadId}` | Retrieve the current Thread summary |
| `DELETE` | `/threads/{threadId}` | Permanently delete a Thread |
| `POST` | `/threads/{threadId}/archive` | Archive a Thread |
| `POST` | `/threads/{threadId}/unarchive` | Unarchive a Thread |
| `GET` | `/threads/{threadId}/events` | Read the stable public event snapshot |
| `POST` | `/threads/{threadId}/events` | Submit an ordered batch of user messages, permission decisions, or interrupts |
| `GET` | `/threads/{threadId}/events/stream` | Stream public events with SSE |
| `GET` | `/threads/{threadId}/files` | List claimed attachments and Agent artifacts |
| `DELETE` | `/threads/{threadId}/files/{fileId}` | Remove a file from a writable Thread |
| `GET` | `/openapi.json` | Retrieve the machine-readable contract |

## Lifecycle and limits

- A Project create body requires `configuration`. Omit `input` to create an
  idle Session without a Run. Its Agent provenance may be null.
- A submitted event batch contains at least one event.
- Create-Thread input text is limited to 32000 characters.
- `userId` is optional in v2, required in v1, immutable for the Thread, and limited to 255 characters.
- Public file uploads are limited to 67108864 bytes.
- Event lists default to 100 entries and accept at most 1000.
- Thread lists return at most 100 Threads.

## Errors and retries

Errors use a JSON envelope with a stable `error.code`. Branch on the code rather
than matching the human message.

| Status | Typical meaning | Action |
| --- | --- | --- |
| `400` | Invalid request shape, value, JSON, multipart envelope, or file resource | Correct the request; do not retry unchanged |
| `401` | Missing, invalid, or revoked token | Re-authenticate or rotate the token |
| `403` | The caller cannot use the Agent, Thread, or file | Check token and resource ownership |
| `404` | The resource is absent or not visible | Verify IDs returned by this API |
| `409` | Unpublished or inactive Agent, readiness block, or idempotency conflict | Inspect state; retry only when the conflict is transient |
| `429` | Token request budget exceeded | Back off using `Retry-After` |
| `500` | Unexpected mosoo failure | Retry briefly with backoff and preserve diagnostics |

Retry create and event requests with the same `Idempotency-Key` only when the
method, route, and body are unchanged. Use a new key for a reconciled request.
