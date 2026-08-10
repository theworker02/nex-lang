const fs = require("fs");
const os = require("os");
const path = require("path");
const tar = require("tar");
const { RegistryClient } = require("../out/registry/client");

(async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "nexus-ext-"));
  fs.writeFileSync(
    path.join(dir, "nexus.toml"),
    [
      'name = "demo"',
      'version = "0.1.0"',
      'author = "tester"',
      'description = "smoke"',
      "",
      "[dependencies]",
      "",
    ].join("\n"),
  );
  fs.writeFileSync(path.join(dir, "main.nex"), "let x = 1;\n");
  fs.mkdirSync(path.join(dir, ".modules", "skip"), { recursive: true });
  fs.writeFileSync(path.join(dir, ".modules", "skip", "x.txt"), "nope");

  const client = new RegistryClient({ registryUrl: "http://localhost:8080" });
  const manifest = await client.readManifest(dir);
  if (manifest.name !== "demo" || manifest.version !== "0.1.0") {
    throw new Error(`unexpected manifest: ${JSON.stringify(manifest)}`);
  }

  const archive = path.join(dir, "demo-0.1.0.tar.gz");
  await client.createArchive(dir, archive);
  if (!fs.existsSync(archive)) {
    throw new Error("archive was not created");
  }

  const checksum = await client.sha256File(archive);
  if (!/^[a-f0-9]{64}$/i.test(checksum)) {
    throw new Error(`bad checksum: ${checksum}`);
  }

  const extractDir = path.join(dir, "extracted");
  fs.mkdirSync(extractDir);
  await tar.x({ file: archive, cwd: extractDir });
  const entries = fs.readdirSync(extractDir);
  if (!entries.includes("main.nex") && !entries.includes("nexus.toml")) {
    // tar may nest under "." — list recursively
    const all = [];
    const walk = (p) => {
      for (const name of fs.readdirSync(p)) {
        const full = path.join(p, name);
        all.push(path.relative(extractDir, full));
        if (fs.statSync(full).isDirectory()) {
          walk(full);
        }
      }
    };
    walk(extractDir);
    if (!all.some((f) => f.endsWith("main.nex"))) {
      throw new Error(`main.nex missing from archive; got ${all.join(", ")}`);
    }
  }
  if (fs.existsSync(path.join(extractDir, ".modules"))) {
    throw new Error(".modules should be excluded from archive");
  }

  console.log("smoke-registry: ok", { checksum: checksum.slice(0, 12) });
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
