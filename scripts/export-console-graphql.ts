import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { schema } from "../.cache/mosoo/apps/api/src/adapters/graphql/codegen-schema.ts";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "..");
const outputPath = resolve(repositoryRoot, ".cache/mosoo/docs/graphql/console.graphql");

const sdl = `${schema.trimEnd()}\n`;
await mkdir(dirname(outputPath), { recursive: true });
await writeFile(outputPath, sdl, "utf8");
console.log(`wrote ${outputPath}`);
