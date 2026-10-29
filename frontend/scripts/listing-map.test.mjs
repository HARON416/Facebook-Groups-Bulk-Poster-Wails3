import { test } from 'node:test';
import assert from 'node:assert/strict';
import { listingMapURLs } from '../src/lib/listing-map.ts';

test('map image and external link follow the selected coordinates', () => {
  const urls = listingMapURLs(49.281808019386, -123.12047562107, 'en-US');
  const image = new URL(urls.image);
  assert.equal(image.hostname, 'external-iad3-2.xx.fbcdn.net');
  assert.equal(image.searchParams.get('center'), '49.281808019386,-123.12047562107');
  assert.equal(image.searchParams.get('size'), '500x80');
  assert.equal(image.searchParams.get('language'), 'en_US');
  assert.ok(image.searchParams.get('circle').endsWith('|49.281808019386,-123.12047562107|2k'));
  assert.equal(new URL(urls.link).searchParams.get('query'), image.searchParams.get('center'));
  assert.notEqual(listingMapURLs(-1.28, 36.82).image, urls.image);
});

test('invalid coordinates do not generate map requests; zero coordinates remain valid', () => {
  for (const [lat, lon] of [[NaN, 0], [0, Infinity], [91, 0], [0, -181], [undefined, undefined]]) {
    assert.equal(listingMapURLs(lat, lon), null);
  }
  assert.ok(listingMapURLs(0, 0));
});
