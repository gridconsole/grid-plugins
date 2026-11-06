// PermissionRequest: ask the Grid server to answer the permission prompt, and wait for it up to
// GRID_PERMISSION_TIMEOUT_MS (default 85 s; the hook's own timeout in hooks.json is longer). The
// server is GRID_URL, or the Grid app's local bridge when that is unset (lib.mjs bridgeTarget).
// Prints a decision only when Grid answers allow or deny. With no answer, an error, a timeout or
// no bridge it prints nothing (the same as {}), so the prompt falls through to wherever it would
// have been shown (the terminal).
import { main, readInput, post, bridgeTarget } from './lib.mjs';

main(async () => {
  const input = await readInput();
  const target = input ? bridgeTarget() : null;
  if (!target) return;
  const wait = Number(process.env.GRID_PERMISSION_TIMEOUT_MS);
  const res = await post(target.url, 'hooks/permission', input, wait > 0 ? wait : 85000, target.token);
  const answer = res?.ok ? res.json : null;
  if (answer?.behavior !== 'allow' && answer?.behavior !== 'deny') return;
  const decision = { behavior: answer.behavior };
  const object = (v) => v && typeof v === 'object' && !Array.isArray(v);
  if (answer.behavior === 'allow' && object(answer.updatedInput)) decision.updatedInput = answer.updatedInput;
  if (answer.behavior === 'deny') {
    if (typeof answer.message === 'string' && answer.message) decision.message = answer.message;
    if (answer.interrupt === true) decision.interrupt = true;
  }
  return { stdout: JSON.stringify({ hookSpecificOutput: { hookEventName: 'PermissionRequest', decision } }) };
});
