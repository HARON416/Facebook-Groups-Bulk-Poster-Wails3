import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mp4Metadata } from '../src/lib/mp4-metadata.ts';

function box(type, ...parts) {
  const payload = Buffer.concat(parts);
  const header = Buffer.alloc(8);
  header.writeUInt32BE(payload.length + 8); header.write(type, 4);
  return Buffer.concat([header, payload]);
}
function movie(seconds, { version = 0, video = true, atEnd = false, extended = false } = {}) {
  const mvhd = Buffer.alloc(version === 1 ? 32 : 20);
  mvhd[0] = version;
  mvhd.writeUInt32BE(1000, version === 1 ? 20 : 12);
  if (version === 1) mvhd.writeBigUInt64BE(BigInt(Math.round(seconds * 1000)), 24);
  else mvhd.writeUInt32BE(Math.round(seconds * 1000), 16);
  const tkhd = Buffer.alloc(84);
  tkhd.writeUInt32BE(4096 * 65536, 76); tkhd.writeUInt32BE(2160 * 65536, 80);
  const handler = Buffer.alloc(12); handler.write(video ? 'vide' : 'soun', 8);
  let moov = box('moov', box('mvhd', mvhd), box('trak', box('tkhd', tkhd), box('mdia', box('hdlr', handler))));
  if (extended) {
    const header = Buffer.alloc(16); header.writeUInt32BE(1); header.write('moov', 4); header.writeBigUInt64BE(BigInt(moov.length + 8), 8);
    moov = Buffer.concat([header, moov.subarray(8)]);
  }
  const ftyp = box('ftyp', Buffer.from('isom0000'));
  const mdat = box('mdat', Buffer.alloc(32));
  return new Blob(atEnd ? [ftyp, mdat, moov] : [ftyp, moov, mdat]);
}
const inspect = blob => mp4Metadata(blob, new AbortController().signal);

test('4K MP4 metadata can be read without any browser decoder', async () => {
  for (const options of [{}, { atEnd: true }, { version: 1 }, { extended: true }]) {
    assert.deepEqual(await inspect(movie(11.947, options)), { duration: 11.947, width: 4096, height: 2160 });
  }
});
test('metadata retains duration at and above the 60-second boundary for validation', async () => {
  assert.equal((await inspect(movie(60))).duration, 60);
  assert.ok((await inspect(movie(60.001))).duration > 60);
});
test('audio-only, zero duration, and malformed files are rejected', async () => {
  for (const blob of [movie(5, { video: false }), movie(0), new Blob(['not a video']), new Blob([Buffer.from([0, 0, 0, 2, 109, 111, 111, 118])])]) {
    await assert.rejects(inspect(blob), /invalid-video/);
  }
});
test('cancelled metadata inspection stops before accessing the file', async () => {
  const controller = new AbortController(); controller.abort();
  await assert.rejects(mp4Metadata(movie(10), controller.signal), { name: 'AbortError' });
});
