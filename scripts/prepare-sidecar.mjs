import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = join(__dirname, "..");

function detectTargetTriple() {
  const rustInfo = execFileSync("rustc", ["-vV"], {
    cwd: root,
    encoding: "utf8",
  });
  const match = rustInfo.match(/^host:\s+(\S+)$/m);
  if (!match) {
    throw new Error("failed to detect rust target triple");
  }
  return match[1];
}

function goPlatformForTarget(target) {
  let goos = "";
  if (target.includes("windows")) {
    goos = "windows";
  } else if (target.includes("apple-darwin")) {
    goos = "darwin";
  } else if (target.includes("linux")) {
    goos = "linux";
  } else {
    throw new Error(`unsupported target triple: ${target}`);
  }

  let goarch = "";
  if (target.startsWith("aarch64")) {
    goarch = "arm64";
  } else if (target.startsWith("x86_64")) {
    goarch = "amd64";
  } else if (target.startsWith("i686")) {
    goarch = "386";
  } else {
    throw new Error(`unsupported target architecture: ${target}`);
  }

  return { goos, goarch };
}

function main() {
  const targetTriple = process.env.TAURI_TARGET_TRIPLE || detectTargetTriple();
  const { goos, goarch } = goPlatformForTarget(targetTriple);
  const extension = goos === "windows" ? ".exe" : "";
  const binariesDir = join(root, "src-tauri", "binaries");
  const output = join(binariesDir, `apiw-sidecar-${targetTriple}${extension}`);

  mkdirSync(binariesDir, { recursive: true });

  execFileSync(
    "go",
    ["build", "-o", output, "./cmd/apiw-sidecar"],
    {
      cwd: root,
      stdio: "inherit",
      env: {
        ...process.env,
        GOOS: goos,
        GOARCH: goarch,
        CGO_ENABLED: "0",
      },
    },
  );
}

main();
