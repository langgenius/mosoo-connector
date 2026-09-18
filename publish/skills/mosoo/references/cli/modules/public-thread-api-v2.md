# Module `public-thread-api-v2`

## Source

- Backend: `openapi3`
- Default hostname: `http://127.0.0.1:8787/api/v2`
- Repository: https://github.com/langgenius/mosoo.git
- Pinned tag: `e060f465ac24a11fe7efe0cf55fde78042f42e2f`
- Files: `docs/openapi/public-thread-api.v2.openapi.json`
- Resolved SHA: `e060f465ac24a11fe7efe0cf55fde78042f42e2f`

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
- Notes:
  - Set top-level maxCostUsd with --set maxCostUsd=<usd-amount> or a numeric JSON field in --file. Choose a positive USD amount with at most six decimal places, within the deployment policy maximum.
  - The cap applies only to this turn. Omission uses the deployment's configured default when available; the CLI does not supply a default amount or platform-funded inference.
  - Per-turn budgets are unreleased. Check maxCostUsd in the target's /api/v2/openapi.json and confirm its deployment budget policy before use. An explicit cap without a policy fails with 409 readiness_blocked.
  - Costs are estimates, not invoices. In-flight usage can exceed the cap; reaching the threshold rejects new model requests, and unknown usage fails closed. Native provider protocols are unchanged.
  - Budgeted runs expose run.budget with capUsd, estimatedCostUsd and state: available, settling, budget_exhausted or budget_usage_unavailable. Unavailable usage means the estimate covers only established usage.
  - A budget failure remains a failed run. Inspect available files and events; --final-output is only for completed runs, and a failed turn does not guarantee a successful checkpoint.
- Known errors:
  - HTTP 401: Invalid or revoked credential. Rotate the Project API key or run mosoo auth login again.
  - HTTP 409: Idempotency key reused while the original request is still processing.
  - HTTP 400: maxCostUsd is invalid, exceeds the deployment maximum, or is supplied without a user_message event.
  - HTTP 409: readiness_blocked: an explicit maxCostUsd was supplied but this deployment has no budget policy.
- Examples:
  - Send a user message event to an existing thread.
    Command: `mosoo public-thread-api-v2 events send --thread-id <thread-id> --file events.json -o json`
    Body shape: `{"events":[{"requestId":"cli-send-001","text":"Continue the task with this follow-up.","type":"user_message"}]}`
    Output list path: `events`
    Follow-up commands:
      - `mosoo public-thread-api-v2 events list-events --thread-id <thread-id> -o json`
  - Send a new user-message turn with a caller-selected estimate cap; set TURN_MAX_COST_USD first.
    Command: `mosoo public-thread-api-v2 events send --thread-id <thread-id> --set "events[0].type=user_message" --set-str "events[0].text=Continue this task." --set "maxCostUsd=$TURN_MAX_COST_USD" -o json`
    Output list path: `events`

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

- Summary: Upload a file for an agent
- HTTP: `POST /agents/{agentId}/files`
- Auth: required
- Body: required; media type `multipart/form-data`
- Flags:
  - `--agent-id` (path, required, ulid): Agent API Endpoint ID from the Agent's API Access panel. v1 IDs are bare ULIDs.
  - `--file` (formData, required, binary): file
- Output: response media `application/json`
- Known errors:
  - HTTP 401: Invalid or revoked credential. Rotate the Project API key or run mosoo auth login again.
  - HTTP 400: The multipart request must contain exactly one file field.
  - HTTP 413: The upload exceeds the Public API file size limit.
- Examples:
  - Upload a file and capture the draft file ID for a thread request.
    Command: `mosoo public-thread-api-v2 files upload --agent-id <agent-id> --file <path> -o json`
    Output ID path: `file.id`
    Follow-up commands:
      - `mosoo public-thread-api-v2 threads create --agent-id <agent-id> --file thread-create.json -o json`

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

- Summary: Create a thread for an agent
- HTTP: `POST /agents/{agentId}/threads`
- Auth: required
- Body: optional; media type `application/json`
- Flags:
  - `--agent-id` (path, required, ulid): Agent API Endpoint ID from the Agent's API Access panel. v1 IDs are bare ULIDs.
  - `--idempotency-key` (header): Optional key for retry-safe create-thread and send-events calls. Reusing the same key with the same request returns the original response. Reusing the key while the original request is still processing returns 409.
- Output: response media `application/json`
- Notes:
  - Set top-level maxCostUsd with --set maxCostUsd=<usd-amount> or a numeric JSON field in --file. Choose a positive USD amount with at most six decimal places, within the deployment policy maximum.
  - The cap applies only to this turn. Omission uses the deployment's configured default when available; the CLI does not supply a default amount or platform-funded inference.
  - Per-turn budgets are unreleased. Check maxCostUsd in the target's /api/v2/openapi.json and confirm its deployment budget policy before use. An explicit cap without a policy fails with 409 readiness_blocked.
  - Costs are estimates, not invoices. In-flight usage can exceed the cap; reaching the threshold rejects new model requests, and unknown usage fails closed. Native provider protocols are unchanged.
  - Budgeted runs expose run.budget with capUsd, estimatedCostUsd and state: available, settling, budget_exhausted or budget_usage_unavailable. Unavailable usage means the estimate covers only established usage.
  - A budget failure remains a failed run. Inspect available files and events; --final-output is only for completed runs, and a failed turn does not guarantee a successful checkpoint.
- Known errors:
  - HTTP 400: The body or userId is invalid, or maxCostUsd is invalid, exceeds the deployment maximum, or is supplied without input.
  - HTTP 404: Agent not found or outside this Project.
  - HTTP 409: readiness_blocked: an explicit maxCostUsd was supplied but this deployment has no budget policy.
- Examples:
  - Create a Thread with an initial user message and capture the Thread ID.
    Command: `mosoo public-thread-api-v2 threads create --agent-id <agent-id> --file thread-create.json -o json`
    Body shape: `{"input":{"content":[{"text":"Say hello from the API.","type":"text"}],"type":"user.message"}}`
    Output ID path: `thread.id`
    Follow-up commands:
      - `mosoo public-thread-api-v2 threads retrieve --thread-id <thread-id> -o json`
      - `mosoo public-thread-api-v2 events list-events --thread-id <thread-id> -o json`
  - Create a Thread with a file uploaded through the Agent endpoint.
    Command: `mosoo public-thread-api-v2 threads create --agent-id <agent-id> --file thread-create-with-file.json -o json`
    Body shape: `{"input":{"content":[{"text":"Summarize the attachment.","type":"text"}],"type":"user.message"},"resources":[{"file_id":"\u003cfile-id\u003e","type":"file"}]}`
    Output ID path: `thread.id`
    Follow-up commands:
      - `mosoo public-thread-api-v2 events list-events --thread-id <thread-id> -o json`
  - Create an initial turn with a caller-selected estimate cap; set TURN_MAX_COST_USD to your chosen amount first.
    Command: `mosoo public-thread-api-v2 threads create --agent-id <agent-id> --set input.type=user.message --set "input.content[0].type=text" --set-str "input.content[0].text=Start this turn." --set "maxCostUsd=$TURN_MAX_COST_USD" -o json`
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
- Notes:
  - Costs are estimates, not invoices. In-flight usage can exceed the cap; reaching the threshold rejects new model requests, and unknown usage fails closed. Native provider protocols are unchanged.
  - Budgeted runs expose run.budget with capUsd, estimatedCostUsd and state: available, settling, budget_exhausted or budget_usage_unavailable. Unavailable usage means the estimate covers only established usage.
  - A budget failure remains a failed run. Inspect available files and events; --final-output is only for completed runs, and a failed turn does not guarantee a successful checkpoint.
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
