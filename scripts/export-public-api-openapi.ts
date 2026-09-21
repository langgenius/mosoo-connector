import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { createPublicApiOpenApiDocument } from "../.cache/mosoo/apps/api/src/adapters/http/routes/public-api-openapi.ts";

type HttpMethod = "delete" | "get" | "post";

interface OperationMetadata {
	operationId: string;
	tags: string[];
}

const operationMetadata: Record<string, Partial<Record<HttpMethod, OperationMetadata>>> = {
	"/agents/{agentId}/files": {
		post: { operationId: "AgentFiles_Upload", tags: ["Files"] },
	},
	"/agents/{agentId}/threads": {
		get: { operationId: "Threads_ListForAgent", tags: ["Threads"] },
		post: { operationId: "Threads_Create", tags: ["Threads"] },
	},
	"/files/{fileId}": {
		delete: { operationId: "PublicFiles_DeleteFile", tags: ["Files"] },
		get: { operationId: "PublicFiles_RetrieveFile", tags: ["Files"] },
	},
	"/files/{fileId}/content": {
		get: { operationId: "PublicFiles_Download", tags: ["Files"] },
	},
	"/threads/{threadId}": {
		delete: { operationId: "Threads_Delete", tags: ["Threads"] },
		get: { operationId: "Threads_Retrieve", tags: ["Threads"] },
	},
	"/threads/{threadId}/archive": {
		post: { operationId: "Threads_Archive", tags: ["Threads"] },
	},
	"/threads/{threadId}/events": {
		get: { operationId: "ThreadEvents_ListEvents", tags: ["Events"] },
		post: { operationId: "ThreadEvents_Send", tags: ["Events"] },
	},
	"/threads/{threadId}/events/stream": {
		get: { operationId: "ThreadEvents_Stream", tags: ["Events"] },
	},
	"/threads/{threadId}/files": {
		get: { operationId: "ThreadFiles_ListFiles", tags: ["Files"] },
	},
	"/threads/{threadId}/files/{fileId}": {
		delete: { operationId: "ThreadFiles_Remove", tags: ["Files"] },
	},
	"/threads/{threadId}/unarchive": {
		post: { operationId: "Threads_Unarchive", tags: ["Threads"] },
	},
};

const v2OperationMetadata: typeof operationMetadata = {
	"/projects/{projectId}/threads": {
		post: { operationId: "Threads_CreateInProject", tags: ["Threads"] },
	},
	"/projects/{projectId}/files": {
		post: { operationId: "ProjectFiles_Upload", tags: ["Files"] },
	},
	"/threads/{threadId}/usage": {
		get: { operationId: "Threads_Usage", tags: ["Threads"] },
	},
};

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "..");
for (const version of ["v1", "v2"] as const) {
const outputPath = resolve(repositoryRoot, `.cache/mosoo/docs/openapi/public-thread-api${version === "v2" ? ".v2" : ""}.openapi.json`);
const committedOrigin = "https://cloud.mosoo.ai";

const document = createPublicApiOpenApiDocument(committedOrigin, version);
// Target resolution owns the API version base; keep operation paths relative to it.
document.servers = [];
for (const [path, methods] of Object.entries({ ...operationMetadata, ...(version === "v2" ? v2OperationMetadata : {}) })) {
	const pathItem = document.paths[path];
	if (pathItem === undefined) {
		throw new Error(`OpenAPI path missing: ${path}`);
	}

	for (const [method, metadata] of Object.entries(methods) as [HttpMethod, OperationMetadata][]) {
		const operation = pathItem[method];
		if (operation === undefined) {
			throw new Error(`OpenAPI operation missing: ${method.toUpperCase()} ${path}`);
		}

		Object.assign(operation, metadata);
	}
}
await mkdir(dirname(outputPath), { recursive: true });
await writeFile(outputPath, `${JSON.stringify(document, null, 2)}\n`, "utf8");
console.log(`wrote ${outputPath}`);

}
