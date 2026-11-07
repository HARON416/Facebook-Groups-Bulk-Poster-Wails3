import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { test } from 'node:test';

registerHooks({
  resolve(specifier, context, next) {
    if (specifier === '@wailsio/runtime') return {
      url: 'data:text/javascript,export const Call = { ByID: (...args) => globalThis.marketplaceCall(...args) };',
      shortCircuit: true,
    };
    return next(specifier, context);
  },
});
const { publishMarketplaceListingMedia, testMarketplaceListingMedia } = await import('../src/lib/media-upload.ts');
const { emptyItemListing } = await import('../src/lib/marketplace-drafts.ts');
globalThis.FileReader = class {
  readAsDataURL() {
    queueMicrotask(() => { this.result = 'data:image/png;base64,aW1hZ2U='; this.onload?.(); this.onloadend?.(); });
  }
  abort() { this.onabort?.(); this.onloadend?.(); }
};
const photo = new File(['image'], 'photo.png', { type: 'image/png' });
const fields = { ...emptyItemListing(), vehicleType: 'car-truck', homeOffer: 'rent', privateRoom: true, catFriendly: true, dogFriendly: true };
function mockCall(handler) {
  globalThis.marketplaceCall = (...args) => {
    const promise = Promise.resolve().then(() => handler(...args));
    promise.cancel = async () => {};
    return promise;
  };
}

test('Publish uses its own binding and preserves each listing payload; Test stays separate', async () => {
  for (const [kind, testID, key] of [['items', 4270654618, 'fields'], ['vehicles', 4270654619, 'vehicleFields'], ['homes', 4270654620, 'homeFields']]) {
    const calls = [];
    mockCall((...args) => { calls.push(args); return { state: 'submitted' }; });
    const args = [kind, 'account-1', [photo], null, fields, new AbortController().signal];
    assert.equal((await publishMarketplaceListingMedia(...args)).state, 'submitted');
    await testMarketplaceListingMedia(...args);
    assert.deepEqual(calls.map(call => call[0]), [4270654621, testID]);
    assert.equal(calls[0][1], 'account-1');
    assert.deepEqual(calls[0][2], calls[1][2]);
    assert.equal(calls[0][2].photos[0].data, 'aW1hZ2U=');
    assert.ok(calls[0][2][key]);
    assert.equal(['fields', 'vehicleFields', 'homeFields'].filter(name => name in calls[0][2]).length, 1);
    if (kind === 'homes') {
      assert.equal(calls[0][2].homeFields.privateRoom, true);
      assert.equal(calls[0][2].homeFields.catFriendly, true);
      assert.equal(calls[0][2].homeFields.dogFriendly, true);
    }
  }
});

test('only confirmed pre-click errors are retryable; transport errors remain uncertain without retries', async () => {
  const args = ['items', 'account-1', [photo], null, fields, new AbortController().signal];
  mockCall(() => ({ state: 'failed', message: 'Validation failed before Publish' }));
  await assert.rejects(publishMarketplaceListingMedia(...args), /Validation failed before Publish/);
  let calls = 0;
  mockCall(() => { calls++; throw new Error('Connection lost'); });
  assert.deepEqual(await publishMarketplaceListingMedia(...args), { state: 'unknown' });
  assert.equal(calls, 1);
  await assert.rejects(testMarketplaceListingMedia(...args), /Connection lost/);
  mockCall(() => undefined);
  assert.deepEqual(await publishMarketplaceListingMedia(...args), { state: 'unknown' });
});

test('cancelled preparation never calls the Publish binding', async () => {
  mockCall(() => assert.fail('Publish binding should not run'));
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(publishMarketplaceListingMedia('homes', 'account-1', [photo], null, fields, controller.signal), { name: 'AbortError' });
});
