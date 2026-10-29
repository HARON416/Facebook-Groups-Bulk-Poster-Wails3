<script module lang="ts">
  import type { ListingFieldOption } from '../lib/marketplace-fields';

  export function listingConditions(): ListingFieldOption[] {
    return [
      { value: 'new', label: 'New' },
      { value: 'used-like-new', label: 'Used - Like New' },
      { value: 'used-good', label: 'Used - Good' },
      { value: 'used-fair', label: 'Used - Fair' },
    ];
  }
</script>
