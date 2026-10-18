import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { createHash } from "node:crypto";
import assert from "node:assert/strict";
import { pathToFileURL } from "node:url";

export function verifyInstallers(dir) {
  const files = readdirSync(dir).filter((p) => /\.(dmg|deb|exe)$/.test(p));
  for (const pattern of [/aarch64\.dmg$/, /x64\.dmg$/, /\.deb$/, /\.exe$/]) {
    assert.equal(
      files.filter((p) => pattern.test(p)).length,
      1,
      `Expected one installer matching ${pattern}`,
    );
  }
  assert.equal(files.length, 4);
  const sums = files.sort().map((p) => {
    assert.ok(statSync(join(dir, p)).size > 0, "Empty installer");
    return `${createHash("sha256")
      .update(readFileSync(join(dir, p)))
      .digest("hex")}  ${p}`;
  });
  writeFileSync(join(dir, "SHA256SUMS"), sums.join("\n") + "\n");
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href)
  verifyInstallers(process.argv[2]);
