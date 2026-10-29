import type { ComponentProps } from 'svelte';
import type PhosphorIcon from '../components/PhosphorIcon.svelte';
import { listingCategories } from '../components/MarketplaceCategories.svelte';
import { listingConditions } from '../components/MarketplaceConditions.svelte';
import { listingAvailability } from '../components/MarketplaceAvailability.svelte';
import { listingHomeOptions } from '../components/MarketplaceHomes.svelte';
import { listingVehicleTypes, listingVehicleYears, listingCarTruckOptions, listingMotorcycleOptions } from '../components/MarketplaceVehicles.svelte';

export interface ListingFieldGroup {
  id: string; label: string; icon?: ComponentProps<typeof PhosphorIcon>['name'];
}
export interface ListingFieldOption {
  value: string; label: string; description?: string;
  group?: ListingFieldGroup; icon?: ComponentProps<typeof PhosphorIcon>['name'];
}
export type CarTruckField = 'make' | 'bodyStyle' | 'exteriorColor' | 'interiorColor' | 'condition' | 'fuelType' | 'transmission';
export type MotorcycleField = 'make' | 'exteriorColor' | 'fuelType' | 'transmission';
export type ItemListingField = 'category' | 'condition' | 'availability' | 'location';
export type HomeListingField = 'homeOffer' | 'propertyType' | 'laundry' | 'parking' | 'airConditioning' | 'heating' | 'bathroomType';

// Resolve the owner-supplied home lists in the current locale.
export const homeListingFieldOptions: Record<HomeListingField, ListingFieldOption[]> = {
  get homeOffer() { return listingHomeOptions().homeOffer; },
  get propertyType() { return listingHomeOptions().propertyType; },
  get laundry() { return listingHomeOptions().laundry; },
  get parking() { return listingHomeOptions().parking; },
  get airConditioning() { return listingHomeOptions().airConditioning; },
  get heating() { return listingHomeOptions().heating; },
  get bathroomType() { return listingHomeOptions().bathroomType; },
};

// Shared option lists for listing forms. Values are local draft keys, not
// Facebook field IDs. Display labels are resolved in the active locale.
export const itemListingFieldOptions: Record<ItemListingField, ListingFieldOption[]> = {
  get category() { return listingCategories(); },
  get condition() { return listingConditions(); },
  get availability() { return listingAvailability(); },
  location: [],
};

// Vehicle dropdown values will be supplied with their supported option lists.
export const vehicleListingFieldOptions: { vehicleType: ListingFieldOption[]; year: ListingFieldOption[]; carTruck: Record<CarTruckField, ListingFieldOption[]>; motorcycle: Record<MotorcycleField, ListingFieldOption[]> } = {
  get vehicleType() { return listingVehicleTypes(); },
  get year() { return listingVehicleYears(); },
  get carTruck() { return listingCarTruckOptions(); },
  get motorcycle() { return listingMotorcycleOptions(); },
};
