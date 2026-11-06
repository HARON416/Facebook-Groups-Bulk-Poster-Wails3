import { Call } from '@wailsio/runtime';
import type { ItemListingFields, MarketplaceListingKind } from './marketplace-drafts';

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

export function canTestMarketplaceListing(kind: MarketplaceListingKind, fields: ItemListingFields): boolean {
  return kind === 'items' || (kind === 'vehicles' && ['car-truck', 'motorcycle', 'powersport', 'rv-camper', 'trailer', 'boat', 'commercial-industrial', 'other'].includes(fields.vehicleType)) || (kind === 'homes' && fields.homeOffer === 'sale');
}

export async function testMarketplaceItemMedia(profileID: string, photos: File[], video: File | null, fields: ItemListingFields, signal: AbortSignal): Promise<void> {
  return testMarketplaceListingMedia('items', profileID, photos, video, fields, signal);
}

export async function testMarketplaceListingMedia(kind: MarketplaceListingKind, profileID: string, photos: File[], video: File | null, fields: ItemListingFields, signal: AbortSignal): Promise<void> {
  if (!canTestMarketplaceListing(kind, fields)) throw new Error('This listing type is not supported for testing yet');
  const vehicle = kind === 'vehicles';
  const home = kind === 'homes';
  if (!photos.length && !video) throw new Error('Select at least one image or video');
  if (photos.length > (home ? 50 : vehicle ? 20 : 10)) throw new Error(home ? 'You may only select a maximum of 50 photos' : vehicle ? 'You may only select a maximum of 20 photos' : 'You may only select a maximum of 10 photos');
  const uploads: { name: string; data: string }[] = [];
  for (const file of photos) uploads.push({ name: file.name, data: await encodeMediaFile(file, signal) });
  const movie = video ? { name: video.name, data: await encodeMediaFile(video, signal) } : null;
  if (signal.aborted) throw new DOMException('Aborted', 'AbortError');
  // Stable native binding registered in marketplace_item_test_media.go.
  const itemFields = {
    title: fields.title, price: fields.price, category: fields.category, condition: fields.condition,
    description: fields.description, availability: fields.availability,
    tags: fields.tags.split(',').map(tag => tag.trim()).filter(Boolean), sku: fields.sku,
    location: fields.locationData, publicMeetup: fields.publicMeetup, doorPickup: fields.doorPickup,
    doorDropoff: fields.doorDropoff, promoteAfterPublish: fields.promoteAfterPublish, hideFromFriends: fields.hideFromFriends,
  };
  const vehicleFields = {
    vehicleType: fields.vehicleType, year: fields.year, make: fields.make, model: fields.model,
    mileage: fields.mileage, price: fields.price, bodyStyle: fields.bodyStyle,
    exteriorColor: fields.exteriorColor, interiorColor: fields.interiorColor,
    vehicleCondition: fields.vehicleCondition, fuelType: fields.fuelType, transmission: fields.transmission,
    cleanTitle: fields.cleanTitle, description: fields.description, location: fields.locationData,
    promoteAfterPublish: fields.promoteAfterPublish,
  };
  const homeFields = {
    homeOffer: fields.homeOffer, propertyType: fields.propertyType, bedrooms: fields.bedrooms,
    bathrooms: fields.bathrooms, price: fields.price, address: fields.address, location: fields.locationData,
    description: fields.description, squareFeet: fields.squareFeet, laundry: fields.laundry,
    parking: fields.parking, airConditioning: fields.airConditioning, heating: fields.heating,
    promoteAfterPublish: fields.promoteAfterPublish,
  };
  const details = home ? { homeFields } : vehicle ? { vehicleFields } : { fields: itemFields };
  const request = Call.ByID(home ? 4270654620 : vehicle ? 4270654619 : 4270654618,
    profileID, { photos: uploads, video: movie, ...details });
  const abort = () => { void request.cancel().catch(() => {}); };
  signal.addEventListener('abort', abort, { once: true });
  try { await request; } finally { signal.removeEventListener('abort', abort); }
}
