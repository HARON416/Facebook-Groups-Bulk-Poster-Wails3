import type { MarketplaceLocationHit } from '../../bindings/postika';

export type MarketplaceListingKind = 'items' | 'vehicles' | 'homes';
export interface ItemListingFields {
  homeOffer: string; propertyType: string; bedrooms: string; bathrooms: string; address: string;
  squareFeet: string; laundry: string; parking: string; airConditioning: string; heating: string;
  privateRoom: boolean; dateAvailable: string; catFriendly: boolean; dogFriendly: boolean;
  occupants: string; bathroomType: string; roomFurnished: boolean;
  vehicleType: string; year: string; make: string; model: string;
  mileage: string; bodyStyle: string; exteriorColor: string; interiorColor: string;
  vehicleCondition: string; fuelType: string; transmission: string; cleanTitle: boolean;
  title: string; price: string; category: string; condition: string;
  description: string; availability: string; tags: string; sku: string; location: string;
  locationData: MarketplaceLocationHit | null;
  publicMeetup: boolean; doorPickup: boolean; doorDropoff: boolean;
  promoteAfterPublish: boolean; hideFromFriends: boolean;
}
export const emptyItemListing = (): ItemListingFields => ({
  homeOffer: '', propertyType: '', bedrooms: '', bathrooms: '', address: '',
  squareFeet: '', laundry: '', parking: '', airConditioning: '', heating: '',
  privateRoom: false, dateAvailable: '', catFriendly: false, dogFriendly: false,
  occupants: '', bathroomType: '', roomFurnished: false,
  vehicleType: '', year: '', make: '', model: '',
  mileage: '', bodyStyle: '', exteriorColor: '', interiorColor: '', vehicleCondition: '', fuelType: '', transmission: 'automatic', cleanTitle: false,
  title: '', price: '', category: '', condition: '', description: '',
  availability: 'single', tags: '', sku: '', location: '', locationData: null,
  publicMeetup: false, doorPickup: false, doorDropoff: false,
  promoteAfterPublish: false, hideFromFriends: false,
});
export interface ListingEditorSnapshot { listingID?: string; fields: ItemListingFields; photos: File[]; activePhoto: number; video?: File | null; showingVideo?: boolean; locationQuery?: string; locationTouched?: boolean }
const draftKey = (profileID: string, kind: MarketplaceListingKind): IDBValidKey => kind === 'items' ? profileID : [kind, profileID];

interface StoredPhoto { name: string; type: string; lastModified: number; data: ArrayBuffer }
interface StoredDraft { fields: ItemListingFields; photos: StoredPhoto[]; video?: StoredPhoto }
interface StoredListing extends StoredDraft { id: string; profileID: string; kind: MarketplaceListingKind; updatedAt: number }
export interface MarketplaceListing extends ListingEditorSnapshot { id: string; kind: MarketplaceListingKind; updatedAt: number }

function changed() {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new Event('postika:listings-changed'));
  const channel = new BroadcastChannel('postika-marketplace');
  channel.postMessage('changed');
  channel.close();
}

const restore = (record: StoredDraft) => ({
  fields: { ...emptyItemListing(), ...record.fields },
  photos: record.photos.map(photo => new File([photo.data], photo.name, { type: photo.type, lastModified: photo.lastModified })),
  video: record.video ? new File([record.video.data], record.video.name, { type: record.video.type, lastModified: record.video.lastModified }) : null,
});

export async function listMarketplaceListings(profileID: string): Promise<MarketplaceListing[]> {
  const db = await openDatabase();
  // Move existing single drafts into the collection once, keeping every attachment.
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction('items', 'readwrite');
    const store = transaction.objectStore('items');
    for (const kind of ['items', 'vehicles', 'homes'] as const) {
      const key = draftKey(profileID, kind);
      const request = store.get(key);
      request.onsuccess = () => {
        if (!request.result) return;
        const id = `legacy-${kind}`;
        store.put({ ...request.result, id, profileID, kind, updatedAt: Date.now() }, ['listing', profileID, id]);
        store.delete(key);
      };
    }
    transaction.oncomplete = () => resolve();
    transaction.onerror = () => reject(transaction.error);
    transaction.onabort = () => reject(transaction.error);
  });
  const records = await new Promise<StoredListing[]>((resolve, reject) => {
    const request = db.transaction('items').objectStore('items').getAll(IDBKeyRange.bound(['listing', profileID], ['listing', profileID, []]));
    request.onsuccess = () => resolve(request.result.filter((record: StoredListing) => record.profileID === profileID && record.id));
    request.onerror = () => reject(request.error);
  });
  return records.sort((a, b) => b.updatedAt - a.updatedAt).map(record => ({ ...restore(record), id: record.id, listingID: record.id, kind: record.kind, updatedAt: record.updatedAt, activePhoto: 0 }));
}

export async function saveMarketplaceListing(profileID: string, draft: ListingEditorSnapshot, kind: MarketplaceListingKind, id: string = crypto.randomUUID()) {
  const encode = async (file: File): Promise<StoredPhoto> => ({ name: file.name, type: file.type, lastModified: file.lastModified, data: await file.arrayBuffer() });
  const record: StoredListing = { id, profileID, kind, updatedAt: Date.now(), fields: { ...draft.fields }, photos: await Promise.all(draft.photos.map(encode)), ...(draft.video ? { video: await encode(draft.video) } : {}) };
  const db = await openDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction('items', 'readwrite');
    transaction.objectStore('items').put(record, ['listing', profileID, id]);
    if (id === `legacy-${kind}`) transaction.objectStore('items').delete(draftKey(profileID, kind));
    transaction.oncomplete = () => resolve();
    transaction.onerror = () => reject(transaction.error);
    transaction.onabort = () => reject(transaction.error);
  });
  changed();
  return id;
}

export async function deleteMarketplaceListing(profileID: string, id: string) {
  const db = await openDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction('items', 'readwrite');
    transaction.objectStore('items').delete(['listing', profileID, id]);
    transaction.oncomplete = () => resolve();
    transaction.onerror = () => reject(transaction.error);
    transaction.onabort = () => reject(transaction.error);
  });
  changed();
}

let database: Promise<IDBDatabase> | undefined;
function openDatabase() {
  return database ??= new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open('postika-marketplace-drafts', 1);
    request.onupgradeneeded = () => request.result.createObjectStore('items');
    request.onsuccess = () => {
      request.result.onversionchange = () => { request.result.close(); database = undefined; };
      resolve(request.result);
    };
    request.onerror = () => { database = undefined; reject(request.error); };
    request.onblocked = () => { database = undefined; reject(new Error('Marketplace draft storage is busy')); };
  });
}

export async function readItemListingDraft(profileID: string, kind: MarketplaceListingKind = 'items') {
  const db = await openDatabase();
  const record = await new Promise<StoredDraft | undefined>((resolve, reject) => {
    const request = db.transaction('items').objectStore('items').get(draftKey(profileID, kind));
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  if (!record) return null;
  return {
    fields: { ...emptyItemListing(), ...record.fields },
    video: record.video ? new File([record.video.data], record.video.name, { type: record.video.type, lastModified: record.video.lastModified }) : null,
    photos: record.photos.map(photo => new File([photo.data], photo.name, { type: photo.type, lastModified: photo.lastModified })),
  };
}

export async function saveItemListingDraft(profileID: string, fields: ItemListingFields, photos: File[], kind: MarketplaceListingKind = 'items', video: File | null = null) {
  const record: StoredDraft = {
    fields: { ...fields },
    ...(video ? { video: { name: video.name, type: video.type, lastModified: video.lastModified, data: await video.arrayBuffer() } } : {}),
    photos: await Promise.all(photos.map(async file => ({ name: file.name, type: file.type, lastModified: file.lastModified, data: await file.arrayBuffer() }))),
  };
  const db = await openDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction('items', 'readwrite');
    transaction.objectStore('items').put(record, draftKey(profileID, kind));
    transaction.oncomplete = () => resolve();
    transaction.onerror = () => reject(transaction.error);
    transaction.onabort = () => reject(transaction.error);
  });
}

export async function deleteItemListingDraft(profileID: string) {
  const db = await openDatabase();
  await new Promise<void>((resolve, reject) => {
    const transaction = db.transaction('items', 'readwrite');
    transaction.objectStore('items').delete(profileID);
    transaction.objectStore('items').delete(draftKey(profileID, 'vehicles'));
    transaction.objectStore('items').delete(draftKey(profileID, 'homes'));
    const keys = transaction.objectStore('items').getAllKeys();
    keys.onsuccess = () => {
      for (const key of keys.result) {
        if (Array.isArray(key) && key[0] === 'listing' && key[1] === profileID) transaction.objectStore('items').delete(key);
      }
    };
    transaction.oncomplete = () => resolve();
    transaction.onerror = () => reject(transaction.error);
    transaction.onabort = () => reject(transaction.error);
  });
}
