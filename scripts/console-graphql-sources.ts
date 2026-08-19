import {
	agentGraphQLSpec,
	appGraphQLSpec,
	channelGraphQLSpec,
	commonGraphQLSpec,
	costGraphQLSpec,
	environmentGraphQLSpec,
	fileGraphQLSpec,
	mcpGraphQLSpec,
	onboardingGraphQLSpec,
	sessionGraphQLSpec,
	skillGraphQLSpec,
	userGraphQLSpec,
	vendorCredentialGraphQLSpec,
} from "../.cache/mosoo/apps/api/src/adapters/graphql/graphql-module-specs.ts";

// Console operations that exist in the pinned Mosoo schema but must not be
// generated as CLI commands. The App Deployment Secret surface was rolled back
// by langgenius/mosoo#560 (Multica YEF-1056); its operations are excluded here
// so the published CLI stops offering commands the API no longer serves. Drop
// those three entries once the pinned upstream commit no longer exposes them.
export const excludedOperations = new Set([
	"sendAgentSessionEvents",
	"appDeploymentSecretList",
	"deleteAppDeploymentSecret",
	"setAppDeploymentSecret",
]);

export const moduleGroups: { group: string; spec: { queryFields?: string[]; mutationFields?: string[] } }[] = [
	{ group: "Common", spec: commonGraphQLSpec },
	{ group: "Agents", spec: agentGraphQLSpec },
	{ group: "Channels", spec: channelGraphQLSpec },
	{ group: "Cost", spec: costGraphQLSpec },
	{ group: "Environments", spec: environmentGraphQLSpec },
	{ group: "Files", spec: fileGraphQLSpec },
	{ group: "MCP", spec: mcpGraphQLSpec },
	{ group: "Onboarding", spec: onboardingGraphQLSpec },
	{ group: "Apps", spec: appGraphQLSpec },
	{ group: "Sessions", spec: sessionGraphQLSpec },
	{ group: "Skills", spec: skillGraphQLSpec },
	{ group: "User", spec: userGraphQLSpec },
	{ group: "Credentials", spec: vendorCredentialGraphQLSpec },
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
