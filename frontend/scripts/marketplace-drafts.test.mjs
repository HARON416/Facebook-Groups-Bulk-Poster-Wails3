import { test } from 'node:test';
import assert from 'node:assert/strict';
import { emptyItemListing, saveItemListingDraft, readItemListingDraft, deleteItemListingDraft, listMarketplaceListings, saveMarketplaceListing, deleteMarketplaceListing } from '../src/lib/marketplace-drafts.ts';

// Exercise draft records and binary round trips with an in-memory IDB store.
const records = new Map();
const database = {
  createObjectStore() {},
  transaction() {
    const transaction = {
      objectStore() {
        return {
          get(key) {
            const request = { result: records.get(JSON.stringify(key)) };
            queueMicrotask(() => request.onsuccess?.());
            return request;
          },
          getAll(range) {
            const values = [...records.entries()].filter(([key]) => !range || (JSON.parse(key)[0] === range.lower[0] && JSON.parse(key)[1] === range.lower[1])).map(([, value]) => value);
            const request = { result: structuredClone(values) };
            queueMicrotask(() => request.onsuccess?.());
            return request;
          },
          getAllKeys() {
            const request = { result: [...records.keys()].map(key => JSON.parse(key)) };
            queueMicrotask(() => request.onsuccess?.());
            return request;
          },
          put(value, key) { records.set(JSON.stringify(key), structuredClone(value)); },
          delete(key) { records.delete(JSON.stringify(key)); },
        };
      },
    };
    queueMicrotask(() => transaction.oncomplete?.());
    return transaction;
  },
};
globalThis.IDBKeyRange = { bound: (lower, upper) => ({ lower, upper }) };
globalThis.indexedDB = {
  open() {
    const request = { result: database };
    queueMicrotask(() => { request.onupgradeneeded?.(); request.onsuccess?.(); });
    return request;
  },
};

test('home fields and media remain separate from item and vehicle drafts', async () => {
  const photo = new File(['photo'], 'home.jpg', { type: 'image/jpeg', lastModified: 123 });
  const video = new File(['video'], 'tour.webm', { type: 'video/webm', lastModified: 456 });
  await saveItemListingDraft('one', { ...emptyItemListing(), title: 'Item' }, [], 'items');
  await saveItemListingDraft('one', { ...emptyItemListing(), make: 'Honda' }, [], 'vehicles');
  await saveItemListingDraft('one', { ...emptyItemListing(), homeOffer: 'rent', propertyType: 'house', bedrooms: '2', bathrooms: '1.5', address: 'Example address', heating: 'central', privateRoom: true, dateAvailable: '2026-11-01', catFriendly: true, dogFriendly: true, occupants: '3', bathroomType: 'private', roomFurnished: true }, Array(51).fill(photo), 'homes', video);
  const home = await readItemListingDraft('one', 'homes');
  assert.equal(home.fields.bedrooms, '2');
  assert.equal(home.fields.bathrooms, '1.5');
  assert.equal(home.fields.address, 'Example address');
  assert.equal(home.fields.heating, 'central');
  assert.equal(home.fields.privateRoom, true);
  assert.equal(home.fields.occupants, '3');
  assert.equal(home.fields.bathroomType, 'private');
  assert.equal(home.fields.roomFurnished, true);
  assert.equal(home.fields.dateAvailable, '2026-11-01');
  assert.equal(home.fields.catFriendly, true);
  assert.equal(home.fields.dogFriendly, true);
  assert.equal(home.photos.length, 51);
  assert.equal(await home.photos[0].text(), 'photo');
  assert.equal(home.photos[0].lastModified, 123);
  assert.equal(await home.video.text(), 'video');
  assert.equal(home.video.lastModified, 456);
  assert.equal((await readItemListingDraft('one', 'items')).fields.title, 'Item');
  assert.equal((await readItemListingDraft('one', 'vehicles')).fields.make, 'Honda');
  assert.equal(await readItemListingDraft('two', 'homes'), null);
});

test('older home drafts receive defaults for new rental fields', async () => {
  records.set(JSON.stringify(['homes', 'legacy']), { fields: { homeOffer: 'rent', propertyType: 'house' }, photos: [] });
  const draft = await readItemListingDraft('legacy', 'homes');
  assert.equal(draft.fields.privateRoom, false);
  assert.equal(draft.fields.occupants, '');
  assert.equal(draft.fields.bathroomType, '');
  assert.equal(draft.fields.roomFurnished, false);
  assert.equal(draft.fields.dateAvailable, '');
  assert.equal(draft.fields.catFriendly, false);
  assert.equal(draft.fields.dogFriendly, false);
});

test('item video drafts restore and removal clears the stored binary', async () => {
  const video = new File(['item-video'], 'product.webm', { type: 'video/webm', lastModified: 789 });
  await saveItemListingDraft('item-video', emptyItemListing(), [], 'items', video);
  const draft = await readItemListingDraft('item-video', 'items');
  assert.equal(draft.video.name, 'product.webm');
  assert.equal(draft.video.type, 'video/webm');
  assert.equal(draft.video.lastModified, 789);
  assert.equal(await draft.video.text(), 'item-video');
  await saveItemListingDraft('item-video', draft.fields, [], 'items', null);
  assert.equal((await readItemListingDraft('item-video', 'items')).video, null);
});

test('account deletion removes all listing kinds and preserves other accounts', async () => {
  for (const kind of ['items', 'vehicles', 'homes']) {
    await saveItemListingDraft('delete', emptyItemListing(), [], kind);
    await saveItemListingDraft('keep', emptyItemListing(), [], kind);
  }
  await deleteItemListingDraft('delete');
  for (const kind of ['items', 'vehicles', 'homes']) {
    assert.equal(await readItemListingDraft('delete', kind), null);
    assert.ok(await readItemListingDraft('keep', kind));
  }
});


test('restoring over-limit selections preserves every photo and its order for each listing kind', async () => {
  for (const [kind, count] of [['items', 12], ['vehicles', 22], ['homes', 52]]) {
    const photos = Array.from({ length: count }, (_, index) => new File([String(index)], `photo-${index}.jpg`, { type: 'image/jpeg' }));
    await saveItemListingDraft('overflow', emptyItemListing(), photos, kind);
    const restored = await readItemListingDraft('overflow', kind);
    assert.equal(restored.photos.length, count);
    assert.deepEqual(restored.photos.map(photo => photo.name), photos.map(photo => photo.name));
    assert.equal(await restored.photos.at(-1).text(), String(count - 1));
  }
});


test('multiple listings survive updates, copies, removal, and account deletion', async () => {
  const draft = { fields: { ...emptyItemListing(), title: 'First' }, photos: [new File(['image'], 'photo.jpg', { type: 'image/jpeg', lastModified: 123 })], video: new File(['clip'], 'tour.mp4', { type: 'video/mp4' }), activePhoto: 0 };
  const first = await saveMarketplaceListing('collection', draft, 'items');
  const second = await saveMarketplaceListing('collection', { ...draft, fields: { ...draft.fields, title: 'Second' } }, 'items');
  await saveMarketplaceListing('other-account', draft, 'homes');
  assert.notEqual(first, second);
  let listings = await listMarketplaceListings('collection');
  assert.equal(listings.length, 2);
  const restored = listings.find(listing => listing.id === first);
  assert.equal(await restored.photos[0].text(), 'image');
  assert.equal(restored.photos[0].lastModified, 123);
  assert.equal(await restored.video.text(), 'clip');
  await saveMarketplaceListing('collection', { ...restored, fields: { ...restored.fields, title: 'Edited' }, video: null }, 'items', first);
  listings = await listMarketplaceListings('collection');
  assert.equal(listings.length, 2);
  assert.equal(listings.find(listing => listing.id === first).fields.title, 'Edited');
  assert.equal(listings.find(listing => listing.id === first).video, null);
  const copy = await saveMarketplaceListing('collection', restored, 'items');
  assert.notEqual(copy, first);
  await deleteMarketplaceListing('collection', second);
  assert.equal((await listMarketplaceListings('collection')).length, 2);
  await deleteItemListingDraft('collection');
  assert.deepEqual(await listMarketplaceListings('collection'), []);
  assert.equal((await listMarketplaceListings('other-account')).length, 1);
});

test('legacy drafts migrate once with all media and cannot overwrite an edited listing', async () => {
  for (const kind of ['items', 'vehicles', 'homes']) {
    await saveItemListingDraft('migrate', { ...emptyItemListing(), title: kind }, [new File([kind], `${kind}.jpg`)], kind);
  }
  const listings = await listMarketplaceListings('migrate');
  assert.equal(listings.length, 3);
  for (const listing of listings) {
    assert.equal(await listing.photos[0].text(), listing.kind);
    assert.equal(await readItemListingDraft('migrate', listing.kind), null);
  }
  const item = listings.find(listing => listing.kind === 'items');
  await saveMarketplaceListing('migrate', { ...item, fields: { ...item.fields, title: 'Edited migration' } }, 'items', item.id);
  assert.equal((await listMarketplaceListings('migrate')).length, 3);
  assert.equal((await listMarketplaceListings('migrate')).find(listing => listing.id === item.id).fields.title, 'Edited migration');
});
