<script module lang="ts">
  import type { HomeListingField, ListingFieldOption } from '../lib/marketplace-fields';

  // Preserve the supplied labels and order; resolve labels in the active locale.
  export function listingHomeOptions(): Record<HomeListingField, ListingFieldOption[]> {
    return {
      bathroomType: [
        { value: 'private', label: 'Private' },
        { value: 'shared', label: 'Shared' },
      ],
      homeOffer: [
        { value: 'rent', label: 'Rent' },
        { value: 'sale', label: 'For Sale' },
      ],
      propertyType: [
        { value: 'apartment', label: 'Apartment' },
        { value: 'house', label: 'House' },
        { value: 'townhouse', label: 'Townhouse' },
      ],
      laundry: [
        { value: 'in-unit', label: 'In-unit laundry' },
        { value: 'in-building', label: 'Laundry in building' },
        { value: 'available', label: 'Laundry available' },
        { value: 'none', label: 'None' },
      ],
      parking: [
        { value: 'garage', label: 'Garage parking' },
        { value: 'street', label: 'Street parking' },
        { value: 'off-street', label: 'Off-street parking' },
        { value: 'available', label: 'Parking available' },
        { value: 'none', label: 'None' },
      ],
      airConditioning: [
        { value: 'central', label: 'Central AC' },
        { value: 'available', label: 'AC available' },
        { value: 'none', label: 'None' },
      ],
      heating: [
        { value: 'central', label: 'Central heating' },
        { value: 'electric', label: 'Electric heating' },
        { value: 'gas', label: 'Gas heating' },
        { value: 'radiator', label: 'Radiator heating' },
        { value: 'available', label: 'Heating available' },
        { value: 'none', label: 'None' },
      ],
    };
  }
</script>
