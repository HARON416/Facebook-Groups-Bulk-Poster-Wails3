export function listingMapURLs(latitude: number, longitude: number, locale = 'en') {
  if (!Number.isFinite(latitude) || !Number.isFinite(longitude)
    || Math.abs(latitude) > 90 || Math.abs(longitude) > 180) return null;
  const center = `${latitude},${longitude}`;
  // Match the supplied Marketplace static-map format and approximate area.
  const params = new URLSearchParams({
    v: '2070', theme: 'default', ccb: '4-4', size: '500x80',
    language: locale.replaceAll('-', '_'), scale: '1', zoom: '10', center,
    circle: `weight:2|color:0x4D6AA47f|fillcolor:0x4D6AA41c|${center}|2k`,
    _nc_client_id: 'marketplace_pdp_map_comet',
    _nc_client_caller: 'MarketplaceStaticMap.react',
  });
  return {
    image: `https://external-iad3-2.xx.fbcdn.net/static_map.php?${params}`,
    link: `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(center)}`,
  };
}
