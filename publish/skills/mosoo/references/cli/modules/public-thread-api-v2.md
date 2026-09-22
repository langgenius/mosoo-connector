# Module `public-thread-api-v2`

## Source

- Backend: `openapi3`
- Default hostname: `http://127.0.0.1:8787/api/v2`
- Repository: https://github.com/langgenius/mosoo.git
- Pinned tag: `be71268498d41f949c4d16d37c51587b0e572f7a`
- Files: `docs/openapi/public-thread-api.v2.openapi.json`
- Resolved SHA: `be71268498d41f949c4d16d37c51587b0e572f7a`

## Events

### `mosoo public-thread-api-v2 events list-events`

- Summary: List thread events
- HTTP: `GET /threads/{threadId}/events`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
  - `--limit` (query, default `100`): Maximum number of latest Thread events to return.
- Output: list path `events`; columns `type`, `id`, `content`, `durationMs`, `occurredAt`, `runId`; response media `application/json`; pagination `cursor`
- Known errors:
  - HTTP 401: Invalid or revoked credential. Rotate the Project API key or run mosoo auth login again.
  - HTTP 404: Thread not found for this caller.
- Example: `mosoo public-thread-api-v2 events list-events --thread-id <thread-id>`

### `mosoo public-thread-api-v2 events send`

- Summary: Send events to a thread
- HTTP: `POST /threads/{threadId}/events`
- Auth: required
- Body: required; media type `application/json`
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
  - `--idempotency-key` (header): Optional key for retry-safe create-thread and send-events calls. Reusing the same key with the same request returns the original response. Reusing the key while the original request is still processing returns 409.
- Output: list path `events`; columns `type`, `requestId`, `run`; response media `application/json`
- Known errors:
  - HTTP 401: Invalid or revoked credential. Rotate the Project API key or run mosoo auth login again.
  - HTTP 409: Idempotency key reused while the original request is still processing.
- Examples:
  - Send a user message event to an existing thread.
    Command: `mosoo public-thread-api-v2 events send --thread-id <thread-id> --file events.json -o json`
    Body shape: `{"events":[{"requestId":"cli-send-001","text":"Continue the task with this follow-up.","type":"user_message"}]}`
    Output list path: `events`
    Follow-up commands:
      - `mosoo public-thread-api-v2 events list-events --thread-id <thread-id> -o json`

### `mosoo public-thread-api-v2 events stream`

- Summary: Stream thread events (SSE)
- HTTP: `GET /threads/{threadId}/events/stream`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
  - `--limit` (query, default `100`): Maximum number of latest Thread events to return.
- Output: response media `text/event-stream`; streaming `sse`
- Example: `mosoo public-thread-api-v2 events stream --thread-id <thread-id> -o raw`

## Files

### `mosoo public-thread-api-v2 files delete-file`

- Summary: Delete a file
- HTTP: `DELETE /files/{fileId}`
- Auth: required
- Body: none
- Flags:
  - `--file-id` (path, required, ulid): File ID returned by add or list Thread files. v1 IDs are bare ULIDs.
- Output: response media `application/json`
- Example: `mosoo public-thread-api-v2 files delete-file --file-id <file-id> -o json`

### `mosoo public-thread-api-v2 files download`

- Summary: Download file content
- HTTP: `GET /files/{fileId}/content`
- Auth: required
- Body: none
- Flags:
  - `--file-id` (path, required, ulid): File ID returned by add or list Thread files. v1 IDs are bare ULIDs.
  - `--disposition` (query, default `attachment`, one of: attachment|inline): Controls the Content-Disposition response header. Use attachment for downloads or inline for previewable content.
- Output: response media `application/octet-stream`
- Example: `mosoo public-thread-api-v2 files download --file-id <file-id> -o raw`

### `mosoo public-thread-api-v2 files list-files`

- Summary: List thread files
- HTTP: `GET /threads/{threadId}/files`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
- Output: list path `files`; columns `name`, `kind`, `id`, `committed`, `createdAt`, `mimeType`; response media `application/json`
- Example: `mosoo public-thread-api-v2 files list-files --thread-id <thread-id>`

### `mosoo public-thread-api-v2 files remove`

- Summary: Remove a thread file
- HTTP: `DELETE /threads/{threadId}/files/{fileId}`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
  - `--file-id` (path, required, ulid): File ID returned by add or list Thread files. v1 IDs are bare ULIDs.
- Output: response media `application/json`
- Example: `mosoo public-thread-api-v2 files remove --thread-id <thread-id> --file-id <file-id>`

### `mosoo public-thread-api-v2 files retrieve-file`

- Summary: Retrieve file metadata
- HTTP: `GET /files/{fileId}`
- Auth: required
- Body: none
- Flags:
  - `--file-id` (path, required, ulid): File ID returned by add or list Thread files. v1 IDs are bare ULIDs.
- Output: response media `application/json`
- Example: `mosoo public-thread-api-v2 files retrieve-file --file-id <file-id> -o json`

### `mosoo public-thread-api-v2 files upload`

- Summary: Upload a file to a Project
- HTTP: `POST /projects/{projectId}/files`
- Auth: required
- Body: required; media type `multipart/form-data`
- Flags:
  - `--project-id` (path, required, ulid): Owned Project. A Project key can access only its own Project; CLI login supplies an explicit owned Project.
  - `--file` (formData, required, binary): file
- Output: response media `application/json`
- Notes:
  - A Project key is restricted to its own Project; CLI login supplies an explicit owned Project.
  - The compatibility --agent-id form remains available. --project-id and --agent-id are mutually exclusive.
- Known errors:
  - HTTP 401: Invalid or revoked credential. Rotate the Project API key or run mosoo auth login again.
  - HTTP 400: The multipart request must contain exactly one file field.
  - HTTP 413: The upload exceeds the Public API file size limit.
- Examples:
  - Upload a Project file for a direct or preset Session.
    Command: `mosoo public-thread-api-v2 files upload --project-id <project-id> --file <path> -o json`
    Output ID path: `file.id`
    Follow-up commands:
      - `mosoo public-thread-api-v2 threads create --project-id <project-id> --file session.json -o json`

## Threads

### `mosoo public-thread-api-v2 threads archive`

- Summary: Archive a thread
- HTTP: `POST /threads/{threadId}/archive`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
- Output: response media `application/json`
- Example: `mosoo public-thread-api-v2 threads archive --thread-id <thread-id>`

### `mosoo public-thread-api-v2 threads create`

- Summary: Create a durable Session in a Project
- HTTP: `POST /projects/{projectId}/threads`
- Auth: required
- Body: required; media type `application/json`
- Flags:
  - `--project-id` (path, required, ulid): Owned Project. A Project key can access only its own Project; CLI login supplies an explicit owned Project.
  - `--idempotency-key` (header): Optional key for retry-safe create-thread and send-events calls. Reusing the same key with the same request returns the original response. Reusing the key while the original request is still processing returns 409.
- Output: response media `application/json`
- Notes:
  - Project credentials are BYOK. A Project key is restricted to its own Project; CLI login must supply an explicit owned Project. No Agent is created for inline execution.
  - Continue with events send --thread-id using the returned thread.id. Reuse the same idempotency key only with an unchanged request; changing configuration under that key returns 409.
  - The compatibility form threads create --agent-id <agent-id> retains the saved-private Agent route and optional body. --agent-id and --project-id are mutually exclusive; use configuration.type=agent for a Project-scoped preset.
- Known errors:
  - HTTP 400: The configuration is missing, mixes inline and preset fields, or has blank instructions; userId may also be invalid.
  - HTTP 404: Project or preset Agent not found or not owned by this caller.
  - HTTP 409: The idempotency key was reused with a different request or configuration.
- Examples:
  - Create a Session directly from harness/model/instructions and an uploaded Project file.
    Command: `mosoo public-thread-api-v2 threads create --project-id <project-id> --file session.json --idempotency-key <stable-create-key> -o json`
    Body shape: `{"configuration":{"harness":"openai-runtime","instructions":"Analyze the supplied material and save the results.","model":"\u003cmodel-id\u003e","provider":"openai","type":"inline"},"input":{"content":[{"text":"Summarize the attachment.","type":"text"}],"type":"user.message"},"resources":[{"file_id":"\u003cfile-id\u003e","type":"file"}]}`
    Output ID path: `thread.id`
    Follow-up commands:
      - `mosoo public-thread-api-v2 events send --thread-id <thread-id> --file events.json -o json`
  - Create an idle Session from an optional saved private Agent preset.
    Command: `mosoo public-thread-api-v2 threads create --project-id <project-id> --set configuration.type=agent --set-str configuration.agent_id=<agent-id> -o json`
    Body shape: `{"configuration":{"agent_id":"\u003cagent-id\u003e","type":"agent"}}`
    Output ID path: `thread.id`

### `mosoo public-thread-api-v2 threads delete`

- Summary: Delete a thread
- HTTP: `DELETE /threads/{threadId}`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
- Output: response media `application/json`
- Example: `mosoo public-thread-api-v2 threads delete --thread-id <thread-id>`

### `mosoo public-thread-api-v2 threads list-for-agent`

- Summary: List threads for an agent
- HTTP: `GET /agents/{agentId}/threads`
- Auth: required
- Body: none
- Flags:
  - `--agent-id` (path, required, ulid): Agent API Endpoint ID from the Agent's API Access panel. v1 IDs are bare ULIDs.
  - `--archived` (query): Filter by archived state: true returns only archived Threads, false only active ones. Omit to return all Threads.
- Output: list path `threads`; columns `kind`, `id`, `agent_id`, `created_at`, `last_run_id`, `source`; response media `application/json`
- Example: `mosoo public-thread-api-v2 threads list-for-agent --agent-id <agent-id>`

### `mosoo public-thread-api-v2 threads retrieve`

- Summary: Retrieve a thread
- HTTP: `GET /threads/{threadId}`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
- Output: response media `application/json`
- Example: `mosoo public-thread-api-v2 threads retrieve --thread-id <thread-id>`

### `mosoo public-thread-api-v2 threads unarchive`

- Summary: Unarchive a thread
- HTTP: `POST /threads/{threadId}/unarchive`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
- Output: response media `application/json`
- Example: `mosoo public-thread-api-v2 threads unarchive --thread-id <thread-id>`

### `mosoo public-thread-api-v2 threads usage`

- Summary: Read recorded Session usage
- HTTP: `GET /threads/{threadId}/usage`
- Auth: required
- Body: none
- Flags:
  - `--thread-id` (path, required, ulid): Thread ID returned by create thread. v1 IDs are bare ULIDs.
  - `--limit` (query, default `100`): Maximum usage observations per page.
  - `--after` (query, ulid): The nextCursor returned by the previous page.
- Output: list path `usage`; response media `application/json`; pagination `cursor`
- Example: `mosoo public-thread-api-v2 threads usage --thread-id <thread-id> -o json`
