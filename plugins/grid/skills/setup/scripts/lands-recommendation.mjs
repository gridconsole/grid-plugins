#!/usr/bin/env node
// Setup's one edit outside proposals.md: when no repository of the project has a remote, the still-open "how should finished
// work land" question recommends the local merge instead of a pull request. Usage: lands-recommendation.mjs <questions file> <repo folder>...
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { hasRemote } from '../../deliver/scripts/land-plan.mjs';

const PR = 'Recommendation: Open a pull request, so every change is reviewed before it is merged.';
export const LOCAL = "Recommendation: Merge, don't push, because the repository has no remote to open a pull request on.";

export function recommend(text, anyRemote) {
  if (anyRemote) return text;
  return text.split('\n\n').map((b) => (b.includes('How should finished work land') && b.includes('Answer: —') ? b.replace(PR, LOCAL) : b)).join('\n\n');
}

if (isMain()) {
  const [file, ...repos] = process.argv.slice(2);
  const text = fs.readFileSync(file, 'utf8');
  const next = recommend(text, repos.length === 0 || repos.some(hasRemote));
  if (next !== text) fs.writeFileSync(file, next);
  process.stdout.write(next === text ? 'unchanged\n' : 'recommends the local merge\n');
}

function isMain() {
  try { return fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url)); } catch { return false; }
}
