import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../..", import.meta.url);
const corpusText = readFileSync(new URL("tools/ui-font-corpus.txt", root), "utf8");
const zhLocale = JSON.parse(
  readFileSync(new URL("frontend/src/app/locales/zh-CN/translation.json", root), "utf8"),
);
const enLocale = JSON.parse(
  readFileSync(new URL("frontend/src/app/locales/en-US/translation.json", root), "utf8"),
);

function parseCorpusFile(text) {
  const chars = new Set();
  for (const line of text.split(/\r?\n/)) {
    if (!line || line.startsWith("#")) continue;
    for (const ch of line) {
      chars.add(ch.codePointAt(0));
    }
  }
  return chars;
}

function collectJsonStringChars(value, target) {
  if (typeof value === "string") {
    for (const ch of value) {
      const cp = ch.codePointAt(0);
      if (cp >= 0x20 && cp !== 0x7f && cp !== 0xfeff) {
        target.add(cp);
      }
    }
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) collectJsonStringChars(item, target);
    return;
  }
  if (value && typeof value === "object") {
    for (const item of Object.values(value)) collectJsonStringChars(item, target);
  }
}

function parseUnicodeRangeExpr(expr) {
  const cps = new Set();
  for (const part of expr.split(",")) {
    const token = part.trim().toUpperCase();
    if (!token.startsWith("U+")) continue;
    const body = token.slice(2);
    if (body.includes("-")) {
      const [startHex, endHex] = body.split("-", 2);
      const start = parseInt(startHex, 16);
      const end = parseInt(endHex, 16);
      for (let cp = start; cp <= end; cp++) cps.add(cp);
    } else if (body.includes("?")) {
      const start = parseInt(body.replace(/\?/g, "0"), 16);
      const end = parseInt(body.replace(/\?/g, "F"), 16);
      for (let cp = start; cp <= end; cp++) cps.add(cp);
    } else {
      cps.add(parseInt(body, 16));
    }
  }
  return cps;
}

function extractUnicodeRangeForAsset(cssText, assetName) {
  const escaped = assetName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const re = new RegExp(
    `@font-face\\s*\\{[^}]*?url\\(["'][^"']*${escaped}["']\\)[^}]*?unicode-range:\\s*([^;]+);`,
    "s",
  );
  const match = cssText.match(re);
  assert.ok(match, `expected @font-face unicode-range for ${assetName}`);
  return parseUnicodeRangeExpr(match[1]);
}

function hasJapaneseKana(cps) {
  for (const cp of cps) {
    if (cp >= 0x3040 && cp <= 0x30ff) return true;
  }
  return false;
}

test("curated UI font corpus covers ASCII letters/digits, zh-CN settings, and theme metadata", () => {
  const corpus = parseCorpusFile(corpusText);
  assert.ok(corpus.size > 900, `expected curated corpus to be non-trivial, got ${corpus.size}`);

  for (let cp = 0x20; cp <= 0x7e; cp++) {
    assert.ok(
      corpus.has(cp),
      `corpus missing ASCII U+${cp.toString(16).toUpperCase()} (${String.fromCodePoint(cp)})`,
    );
  }
  assert.equal(hasJapaneseKana(corpus), false, "corpus should not retain unused Japanese Kana");

  const requiredChars = new Set();
  collectJsonStringChars(zhLocale, requiredChars);
  collectJsonStringChars(enLocale, requiredChars);

  const themesDir = new URL("themes/", root);
  for (const entry of readdirSync(themesDir, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const manifestUrl = new URL(`themes/${entry.name}/theme.json`, root);
    if (!existsSync(manifestUrl)) continue;
    const manifest = JSON.parse(readFileSync(manifestUrl, "utf8"));
    collectJsonStringChars(manifest, requiredChars);
  }

  const missing = [...requiredChars]
    .filter((cp) => !corpus.has(cp))
    .map((cp) => String.fromCodePoint(cp))
    .join("");
  assert.equal(
    missing,
    "",
    `tools/ui-font-corpus.txt is missing live UI characters: ${missing} (run python tools/extract_ui_corpus.py --write-corpus --rebuild-fonts --verify)`,
  );
});

test("themed custom @font-face unicode-ranges stay aligned with curated corpus without Kana bloat", () => {
  const corpus = parseCorpusFile(corpusText);

  const mikuCss = readFileSync(new URL("themes/miku-01/theme.css", root), "utf8");
  const mikuUiRange = extractUnicodeRangeForAsset(mikuCss, "miku-xiaolai-ui-subset.woff2");
  const mikuRuntimeRange = extractUnicodeRangeForAsset(mikuCss, "miku-xiaolai-runtime-patch.woff2");
  assert.equal(hasJapaneseKana(mikuUiRange), false);
  assert.equal(hasJapaneseKana(mikuRuntimeRange), false);

  const mikuUnion = new Set([...mikuUiRange, ...mikuRuntimeRange]);
  const missingInMiku = [...corpus]
    .filter((cp) => !mikuUnion.has(cp))
    .map((cp) => String.fromCodePoint(cp))
    .join("");
  assert.equal(missingInMiku, "", `miku-01 CSS unicode-range missing UI chars: ${missingInMiku}`);

  const shinchanCss = readFileSync(new URL("themes/shinchan/theme.css", root), "utf8");
  const shinchanPatchRange = extractUnicodeRangeForAsset(shinchanCss, "advanced-glyph-patch-ui.woff2");
  assert.ok(shinchanPatchRange.size > 100);
  assert.equal(hasJapaneseKana(shinchanPatchRange), false);

  const xiaobaCss = readFileSync(new URL("themes/xiaoba-deluxe/theme.css", root), "utf8");
  const xiaobaPatchRange = extractUnicodeRangeForAsset(xiaobaCss, "advanced-glyph-patch-ui.woff2");
  assert.equal(hasJapaneseKana(xiaobaPatchRange), false);
  assert.ok(xiaobaPatchRange.size <= 10, "xiaoba-deluxe should use Maoken natively instead of Noto patch overrides");
});
