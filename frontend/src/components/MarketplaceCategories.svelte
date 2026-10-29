<script module lang="ts">
  import type { ListingFieldOption, ListingFieldGroup } from '../lib/marketplace-fields';

  // Labels and order follow the owner-supplied category menu. Resolve them on
  // each call so open forms and saved selections follow language changes.
  export function listingCategories(): ListingFieldOption[] {
    const groups = {
      home: { id: 'home-garden', label: 'Home & Garden', icon: 'house' },
      entertainment: { id: 'entertainment', label: 'Entertainment', icon: 'game-controller' },
      clothing: { id: 'clothing-accessories', label: 'Clothing & Accessories', icon: 't-shirt' },
      family: { id: 'family', label: 'Family', icon: 'users' },
      electronics: { id: 'electronics', label: 'Electronics', icon: 'desktop' },
      hobbies: { id: 'hobbies', label: 'Hobbies', icon: 'bicycle' },
      classifieds: { id: 'classifieds', label: 'Classifieds', icon: 'binoculars' },
    } satisfies Record<string, ListingFieldGroup>;
    const shipping = /* @wc-include */ 'Shipping available';
    return [
      { value: 'tools', label: 'Tools', group: groups.home },
      { value: 'furniture', label: 'Furniture', group: groups.home },
      { value: 'household', label: 'Household', description: shipping, group: groups.home },
      { value: 'garden', label: 'Garden', group: groups.home },
      { value: 'appliances', label: 'Appliances', group: groups.home },
      { value: 'video-games', label: 'Video Games', group: groups.entertainment },
      { value: 'books-movies-music', label: 'Books, Movies & Music', description: shipping, group: groups.entertainment },
      { value: 'bags-luggage', label: 'Bags & Luggage', description: shipping, group: groups.clothing },
      { value: 'womens-clothing-shoes', label: "Women's clothing & shoes", description: shipping, group: groups.clothing },
      { value: 'mens-clothing-shoes', label: "Men's clothing & shoes", description: shipping, group: groups.clothing },
      { value: 'jewelry-accessories', label: 'Jewelry & Accessories', description: shipping, group: groups.clothing },
      { value: 'health-beauty', label: 'Health & beauty', description: shipping, group: groups.family },
      { value: 'pet-supplies', label: 'Pet Supplies', group: groups.family },
      { value: 'baby-kids', label: 'Baby & kids', description: shipping, group: groups.family },
      { value: 'toys-games', label: 'Toys & Games', description: shipping, group: groups.family },
      { value: 'electronics-computers', label: 'Electronics & computers', description: shipping, group: groups.electronics },
      { value: 'mobile-phones', label: 'Mobile phones', description: shipping, group: groups.electronics },
      { value: 'bicycles', label: 'Bicycles', group: groups.hobbies },
      { value: 'arts-crafts', label: 'Arts & Crafts', group: groups.hobbies },
      { value: 'sports-outdoors', label: 'Sports & Outdoors', description: shipping, group: groups.hobbies },
      { value: 'auto-parts', label: 'Auto parts', group: groups.hobbies },
      { value: 'musical-instruments', label: 'Musical Instruments', group: groups.hobbies },
      { value: 'antiques-collectibles', label: 'Antiques & Collectibles', group: groups.hobbies },
      { value: 'garage-sale', label: 'Garage Sale', group: groups.classifieds },
      { value: 'miscellaneous', label: 'Miscellaneous', group: groups.classifieds },
      { value: 'vehicles', label: 'Vehicles', icon: 'car' },
    ];
  }
</script>
