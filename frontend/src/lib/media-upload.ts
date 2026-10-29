import { Call } from '@wailsio/runtime';
import type { ItemListingFields } from './marketplace-drafts';

export function encodeMediaFile(file: File, signal: AbortSignal): Promise<string> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) { reject(new DOMException('Aborted', 'AbortError')); return; }
    const reader = new FileReader();
    const abort = () => reader.abort();
    signal.addEventListener('abort', abort, { once: true });
    reader.onloadend = () => signal.removeEventListener('abort', abort);
    reader.onabort = () => reject(new DOMException('Aborted', 'AbortError'));
    reader.onerror = () => reject(reader.error);
    reader.onload = () => resolve(String(reader.result).split(',', 2)[1]);
    reader.readAsDataURL(file);
  });
}

export async function testMarketplaceItemMedia(profileID: string, photos: File[], video: File | null, fields: ItemListingFields, signal: AbortSignal): Promise<void> {
  if (!photos.length && !video) throw new Error('Select at least one image or video');
  if (photos.length > 10) throw new Error('You may only select a maximum of 10 photos');
  const uploads: { name: string; data: string }[] = [];
  for (const file of photos) uploads.push({ name: file.name, data: await encodeMediaFile(file, signal) });
  const movie = video ? { name: video.name, data: await encodeMediaFile(video, signal) } : null;
  if (signal.aborted) throw new DOMException('Aborted', 'AbortError');
  // Stable native binding registered in marketplace_item_test_media.go.
  const request = Call.ByID(4270654618, profileID, { photos: uploads, video: movie, fields: {
    title: fields.title, price: fields.price, category: fields.category, condition: fields.condition,
    description: fields.description, availability: fields.availability,
    tags: fields.tags.split(',').map(tag => tag.trim()).filter(Boolean), sku: fields.sku,
    location: fields.locationData, publicMeetup: fields.publicMeetup, doorPickup: fields.doorPickup,
    doorDropoff: fields.doorDropoff, promoteAfterPublish: fields.promoteAfterPublish, hideFromFriends: fields.hideFromFriends,
  } });
  const abort = () => { void request.cancel().catch(() => {}); };
  signal.addEventListener('abort', abort, { once: true });
  try { await request; } finally { signal.removeEventListener('abort', abort); }
}
