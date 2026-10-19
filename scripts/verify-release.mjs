import assert from "node:assert/strict";
import { readFileSync, appendFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

export function verifyVersions(root, tag) {
  const json = (path) => JSON.parse(readFileSync(`${root}/${path}`, "utf8"));
  const version = json("package.json").version;
  assert.match(version, /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/);
  assert.equal(tag, `v${version}`, "Tag must match the application version");
  assert.equal(json("package-lock.json").version, version);
  assert.equal(json("package-lock.json").packages[""].version, version);
  assert.equal(json("src-tauri/tauri.conf.json").version, version);
  assert.equal(
    readFileSync(`${root}/src-tauri/Cargo.toml`, "utf8").match(
      /^version\s*=\s*"([^"]+)"/m,
    )?.[1],
    version,
  );
  assert.equal(
    readFileSync(`${root}/src-tauri/Cargo.lock`, "utf8")
      .replaceAll("\r\n", "\n")
      .match(/\[\[package\]\]\nname = "commitdeck"\nversion = "([^"]+)"/)?.[1],
    version,
  );
  return { version, prerelease: version.includes("-") };
}

export function verifyTestedCommit(runs, commit) {
  assert.match(commit, /^[a-f0-9]{40}$/);
  assert.ok(
    runs.workflow_runs.some(
      (r) =>
        r.head_sha === commit &&
        r.head_branch === "main" &&
        r.event === "push" &&
        r.path === ".github/workflows/ci.yml" &&
        r.status === "completed" &&
        r.conclusion === "success",
    ),
    "Release commit must have a successful completed main push CI run",
  );
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  const { prerelease } = verifyVersions(process.cwd(), process.env.RELEASE_TAG);
  if (process.env.RELEASE_RUNS_FILE)
    verifyTestedCommit(
      JSON.parse(readFileSync(process.env.RELEASE_RUNS_FILE, "utf8")),
      process.env.RELEASE_COMMIT,
    );
  if (process.env.GITHUB_OUTPUT)
    appendFileSync(
      process.env.GITHUB_OUTPUT,
      `tag=${process.env.RELEASE_TAG}\nprerelease=${prerelease}\n`,
    );
}
