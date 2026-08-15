// Screenshot a page of the running site (`make run` elsewhere, or point --base).
//
//   node shot.mjs                              the home page, light, to tmp/shot.png
//   node shot.mjs --text "מה נשמע" --theme dark
//   node shot.mjs /about --out tmp/about.png --width 420
import { chromium } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

const args = process.argv.slice(2);
const flag = (name, fallback) => {
  const at = args.indexOf(`--${name}`);
  return at === -1 ? fallback : args[at + 1];
};

const base = flag("base", `http://127.0.0.1:${process.env.RG_E2E_PORT ?? "25256"}`);
const theme = flag("theme", "light");
const width = Number(flag("width", "900"));
const out = path.resolve(root, flag("out", "tmp/shot.png"));
const text = flag("text", null);

let target = args.find((a) => a.startsWith("/")) ?? "/";
if (text !== null) target = "/?text=" + encodeURIComponent(text);

const browser = await chromium.launch();
const page = await browser.newPage({
  viewport: { width, height: 900 },
  colorScheme: theme,
  deviceScaleFactor: 2,
  locale: "he-IL",
});
try {
  const response = await page.goto(base + target, { waitUntil: "networkidle" });
  if (!response?.ok()) throw new Error(`${base + target} returned ${response?.status()}`);
  // Without this the first paint can land before the Hebrew font arrives, and
  // the screenshot shows the fallback font with the niqqud in the wrong place.
  await page.evaluate(() => document.fonts.ready);
  await page.screenshot({ path: out, fullPage: true });
  console.log(`${base + target} -> ${path.relative(root, out)} (${theme}, ${width}px)`);
} finally {
  await browser.close();
}
