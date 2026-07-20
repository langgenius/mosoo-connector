# Mosoo Public Thread API

Use this reference when application backend code calls an already published
Mosoo Agent. For creating, publishing, or changing Mosoo resources, use the
generated CLI workflow in `references/cli.md` instead.

## Documentation sources

- Human documentation: `https://mosoo.ai/docs/`
- Machine-oriented integration guide: `https://mosoo.ai/docs/coding-agents/`
- Complete documentation index: `https://mosoo.ai/docs/llms.txt`
- Published OpenAPI document: `https://mosoo.ai/docs/openapi/mosoo-openapi.en.generated.json`

The checked-in Mosoo OpenAPI document at `GET /api/v1/openapi.json` is the wire
contract. This guide explains the integration workflow and intentionally does
not duplicate every generated schema field.

## Boundary

Your application owns its UI, backend routes, users, business data, correlation
IDs, API token storage, and persisted `thread.id` values. Mosoo owns the
published Agent runtime, provider and tool configuration, sandbox execution,
Thread lifecycle, and public events.

Do not expose `MOSOO_API_TOKEN` in frontend or browser code. Do not send model,
provider, channel, Skill, MCP, or runtime configuration through the Public
Thread API.

## Configuration

Application backends need these values:

```sh
MOSOO_API_BASE=https://try.mosoo.ai/api/v1
MOSOO_AGENT_ID=<published-agent-id>
MOSOO_API_TOKEN=<access-token>
```

Authenticate every request with:

```http
Authorization: Bearer <MOSOO_API_TOKEN>
```

Use `Idempotency-Key` on Thread creation and event submission. Keep keys stable
for retries of the same method, route, and body; use a new key when the body
changes.

## Minimal Thread workflow

Create a Thread and queue its first Run:

```sh
curl -X POST "$MOSOO_API_BASE/agents/$MOSOO_AGENT_ID/threads" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ticket-182-create" \
  -d '{
    "client_external_ref": "ticket-182",
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
        "clientRequestId": "ticket-182-message-1",
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
named `file` to the Agent endpoint before creating or continuing a Thread:

```sh
curl -X POST "$MOSOO_API_BASE/agents/$MOSOO_AGENT_ID/files" \
  -H "Authorization: Bearer $MOSOO_API_TOKEN" \
  -F "file=@brief.txt"
```

The response contains a ready draft at `response.file`. Save
`response.file.id`, then reference it in `resources` when creating a Thread:

```json
{
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

Mosoo validates and claims referenced drafts into the Thread before queueing the
Run. Drafts cannot be claimed across App or caller boundaries. The public
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

All paths are relative to `MOSOO_API_BASE` (`/api/v1`).

| Method | Path | Purpose |
| --- | --- | --- |
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

- Creating a Thread may omit `input`; that creates an idle Thread without a Run.
- A submitted event batch contains at least one event.
- Create-Thread input text is limited to 32000 characters.
- `client_external_ref` is limited to 255 characters.
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
| `500` | Unexpected Mosoo failure | Retry briefly with backoff and preserve diagnostics |

Retry create and event requests with the same `Idempotency-Key` only when the
method, route, and body are unchanged. Use a new key for a reconciled request.
