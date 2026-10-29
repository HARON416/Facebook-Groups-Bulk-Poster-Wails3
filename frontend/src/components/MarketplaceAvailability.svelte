<script module lang="ts">
  import type { ListingFieldOption } from '../lib/marketplace-fields';

  export function listingAvailability(): ListingFieldOption[] {
    return [
      { value: 'single', label: 'List as Single Item', description: 'If you\'re selling one item, show "Only one" on your listing.' },
      { value: 'in-stock', label: 'List as In Stock', description: 'If you\'re selling more than one item, show "In Stock" on your listing.' },
    ];
  }
</script>
