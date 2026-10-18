import { test } from "node:test";
import assert from "node:assert/strict";
import { verifyVersions, verifyTestedCommit } from "./verify-release.mjs";
import { readFileSync } from "node:fs";
import { mkdtempSync, writeFileSync, unlinkSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { verifyInstallers } from "./verify-installers.mjs";

test("all application version files match and other tags are refused", () => {
  const version = JSON.parse(readFileSync("package.json", "utf8")).version;
  assert.equal(verifyVersions(".", `v${version}`).version, version);
  assert.throws(() => verifyVersions(".", "v999.0.0"));
});

test("release refuses missing installers and creates checksums for the complete set", () => {
  const dir = mkdtempSync(join(tmpdir(), "commitdeck-release-test-"));
  try {
    for (const p of [
      "CommitDeck_aarch64.dmg",
      "CommitDeck_x64.dmg",
      "CommitDeck.exe",
      "commitdeck.deb",
    ])
      writeFileSync(join(dir, p), "synthetic installer fixture");
    verifyInstallers(dir);
    assert.equal(
      readFileSync(join(dir, "SHA256SUMS"), "utf8").trim().split("\n").length,
      4,
    );
    unlinkSync(join(dir, "CommitDeck.exe"));
    assert.throws(() => verifyInstallers(dir));
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
test("release requires successful main push CI on the exact commit", () => {
  const commit = "a".repeat(40);
  const run = {
    head_sha: commit,
    head_branch: "main",
    event: "push",
    path: ".github/workflows/ci.yml",
    status: "completed",
    conclusion: "success",
  };
  verifyTestedCommit({ workflow_runs: [run] }, commit);
  for (const change of [
    { head_sha: "b".repeat(40) },
    { event: "pull_request" },
    { head_branch: "feature" },
    { conclusion: "failure" },
    { status: "in_progress" },
    { path: ".github/workflows/release.yml" },
  ])
    assert.throws(() =>
      verifyTestedCommit({ workflow_runs: [{ ...run, ...change }] }, commit),
    );
  assert.throws(() => verifyTestedCommit({ workflow_runs: [] }, commit));
});
