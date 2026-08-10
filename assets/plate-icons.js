const path = require("path");
const sharp = require(path.join(
  __dirname,
  "..",
  "vscode-nexus",
  "node_modules",
  "sharp",
));

const root = path.resolve(__dirname, "..");
const reg = path.join(
  root,
  "..",
  "nex-registry",
  "web",
  "static",
  "img",
);

async function plated(markBuf, size, radius, out) {
  const svg = Buffer.from(
    `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0%" stop-color="#3B0A7A"/><stop offset="100%" stop-color="#1A0538"/></linearGradient></defs><rect width="${size}" height="${size}" rx="${radius}" fill="url(#g)"/></svg>`,
  );
  const pad = Math.round(size * 0.14);
  const inner = size - pad * 2;
  const innerMark = await sharp(markBuf).resize(inner, inner).png().toBuffer();
  await sharp(svg)
    .composite([{ input: innerMark, left: pad, top: pad }])
    .png()
    .toFile(out);
  console.log("plated", out);
}

(async () => {
  const mark = await sharp(path.join(root, "assets/logo.png"))
    .resize(512, 512)
    .ensureAlpha()
    .png()
    .toBuffer();
  await plated(mark, 32, 7, path.join(reg, "favicon.png"));
  await plated(mark, 180, 40, path.join(reg, "apple-touch-icon.png"));
  await plated(mark, 128, 28, path.join(reg, "logo-icon.png"));
  await plated(mark, 256, 48, path.join(root, "vscode-nexus/media/icon.png"));
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
