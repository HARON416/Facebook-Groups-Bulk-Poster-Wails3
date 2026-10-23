import type { FacebookPagePost } from '../../bindings/postika/models';

// Store independent bytes: a structured-cloned File may retain a WebKit disk
// reference that is no longer readable after restarting or replacing the app.
export interface DraftImage { id: string; file: File; unavailable?: boolean; recoveryKey?: string }
export interface AccountDraft {
  body: string;
  bodyHTML: string;
  bodyState: string;
  images: DraftImage[];
  fetchedPosts?: FacebookPagePost[] | null;
  selectedGroupIds: string[];
  currentStep: number;
  minimumInterval: number;
  maximumInterval: number;
  rewrite: boolean;
  instructions: string;
  scheduleTime: string;
}

interface StoredImage {
  id: string;
  name: string;
  type: string;
  lastModified: number;
  key?: string | null;
  data?: ArrayBuffer | null;
}
type StoredDraft = Omit<AccountDraft, 'images'> & { schema: 2; images: StoredImage[] };
const imageBytes = new WeakMap<File, Promise<ArrayBuffer>>();
const writes = new Map<string, Promise<void>>();
const savedFiles = new Map<string, File>();
const savedKeys = new Map<string, Set<string>>();

function bytesFor(file: File): Promise<ArrayBuffer> {
  let bytes = imageBytes.get(file);
  if (!bytes) {
    bytes = file.arrayBuffer().then(data => {
      if (!data.byteLength || data.byteLength !== file.size) throw new Error('Unreadable image data');
      return data;
    });
    imageBytes.set(file, bytes);
    void bytes.catch(() => imageBytes.delete(file));
  }
  return bytes;
}

export async function snapshotImage(file: File): Promise<File> {
  const data = await bytesFor(file);
  const copy = new File([data], file.name, { type: file.type, lastModified: file.lastModified });
  imageBytes.set(copy, Promise.resolve(data));
  return copy;
}

let database: Promise<IDBDatabase> | undefined;
function openDatabase() {
  return database ??= new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open('postika-account-drafts', 2);
    request.onupgradeneeded = () => {
      if (!request.result.objectStoreNames.contains('drafts')) request.result.createObjectStore('drafts');
      if (!request.result.objectStoreNames.contains('images')) request.result.createObjectStore('images');
    };
    request.onsuccess = () => {
      request.result.onversionchange = () => { request.result.close(); database = undefined; savedFiles.clear(); savedKeys.clear(); };
      resolve(request.result);
    };
    request.onerror = () => { database = undefined; reject(request.error); };
    request.onblocked = () => { database = undefined; reject(new Error('Draft storage is busy in another window.')); };
  });
}

export async function readDraft(profileID: string): Promise<AccountDraft | undefined> {
  const db = await openDatabase();
  const stored = await new Promise<AccountDraft | StoredDraft | undefined>((resolve, reject) => {
    const request = db.transaction('drafts').objectStore('drafts').get(profileID);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  if (!stored) return undefined;
  const keys = new Set<string>();
  const images = await Promise.all(stored.images.map(async image => {
    const legacy = 'file' in image;
    const metadata = (legacy ? image.file : image) ?? { name: 'image', type: '', lastModified: 0 };
    try {
      if (legacy) {
        if (image.unavailable) throw new Error('Missing saved image');
        return { id: image.id, file: await snapshotImage(image.file) };
      }
      const data = image.key ? await new Promise<ArrayBuffer | undefined>((resolve, reject) => {
        const request = db.transaction('images').objectStore('images').get(image.key!);
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      }) : image.data;
      if (!(data instanceof ArrayBuffer) || !data.byteLength) throw new Error('Missing saved image');
      const file = new File([data], image.name, { type: image.type, lastModified: image.lastModified });
      imageBytes.set(file, Promise.resolve(data));
      if (image.key) { keys.add(image.key); savedFiles.set(image.key, file); }
      return { id: image.id, file };
    } catch {
      // Retain the missing attachment's identity and position. Never silently
      // post fewer photos or lose the rest of the draft because one file failed.
      const recoveryKey = legacy ? image.recoveryKey : image.key ?? undefined;
      if (recoveryKey) keys.add(recoveryKey);
      return { id: image.id, file: new File([], metadata.name, { type: metadata.type, lastModified: metadata.lastModified }), unavailable: true, recoveryKey };
    }
  }));
  savedKeys.set(profileID, keys);
  return { ...stored, images };
}

async function persistDraft(profileID: string, draft: AccountDraft): Promise<void> {
  const images = draft.images.map(({ id, file, unavailable, recoveryKey }): StoredImage => ({
    id, name: file.name, type: file.type, lastModified: file.lastModified,
    key: unavailable ? recoveryKey ?? null : `${profileID}:${id}`,
  }));
  const changed = await Promise.all(draft.images.flatMap((image, index) => {
    const key = images[index].key;
    return key && !image.unavailable && savedFiles.get(key) !== image.file
      ? [bytesFor(image.file).then(data => ({ key, file: image.file, data }))] : [];
  }));
  const keys = new Set(images.flatMap(image => image.key ? [image.key] : []));
  const removed = [...savedKeys.get(profileID) ?? []].filter(key => !keys.has(key));
  const stored: StoredDraft = { ...draft, schema: 2, images };
  const db = await openDatabase();
  return new Promise((resolve, reject) => {
    const transaction = db.transaction(['drafts', 'images'], 'readwrite');
    for (const image of changed) transaction.objectStore('images').put(image.data, image.key);
    for (const key of removed) transaction.objectStore('images').delete(key);
    transaction.objectStore('drafts').put(stored, profileID);
    transaction.oncomplete = () => {
      for (const image of changed) savedFiles.set(image.key, image.file);
      for (const key of removed) savedFiles.delete(key);
      savedKeys.set(profileID, keys);
      resolve();
    };
    transaction.onabort = () => reject(transaction.error);
    transaction.onerror = () => reject(transaction.error);
  });
}

export function writeDraft(profileID: string, draft: AccountDraft): Promise<void> {
  // Byte reads are async. Serialize writes per account so an older, slower save
  // cannot overwrite a more recent edit. Other accounts save independently.
  const pending = (writes.get(profileID) ?? Promise.resolve()).catch(() => {}).then(() => persistDraft(profileID, draft));
  writes.set(profileID, pending);
  const cleanup = () => { if (writes.get(profileID) === pending) writes.delete(profileID); };
  void pending.then(() => { failedDraftWrites.delete(profileID); cleanup(); }, () => { failedDraftWrites.add(profileID); cleanup(); });
  return pending;
}

const failedDraftWrites = new Set<string>();

export async function flushDrafts(): Promise<void> {
  await Promise.all([...writes.values()]);
  if (failedDraftWrites.size) throw new Error('Could not save all drafts. Keep Postika open and try again before restarting.');
}
