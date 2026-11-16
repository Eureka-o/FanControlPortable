import { existsSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const distDir = resolve(scriptDir, '..', 'dist');

// Unreferenced static files from frontend/public/ that should stay out of the embedded bundle.
const unreferencedDistFiles = [
  'bs1.png',
  'bs2.png',
  'bs2pro.png',
  'brand/mark.png',
  'brand/wordmark-dark.png',
  'brand/wordmark-light.png',
  'brand/fancontrol-flow-loop-v2.png',
  'brand/fancontrol-flow-loop-v2-dark.png',
  'file.svg',
  'globe.svg',
  'next.svg',
  'vercel.svg',
  'window.svg',
];

let removedCount = 0;
for (const relPath of unreferencedDistFiles) {
  const target = resolve(distDir, relPath);
  if (existsSync(target)) {
    rmSync(target, { force: true });
    removedCount += 1;
  }
}

console.log(`prune-dist: removed ${removedCount} unreferenced asset(s) from ${distDir}`);
