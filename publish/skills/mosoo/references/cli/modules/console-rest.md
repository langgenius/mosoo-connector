# Module `console-rest`

## Source

- Backend: `openapi3`
- Default hostname: `http://127.0.0.1:8787/api`
- Repository: https://github.com/langgenius/mosoo.git
- Pinned tag: `51056b328b24c707fc5b8edfdc868b8f01d72527`
- Files: `docs/openapi/console-rest.openapi.json`
- Resolved SHA: `51056b328b24c707fc5b8edfdc868b8f01d72527`

## Access Tokens

### `mosoo console-rest access create`

- Summary: Create a Project API key
- HTTP: `POST /access-tokens`
- Auth: required
- Body: required; media type `application/json`
- Flags: none
- Output: response media `application/json`
- Notes:
  - Creates a Project API key. Authenticate through mosoo auth login first; the secret value is returned once.
- Example: `mosoo console-rest access create --set projectId=<project-id> --set label="ci"`

### `mosoo console-rest access list`

- Summary: List Project API keys
- HTTP: `GET /access-tokens`
- Auth: required
- Body: none
- Flags:
  - `--project-id` (query, required, ulid): Project whose API keys are being managed.
- Output: list path `tokens`; response media `application/json`
- Example: `mosoo console-rest access list --project-id <project-id>`

### `mosoo console-rest access revoke`

- Summary: Revoke a Project API key
- HTTP: `DELETE /access-tokens/{tokenId}`
- Auth: required
- Body: none
- Flags:
  - `--token-id` (path, required, ulid): Project API key ID.
- Output: response media `application/json`
- Example: `mosoo console-rest access revoke --token-id <token-id>`

## Files

### `mosoo console-rest files abort-upload`

- Summary: Abort a pending upload
- HTTP: `DELETE /files/{fileId}/upload`
- Auth: required
- Body: none
- Flags:
  - `--file-id` (path, required, ulid): File ID.
- Output: response media `application/json`

### `mosoo console-rest files complete-upload`

- Summary: Complete a multipart upload
- HTTP: `POST /files/{fileId}/complete`
- Auth: required
- Body: required; media type `application/json`
- Flags:
  - `--file-id` (path, required, ulid): File ID.
- Output: response media `application/json`

### `mosoo console-rest files create-upload`

- Summary: Create a file upload
- HTTP: `POST /files`
- Auth: required
- Body: required; media type `application/json`
- Flags: none
- Output: response media `application/json`

### `mosoo console-rest files delete`

- Summary: Delete a file
- HTTP: `DELETE /files/{fileId}`
- Auth: required
- Body: none
- Flags:
  - `--file-id` (path, required, ulid): File ID.
  - `--if-match` (header): Optional optimistic concurrency token.
- Output: response media `application/json`

### `mosoo console-rest files download-content`

- Summary: Download file content
- HTTP: `GET /files/{fileId}/content`
- Auth: required
- Body: none
- Flags:
  - `--file-id` (path, required, ulid): File ID.
  - `--disposition` (query, one of: attachment|inline): Content disposition hint.
- Output: response media `application/octet-stream`

### `mosoo console-rest files get-upload`

- Summary: Get upload session status
- HTTP: `GET /files/{fileId}/upload`
- Auth: required
- Body: none
- Flags:
  - `--file-id` (path, required, ulid): File ID.
- Output: response media `application/json`

### `mosoo console-rest files list`

- Summary: List files for a Project or session
- HTTP: `GET /files`
- Auth: required
- Body: none
- Flags:
  - `--project-id` (query, required, ulid): Project ID that owns the files.
  - `--session-id` (query, ulid): Optional session ID filter.
  - `--session-kind` (query, one of: artifact|attachment|all): Optional session file kind filter.
- Output: list path `files`; response media `application/json`
- Example: `mosoo console-rest access list --project-id <project-id>`

### `mosoo console-rest files update`

- Summary: Update file metadata
- HTTP: `PATCH /files/{fileId}`
- Auth: required
- Body: required; media type `application/json`
- Flags:
  - `--file-id` (path, required, ulid): File ID.
- Output: response media `application/json`

### `mosoo console-rest files upload-content`

- Summary: Upload file content in a single request
- HTTP: `PUT /files/{fileId}/content`
- Auth: required
- Body: required; media type `application/octet-stream`
- Flags:
  - `--file-id` (path, required, ulid): File ID.
- Output: response media `application/json`

### `mosoo console-rest files upload-part`

- Summary: Upload one multipart part
- HTTP: `PUT /files/{fileId}/parts/{partNumber}`
- Auth: required
- Body: required; media type `application/octet-stream`
- Flags:
  - `--file-id` (path, required, ulid): File ID.
  - `--part-number` (path, required): 1-based multipart part number.
- Output: response media `application/json`

## Skills

### `mosoo console-rest skills download-package`

- Summary: Download a skill package archive
- HTTP: `GET /skill/{skillId}/package`
- Auth: required
- Body: none
- Flags:
  - `--skill-id` (path, required, ulid): Skill ID.
  - `--project-id` (query, required, ulid): Project ID that owns the skill.
- Output: response media `application/zip`

### `mosoo console-rest skills download-source`

- Summary: Download skill source markdown
- HTTP: `GET /skill/{skillId}/source`
- Auth: required
- Body: none
- Flags:
  - `--skill-id` (path, required, ulid): Skill ID.
  - `--project-id` (query, required, ulid): Project ID that owns the skill.
- Output: response media `text/markdown`

### `mosoo console-rest skills inspect`

- Summary: Inspect a skill package from upload or GitHub URL
- HTTP: `POST /skill/inspect`
- Auth: required
- Body: required; media type `multipart/form-data`
- Flags:
  - `--file` (formData, binary): file
  - `--github-url` (formData): githubUrl
- Output: response media `application/json`

### `mosoo console-rest skills package`

- Summary: Create or update a skill package upload
- HTTP: `POST /skill/package`
- Auth: required
- Body: required; media type `multipart/form-data`
- Flags:
  - `--file` (formData, binary): file
  - `--github-url` (formData): githubUrl
  - `--project-id` (formData, required, ulid): projectId
  - `--skill-id` (formData, ulid): skillId
- Output: response media `application/json`
