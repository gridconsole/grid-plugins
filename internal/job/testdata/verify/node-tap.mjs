import { test } from 'node:test';

test('a', () => {});
test('b', () => { throw new Error('example failure'); });
test('nested', async (t) => {
  await t.test('child', () => {});
});
