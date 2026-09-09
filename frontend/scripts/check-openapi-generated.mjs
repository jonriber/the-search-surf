import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, relative, resolve } from "node:path";

const temporaryRoot = mkdtempSync(join(tmpdir(), "the-search-openapi-"));
const generatedDirectory = resolve("src/api/generated");
const temporaryOutput = join(temporaryRoot, "generated");

try {
  const generation = spawnSync(
    resolve("node_modules/.bin/openapi-ts"),
    ["--no-log-file"],
    {
      env: { ...process.env, THE_SEARCH_OPENAPI_OUTPUT: temporaryOutput },
      encoding: "utf8",
    },
  );

  if (generation.status !== 0) {
    process.stderr.write(generation.stderr);
    process.exit(generation.status ?? 1);
  }

  const expectedFiles = listFiles(temporaryOutput);
  const committedFiles = listFiles(generatedDirectory);
  const paths = new Set([...expectedFiles, ...committedFiles]);
  const differences = [...paths].filter((path) => {
    try {
      return (
        readFileSync(join(temporaryOutput, path), "utf8") !==
        readFileSync(join(generatedDirectory, path), "utf8")
      );
    } catch {
      return true;
    }
  });

  if (differences.length > 0) {
    process.stderr.write(
      `Generated API contract is stale:\n${differences.map((path) => `- ${path}`).join("\n")}\nRun npm run api:generate and commit the result.\n`,
    );
    process.exit(1);
  }
} finally {
  rmSync(temporaryRoot, { recursive: true, force: true });
}

function listFiles(root) {
  return readdirSync(root, { recursive: true, withFileTypes: true })
    .filter((entry) => entry.isFile())
    .map((entry) => relative(root, join(entry.parentPath, entry.name)))
    .sort();
}
