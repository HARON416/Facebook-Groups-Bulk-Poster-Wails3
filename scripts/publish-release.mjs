import { createHash } from 'node:crypto';
import { closeSync, createReadStream, existsSync, openSync, readFileSync, readdirSync, realpathSync, rmSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { releaseVersion } from './package-release.mjs';
import { verifyFlatRelease } from './flatten-release.mjs';
import { generateFeeds } from './update-feeds.mjs';
import { generateWebsiteRelease } from './website-release.mjs';
import { cdnOrigin } from './release-layout.mjs';
import { parseVersion } from './version.mjs';

export function bunnyConfig(env = process.env) {
  for (const key of ['BUNNY_STORAGE_ZONE', 'BUNNY_STORAGE_HOST', 'BUNNY_STORAGE_PASSWORD']) {
    if (!env[key]?.trim()) throw new Error(`Set ${key} in ignored .env (see .env.example).`);
  }
  const host = env.BUNNY_STORAGE_HOST.trim();
  const zone = env.BUNNY_STORAGE_ZONE.trim();
  if (!/^(?:[a-z]{2,3}\.)?storage\.bunnycdn\.com$/.test(host)) throw new Error('BUNNY_STORAGE_HOST must be your regional Bunny Storage hostname, without https:// or a path.');
  if (!/^[a-zA-Z0-9][a-zA-Z0-9-]*$/.test(zone)) throw new Error('Invalid Bunny Storage Zone name.');
  return { host, zone, password: env.BUNNY_STORAGE_PASSWORD };
}

export function bunnyClient(config, { request = fetch, sleep = ms => new Promise(resolve => setTimeout(resolve, ms)) } = {}) {
  const storageURL = path => `https://${config.host}/${config.zone}/${path}`;
  async function send(url, init, label) {
    try { return await request(url, { redirect: 'error', signal: AbortSignal.timeout(600_000), ...init }); }
    catch { throw new Error(`Network request failed for ${label}; credentials have not been logged.`); }
  }
  async function getStorage(path) {
    const response = await send(storageURL(path), { headers: { AccessKey: config.password } }, path);
    if (response.status === 404) { await response.body?.cancel(); return null; }
    if (!response.ok) { await response.body?.cancel(); throw new Error(`Bunny Storage returned HTTP ${response.status} for ${path || 'authentication'}. Check the storage password and regional hostname.`); }
    return response.text();
  }
  async function upload(file, path) {
    const checksum = createHash('sha256').update(readFileSync(file)).digest('hex').toUpperCase();
    // A fresh stream per request avoids buffering entire releases in memory.
    const stream = createReadStream(file);
    try {
      const response = await send(storageURL(path), {
        method: 'PUT', duplex: 'half', body: stream,
        headers: { AccessKey: config.password, Checksum: checksum, 'Content-Length': String(statSync(file).size),
          'Content-Type': path.endsWith('.json') ? 'application/json' : 'application/octet-stream' },
      }, path);
      await response.body?.cancel();
      if (!response.ok) throw new Error(`Bunny upload failed with HTTP ${response.status} for ${path}.`);
    } finally { stream.destroy(); }
  }
  async function verifyPublic(file, path, feed = false) {
    const expected = createHash('sha256').update(readFileSync(file)).digest('hex');
    let reason = '';
    for (let attempt = 0; attempt < 5; attempt++) {
      const response = await send(`${cdnOrigin}/${path}`, { headers: { 'Cache-Control': 'no-cache' } }, path);
      if (response.ok) {
        const hash = createHash('sha256');
        for await (const chunk of response.body) hash.update(chunk);
        const cache = response.headers.get('cache-control') ?? '';
        const json = (response.headers.get('content-type') ?? '').includes('application/json');
        if (hash.digest('hex') === expected && (!feed || (/\bno-cache\b|\bno-store\b/i.test(cache) && json))) return;
        reason = feed ? 'content mismatch or missing JSON/no-cache headers' : 'checksum mismatch';
      } else {
        reason = `HTTP ${response.status}`;
        await response.body?.cancel();
      }
      if (attempt < 4) await sleep(1000 * (attempt + 1));
    }
    throw new Error(`Public verification failed for ${path}: ${reason}. Check CDN replication/cache rules, then retry make publish-release.`);
  }
  return { getStorage, upload, verifyPublic };
}

function newer(left, right) {
  const a = parseVersion(left), b = parseVersion(right);
  const index = a.findIndex((part, index) => part !== b[index]);
  return index >= 0 && a[index] > b[index];
}

// Refuse a rollback or a different payload under an already published version.
export async function checkPublishedRelease(client, version, packages, feeds) {
  for (const feed of readdirSync(feeds)) {
    const remote = await client.getStorage(`updates/${feed}`);
    if (remote === null) continue;
    const manifest = JSON.parse(remote);
    if (newer(manifest.version, version)) throw new Error(`Feed ${feed} already publishes ${manifest.version}; refusing to publish older ${version}.`);
    if (manifest.version === version) {
      const local = JSON.parse(readFileSync(join(feeds, feed), 'utf8'));
      if (JSON.stringify(manifest.artifacts) !== JSON.stringify(local.artifacts)) throw new Error(`Feed ${feed} already publishes different payloads for ${version}; bump the version.`);
    }
  }
  const sums = await client.getStorage(`downloads/${version}/SHA256SUMS`);
  if (sums !== null && sums !== readFileSync(join(packages, 'SHA256SUMS'), 'utf8')) {
    throw new Error(`Different packages already exist for ${version}; bump the version.`);
  }
}

export async function publishRelease({ root, client = bunnyClient(bunnyConfig()), prepareFeeds = generateFeeds, log = console.log } = {}) {
  const version = releaseVersion(root);
  const packages = join(root, 'bin/releases', version);
  verifyFlatRelease(packages, version);
  const lock = join(root, 'bin/.publish.lock');
  let fd;
  try { fd = openSync(lock, 'wx'); }
  catch { throw new Error('Another local publisher is running (bin/.publish.lock). If it crashed, check that it stopped before removing the lock.'); }
  try {
    if (await client.getStorage('') === null) throw new Error('Bunny Storage Zone not found. Check .env.');
    const feeds = prepareFeeds(root);
    await checkPublishedRelease(client, version, packages, feeds);
    const metadata = generateWebsiteRelease(root);
    // The checksum manifest acts as a completion marker for safe retries.
    const names = readdirSync(packages).sort((a, b) => (a === 'SHA256SUMS') - (b === 'SHA256SUMS') || a.localeCompare(b));
    for (const name of names) {
      const path = `downloads/${version}/${name}`;
      log(`Uploading ${path}`);
      await client.upload(join(packages, name), path);
      await client.verifyPublic(join(packages, name), path);
    }
    const metadataPath = `downloads/${version}/release.json`;
    await client.upload(metadata, metadataPath);
    await client.verifyPublic(metadata, metadataPath);
    // No stable feed is touched until every package is publicly checksum-verified.
    for (const name of readdirSync(feeds).sort()) {
      const path = `updates/${name}`;
      log(`Publishing ${path}`);
      await client.upload(join(feeds, name), path);
      await client.verifyPublic(join(feeds, name), path, true);
    }
    log(`Published ${version}. Rebuild and deploy the website to include its generated download links.`);
    return version;
  } catch (error) {
    throw new Error(`${error.message}\nRetry this version with make publish-release; it does not bump or rebuild. Older remote packages have been preserved.`);
  } finally { closeSync(fd); rmSync(lock); }
}

export async function preflightPublish(root) {
  const config = bunnyConfig();
  for (const file of ['.update-keys/updater.key', 'internal/updateauth/public-key.pem', `.tools/bin/${process.platform === 'win32' ? 'wails3.exe' : 'wails3'}`]) {
    if (!existsSync(join(root, file))) throw new Error(`Required publishing file missing: ${file}`);
  }
  const client = bunnyClient(config);
  if (await client.getStorage('') === null) throw new Error('Bunny Storage Zone not found.');
  return client;
}

if (process.argv[1] && realpathSync(resolve(process.argv[1])) === realpathSync(fileURLToPath(import.meta.url))) {
  try {
    const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
    if (process.argv.includes('--plan')) {
      const version = releaseVersion(root);
      console.log(`Publish packages to ${cdnOrigin}/downloads/${version}/, then five signed feeds to ${cdnOrigin}/updates/. No uploads were performed.`);
    } else await publishRelease({ root });
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
