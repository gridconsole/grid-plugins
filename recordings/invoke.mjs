// A JSON argument adapter; all behavior lives in the unmodified reference module.
import fs from 'node:fs';
import { pathToFileURL } from 'node:url';
const [source, name] = process.argv.slice(2);
const lib = await import(pathToFileURL(source));
const input = JSON.parse(fs.readFileSync(0, 'utf8'));
const result = name === 'terminalTask' ? lib[name](input) : lib[name](...input.args);
process.stdout.write(`${JSON.stringify(result)}\n`);
