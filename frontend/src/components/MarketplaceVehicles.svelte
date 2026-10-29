<script module lang="ts">
  import { vehicleMakes } from '../lib/vehicle-makes';
  import { motorcycleMakes } from '../lib/motorcycle-makes';
  import type { ListingFieldOption, CarTruckField } from '../lib/marketplace-fields';

  // Values are local draft keys; these are the supplied display options.
  export function listingVehicleTypes(): ListingFieldOption[] {
    return [
      { value: 'car-truck', label: 'Car/Truck' },
      { value: 'motorcycle', label: 'Motorcycle' },
      { value: 'powersport', label: 'Powersport' },
      { value: 'rv-camper', label: 'RV/Camper' },
      { value: 'trailer', label: 'Trailer' },
      { value: 'boat', label: 'Boat' },
      { value: 'commercial-industrial', label: 'Commercial/Industrial' },
      { value: 'other', label: 'Other' },
    ];
  }
  // Preserve the supplied descending range, including the next model year.
  export function listingVehicleYears(): ListingFieldOption[] {
    return Array.from({ length: 2027 - 1901 + 1 }, (_, index) => {
      const year = String(2027 - index);
      return { value: year, label: year };
    });
  }
  // Supplied Car/Truck, Motorcycle, Powersport, and RV/Camper colors share this order.
  export function listingVehicleColors(): ListingFieldOption[] {
    return [
      { value: 'black', label: "Black" },
      { value: 'blue', label: "Blue" },
      { value: 'brown', label: "Brown" },
      { value: 'gold', label: "Gold" },
      { value: 'green', label: "Green" },
      { value: 'gray', label: "Gray" },
      { value: 'pink', label: "Pink" },
      { value: 'purple', label: "Purple" },
      { value: 'red', label: "Red" },
      { value: 'silver', label: "Silver" },
      { value: 'orange', label: "Orange" },
      { value: 'white', label: "White" },
      { value: 'yellow', label: "Yellow" },
      { value: 'charcoal', label: "Charcoal" },
      { value: 'off-white', label: "Off white" },
      { value: 'tan', label: "Tan" },
      { value: 'beige', label: "Beige" },
      { value: 'burgundy', label: "Burgundy" },
      { value: 'turquoise', label: "Turquoise" },
      { value: 'other', label: "Other" },
    ];
  }
  // Supplied motorcycle colors, fuel types, and transmissions match Car/Truck.
  export function listingMotorcycleOptions() {
    const shared = listingCarTruckOptions();
    return {
      make: motorcycleMakes.map(make => ({ value: make, label: make })),
      exteriorColor: shared.exteriorColor,
      fuelType: shared.fuelType,
      transmission: shared.transmission,
    };
  }
  // Keep the supplied order and separate choices, including Gasoline/Petrol.
  // Powersport and RV/Camper also use this supplied fuel list.
  export function listingCarTruckOptions(): Record<CarTruckField, ListingFieldOption[]> {
    return {
      make: vehicleMakes.map(make => ({ value: make, label: make })),
      bodyStyle: [
        { value: 'coupe', label: "Coupe" },
        { value: 'truck', label: "Truck" },
        { value: 'sedan', label: "Sedan" },
        { value: 'hatchback', label: "Hatchback" },
        { value: 'suv', label: "SUV" },
        { value: 'convertible', label: "Convertible" },
        { value: 'wagon', label: "Wagon" },
        { value: 'minivan', label: "Minivan" },
        { value: 'small-car', label: "Small Car" },
        { value: 'other', label: "Other" },
      ],
      exteriorColor: listingVehicleColors(),
      interiorColor: listingVehicleColors(),
      condition: [
        { value: 'excellent', label: "Excellent" },
        { value: 'very-good', label: "Very good" },
        { value: 'good', label: "Good" },
        { value: 'fair', label: "Fair" },
        { value: 'poor', label: "Poor" },
      ],
      fuelType: [
        { value: 'diesel', label: "Diesel" },
        { value: 'electric', label: "Electric" },
        { value: 'gasoline', label: "Gasoline" },
        { value: 'flex', label: "Flex" },
        { value: 'hybrid', label: "Hybrid" },
        { value: 'petrol', label: "Petrol" },
        { value: 'plug-in-hybrid', label: "Plug-in hybrid" },
        { value: 'other', label: "Other" },
      ],
      transmission: [
        { value: 'manual', label: 'Manual transmission' },
        { value: 'automatic', label: 'Automatic transmission' },
      ],
    };
  }
</script>
