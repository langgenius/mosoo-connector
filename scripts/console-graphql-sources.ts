import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

// Console operation inventory for the pinned Mosoo schema, grouped the way the
// generated CLI command tree is organized. This map is maintained HERE (not
// imported from Mosoo internals) so that Host-side refactors of the GraphQL
// module wiring cannot silently break CLI codegen. The guardrail below diffs
// this map against the freshly exported console SDL on every codegen run and
// fails loudly when an operation is added or removed upstream, pointing the
// maintainer at exactly what to update.

export const excludedOperations = new Set(["sendAgentSessionEvents"]);

export const moduleGroups: {
	group: string;
	spec: { queryFields?: string[]; mutationFields?: string[] };
}[] = [
	{
		group: "Agents",
		spec: {
			queryFields: ["accessibleAgentList", "agent", "agentEditorState", "agentManifest", "exportAgentPackage"],
			mutationFields: [
				"createAgentFork",
				"createAgent",
				"deleteAgent",
				"importAgentPackage",
				"publishAgent",
				"unpublishAgent",
				"updateAgentConfig",
			],
		},
	},
	{
		group: "Cost",
		spec: {
			queryFields: ["agentCostCard", "projectCostCard"],
		},
	},
	{
		group: "Environments",
		spec: {
			queryFields: ["environment", "projectEnvironmentList"],
			mutationFields: ["createEnvironment", "deleteEnvironment", "setProjectDefaultEnvironment", "updateEnvironment"],
		},
	},
	{
		group: "Files",
		spec: {
			queryFields: ["fileList"],
		},
	},
	{
		group: "MCP",
		spec: {
			queryFields: ["mcpOAuthFlowStatus", "mcpRegistry"],
			mutationFields: [
				"connectMcpBearer",
				"createProjectMcpServer",
				"deleteMcpServer",
				"revokeMcpCredential",
				"setMcpServerEnabled",
				"startMcpOAuth",
				"updateProjectMcpServer",
			],
		},
	},
	{
		group: "Onboarding",
		spec: {
			mutationFields: ["onboardingBootstrap"],
		},
	},
	{
		group: "Projects",
		spec: {
			queryFields: ["projectList"],
			mutationFields: ["createProject", "renameProject"],
		},
	},
	{
		group: "Sessions",
		spec: {
			queryFields: [
				"agentSessionDiagnostics",
				"threadAgentSessionList",
				"threadAgentSessionRetrieve",
				"threadSessionMessages",
				"threadSessionProcessEvents",
				"agentSessionList",
			],
			mutationFields: [
				"restartSessionDriver",
				"recreateSessionSandbox",
				"addSessionResource",
				"createAgentSession",
				"prewarmAgentSession",
				"sendAgentSessionEvents",
				"archiveAgentSession",
				"deleteAgentSession",
				"unarchiveAgentSession",
			],
		},
	},
	{
		group: "Skills",
		spec: {
			queryFields: ["projectSkillList", "skillDetail"],
			mutationFields: ["createSkillFork", "deleteOwnedSkill"],
		},
	},
	{
		group: "User",
		spec: {
			queryFields: ["viewer"],
			mutationFields: ["updateProfile"],
		},
	},
	{
		group: "Credentials",
		spec: {
			queryFields: ["availableAgentModels", "vendorCredentialList"],
			mutationFields: [
				"createVendorCredential",
				"deleteVendorCredential",
				"setDefaultVendorCredential",
				"testVendorCredential",
				"updateVendorCredential",
			],
		},
	},
	{
		group: "Organization",
		spec: {
			mutationFields: ["renameOrganization"],
		},
	},
];

function operationName(field: string): string {
	const match = field.trim().match(/^(\w+)/);
	if (!match) {
		throw new Error(`invalid GraphQL field declaration: ${field}`);
	}
	return match[1];
}

function listFields(fields: string[] | undefined): string[] {
	return fields?.map(operationName) ?? [];
}

export { listFields };

export function collectConsoleGraphQLOperations(): { group: string; field: string }[] {
	const operations: { group: string; field: string }[] = [];
	for (const { group, spec } of moduleGroups) {
		for (const field of [...listFields(spec.queryFields), ...listFields(spec.mutationFields)]) {
			if (excludedOperations.has(field)) {
				continue;
			}
			operations.push({ group, field });
		}
	}
	return operations;
}

// --- Drift guardrail -------------------------------------------------------
// Parse the exported console SDL (.cache/mosoo/docs/graphql/console.graphql,
// produced by export-console-graphql.ts earlier in the codegen pipeline) and
// require an exact match between its Query/Mutation root fields and this map,
// so a Host schema change surfaces as a loud codegen failure rather than a
// silently diverged CLI.

function rootFieldsFromSdl(sdl: string, typeName: string): string[] {
	const header = new RegExp(`(?:^|\\n)(?:extend\\s+)?type ${typeName}\\s*\\{`, "g");
	const names: string[] = [];
	let headerMatch: RegExpExecArray | null;
	while ((headerMatch = header.exec(sdl)) !== null) {
		const start = header.lastIndex;
		let depth = 1;
		let i = start;
		for (; i < sdl.length && depth > 0; i++) {
			if (sdl[i] === "{") depth++;
			else if (sdl[i] === "}") depth--;
		}
		// Strip parenthesized argument lists so a multi-line arg cannot be
		// mistaken for a root field, then read the leading identifier of each
		// `name: ReturnType` declaration.
		const block = sdl.slice(start, i - 1).replace(/\([^()]*\)/g, "");
		for (const fieldMatch of block.matchAll(/(?:^|\n)\s*([a-zA-Z]\w*)\s*:/g)) {
			names.push(fieldMatch[1]);
		}
	}
	return names;
}

function assertMapMatchesSchema(): void {
	const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
	const sdlPath = resolve(repositoryRoot, ".cache/mosoo/docs/graphql/console.graphql");
	let sdl: string;
	try {
		sdl = readFileSync(sdlPath, "utf8");
	} catch {
		throw new Error(`console SDL not found at ${sdlPath}; run export-console-graphql.ts before this step`);
	}
	const schemaOps = new Set([...rootFieldsFromSdl(sdl, "Query"), ...rootFieldsFromSdl(sdl, "Mutation")]);
	const mapOps = new Set<string>();
	for (const { spec } of moduleGroups) {
		for (const field of [...listFields(spec.queryFields), ...listFields(spec.mutationFields)]) {
			mapOps.add(field);
		}
	}
	const missingFromMap = [...schemaOps].filter((op) => !mapOps.has(op)).sort();
	const missingFromSchema = [...mapOps].filter((op) => !schemaOps.has(op)).sort();
	if (missingFromMap.length > 0 || missingFromSchema.length > 0) {
		const lines = ["console operation map is out of sync with the exported schema."];
		if (missingFromMap.length > 0) {
			lines.push(`  added upstream (add to moduleGroups): ${missingFromMap.join(", ")}`);
		}
		if (missingFromSchema.length > 0) {
			lines.push(`  removed upstream (drop from moduleGroups): ${missingFromSchema.join(", ")}`);
		}
		throw new Error(lines.join("\n"));
	}
}

assertMapMatchesSchema();
