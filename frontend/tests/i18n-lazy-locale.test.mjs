import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/app/lib/i18n.tsx', import.meta.url), 'utf8');

test('keeps only the default locale bundle synchronous in the initial bundle', () => {
  assert.match(source, /import zhCN from "\.\.\/locales\/zh-CN\/translation\.json";/);
  assert.doesNotMatch(source, /import\s+\w+\s+from\s+"\.\.\/locales\/(?:en-US|ja-JP)\/translation\.json";/);
  // 默认语言仍是 i18next 的回退语言：语言包缺失时回退到默认语言内容，而不是裸 key。
  assert.match(source, /fallbackLng: DEFAULT_LOCALE/);
});

test('loads non-default locale bundles on demand via exported ensureLocaleBundle', () => {
  assert.match(source, /export function ensureLocaleBundle\(locale: AppLocale\): Promise<boolean>/);
  assert.match(source, /import\("\.\.\/locales\/en-US\/translation\.json"\)/);
  assert.match(source, /import\("\.\.\/locales\/ja-JP\/translation\.json"\)/);
  assert.match(source, /addResourceBundle\(locale, TRANSLATION_NS, bundle\.default/);
});

test('starts non-default bundle prefetch at module load without blocking first paint', () => {
  assert.match(source, /if \(typeof window !== "undefined"\) \{\s*\n\s*startNonDefaultBundlePrefetch\(\);/);
  assert.match(source, /requestIdleCallback/);
  assert.match(source, /window\.setTimeout\(idlePrefetch/);
});

test('applies changeLanguage only after the target locale bundle is ready', () => {
  const provider = source.slice(source.indexOf('export function AppI18nProvider'));
  const ensureIndex = provider.indexOf('ensureLocaleBundle(locale)');
  const changeIndex = provider.indexOf('i18n.changeLanguage(locale)');
  assert.ok(ensureIndex > -1, 'provider must await ensureLocaleBundle before switching');
  assert.ok(changeIndex > ensureIndex, 'changeLanguage must run after ensureLocaleBundle resolves');
});
