import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
const triple =
  process.env.TAURI_ENV_TARGET_TRIPLE ||
  execFileSync("rustc", ["--print", "host-tuple"], { encoding: "utf8" }).trim();
const platform = triple.includes("windows")
  ? "windows"
  : triple.includes("apple")
    ? "darwin"
    : "linux";
const arch = triple.startsWith("aarch64")
  ? "arm64"
  : triple.startsWith("x86_64")
    ? "amd64"
    : null;
if (!arch) throw new Error(`Unsupported sidecar target: ${triple}`);
mkdirSync("src-tauri/binaries", { recursive: true });
mkdirSync("bin", { recursive: true });
execFileSync(
  "go",
  [
    "build",
    "-trimpath",
    "-o",
    `src-tauri/binaries/commitdeck-core-${triple}${platform === "windows" ? ".exe" : ""}`,
    "./cmd/commitdeck",
  ],
  {
    stdio: "inherit",
    env: { ...process.env, GOOS: platform, GOARCH: arch, CGO_ENABLED: "0" },
  },
);
execFileSync(
  "go",
  [
    "build",
    "-o",
    `bin/commitdeck-core${process.platform === "win32" ? ".exe" : ""}`,
    "./cmd/commitdeck",
  ],
  {
    stdio: "inherit",
  },
);
