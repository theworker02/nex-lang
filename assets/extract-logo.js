/**
 * Extract mark-only Nexus ribbon N from source render:
 * crop wordmark out, chroma-key purple bg → transparent, trim, square pad.
 * Also rasterizes assets/logo.svg.
 */
const fs = require("fs");
const path = require("path");
const sharp = require(path.join(
  __dirname,
  "..",
  "vscode-nexus",
  "node_modules",
  "sharp",
));

const SRC = process.env.NEX_LOGO_SRC || path.join(__dirname, "source-logo.png");
const ROOT = path.resolve(__dirname, "..");
const REG =
  "C:/Users/matth/OneDrive/Desktop/personal projects/nex-registry/web/static/img";

function idx(x, y, w) {
  return (y * w + x) * 4;
}

function dist3(r, g, b, sr, sg, sb) {
  return Math.abs(r - sr) + Math.abs(g - sg) + Math.abs(b - sb);
}

function keepPixel(r, g, b, seeds) {
  // Always drop near-white (wordmark remnants)
  if (r > 220 && g > 220 && b > 220) return 0;

  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const sat = max - min;
  const bri = (r + g + b) / 3;

  // Strong cyan / magenta terminals — always keep
  const isCyan = g > 140 && b > 175 && r < 170;
  const isMagenta = r > 150 && b > 130 && g < 135;
  if (isCyan || isMagenta) return 255;

  // Distance to nearest background seed
  let best = Infinity;
  for (const s of seeds) {
    const d = dist3(r, g, b, s.r, s.g, s.b);
    if (d < best) best = d;
  }

  // Soft key
  if (best < 55) return 0;
  if (best < 100 && sat < 90 && bri < 150) return Math.round(((best - 55) / 45) * 180);

  // Mid-ribbon purple that still differs from bg
  if (best > 85 || (sat > 90 && bri > 70 && max > 140)) return 255;
  if (best > 70) return 200;
  return 0;
}

async function extract() {
  const crop = await sharp(SRC)
    .extract({ left: 190, top: 40, width: 644, height: 560 })
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });

  const { data, info } = crop;
  const w = info.width;
  const h = info.height;
  const pixels = Buffer.from(data);

  // Sample background seeds along the border (handles radial purple plate)
  const seeds = [];
  const sample = (x, y) => {
    const o = idx(x, y, w);
    seeds.push({ r: pixels[o], g: pixels[o + 1], b: pixels[o + 2] });
  };
  for (let i = 0; i < 12; i++) {
    const t = Math.floor((i / 11) * (w - 1));
    sample(t, 2);
    sample(t, h - 3);
    sample(2, Math.floor((i / 11) * (h - 1)));
    sample(w - 3, Math.floor((i / 11) * (h - 1)));
  }
  // interior plate samples away from ribbon (corners of inner frame)
  sample(40, 40);
  sample(w - 40, 40);
  sample(40, h - 40);
  sample(w - 40, h - 40);
  sample(Math.floor(w / 2), 20);
  sample(20, Math.floor(h / 2));

  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const o = idx(x, y, w);
      pixels[o + 3] = keepPixel(pixels[o], pixels[o + 1], pixels[o + 2], seeds);
    }
  }

  let minX = w,
    minY = h,
    maxX = 0,
    maxY = 0;
  let kept = 0;
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      if (pixels[idx(x, y, w) + 3] > 24) {
        kept++;
        if (x < minX) minX = x;
        if (y < minY) minY = y;
        if (x > maxX) maxX = x;
        if (y > maxY) maxY = y;
      }
    }
  }

  const tw = maxX - minX + 1;
  const th = maxY - minY + 1;
  const pad = 40;
  const side = Math.max(tw, th) + pad * 2;
  const out = Buffer.alloc(side * side * 4, 0);
  const ox = Math.floor((side - tw) / 2);
  const oy = Math.floor((side - th) / 2);
  for (let y = 0; y < th; y++) {
    for (let x = 0; x < tw; x++) {
      const so = idx(minX + x, minY + y, w);
      const d = idx(ox + x, oy + y, side);
      out[d] = pixels[so];
      out[d + 1] = pixels[so + 1];
      out[d + 2] = pixels[so + 2];
      out[d + 3] = pixels[so + 3];
    }
  }

  const png = await sharp(out, {
    raw: { width: side, height: side, channels: 4 },
  })
    .png()
    .toBuffer();

  const targets = [
    path.join(ROOT, "assets/logo.png"),
    path.join(ROOT, "vscode-nexus/media/logo.png"),
    path.join(REG, "logo.png"),
    path.join(REG, "logo-full.png"),
  ];
  for (const t of targets) {
    fs.mkdirSync(path.dirname(t), { recursive: true });
    fs.writeFileSync(t, png);
    console.log("wrote", t);
  }

  await sharp(png).resize(512, 512).png().toFile(path.join(ROOT, "assets/logo-512.png"));
  await sharp(png)
    .resize(256, 256)
    .png()
    .toFile(path.join(ROOT, "vscode-nexus/media/logo-256.png"));
  await sharp(png).resize(180, 180).png().toFile(path.join(REG, "apple-touch-icon.png"));
  await sharp(png).resize(32, 32).png().toFile(path.join(REG, "favicon.png"));
  console.log("ok", { side, minX, minY, maxX, maxY, tw, th, kept });
}

async function rasterSvg() {
  const svgPath = path.join(ROOT, "assets/logo.svg");
  const svg = fs.readFileSync(svgPath);
  const buf = await sharp(svg, { density: 320 }).resize(512, 512).png().toBuffer();
  fs.writeFileSync(path.join(ROOT, "assets/logo-svg.png"), buf);
  fs.writeFileSync(path.join(ROOT, "vscode-nexus/media/logo-svg.png"), buf);
  // Prefer SVG-raster as alternate clean mark for README if extract is noisy
  console.log("rasterized SVG");
}

extract()
  .then(rasterSvg)
  .catch((err) => {
    console.error(err);
    process.exit(1);
  });
