// Notification: tell the Grid server that Claude needs the person (a permission prompt, an idle
// session). Silent when there is no GRID_URL or live bridge (lib.mjs bridgeTarget) or nothing is listening.
import { main, readInput, post, bridgeTarget } from './lib.mjs';

main(async () => {
  const input = await readInput();
  const target = input ? bridgeTarget() : null;
  if (!target) return;
  await post(target.url, 'hooks/notification', input, 5000, target.token);
});
