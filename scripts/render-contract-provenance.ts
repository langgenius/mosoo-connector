import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "..");
const mosooRoot = resolve(repositoryRoot, ".cache/mosoo");
const openAPIPath = resolve(mosooRoot, "docs/openapi/public-thread-api.openapi.json");
const requestedCommit = process.env.MOSOO_REF;
const repository = process.env.MOSOO_REPO_URL ?? "https://github.com/langgenius/mosoo.git";

if (requestedCommit === undefined || !/^[0-9a-f]{40}$/.test(requestedCommit)) {
	throw new Error("MOSOO_REF must be an explicit 40-character Mosoo commit SHA");
}

const actualCommit = execFileSync("git", ["-C", mosooRoot, "rev-parse", "HEAD"], {
	encoding: "utf8",
}).trim();
if (actualCommit !== requestedCommit) {
	throw new Error(`Mosoo checkout is ${actualCommit}, expected ${requestedCommit}`);
}

function canonicalJSON(value: unknown): string {
	if (value === null || typeof value === "boolean" || typeof value === "string") {
		return JSON.stringify(value);
	}
	if (typeof value === "number") {
		if (!Number.isFinite(value)) {
			throw new Error("OpenAPI JSON contains a non-finite number");
		}
		return JSON.stringify(value);
	}
	if (Array.isArray(value)) {
		return `[${value.map(canonicalJSON).join(",")}]`;
	}
	if (typeof value === "object") {
		const record = value as Record<string, unknown>;
		return `{${Object.keys(record)
			.sort()
			.map((key) => `${JSON.stringify(key)}:${canonicalJSON(record[key])}`)
			.join(",")}}`;
	}
	throw new Error(`OpenAPI JSON contains unsupported value type ${typeof value}`);
}

const openAPI = JSON.parse(await readFile(openAPIPath, "utf8")) as unknown;
const digest = createHash("sha256").update(canonicalJSON(openAPI)).digest("hex");
const provenance = {
	schemaVersion: 1,
	repository,
	upstreamCommit: actualCommit,
	publicThreadOpenAPI: {
		sha256: digest,
		normalization: "json-sort-keys-v1",
	},
};
const rendered = `${JSON.stringify(provenance, null, 2)}\n`;
const outputs = [
	resolve(repositoryRoot, "specs/mosoo-contract.json"),
	resolve(repositoryRoot, "internal/contractprovenance/provenance.json"),
	resolve(repositoryRoot, "publish/skills/mosoo/references/provenance.json"),
];

for (const output of outputs) {
	await mkdir(dirname(output), { recursive: true });
	await writeFile(output, rendered, "utf8");
}
console.log(`recorded Mosoo ${actualCommit} with normalized Public Thread OpenAPI sha256 ${digest}`);
