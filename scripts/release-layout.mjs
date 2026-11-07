import { parseVersion } from './version.mjs';

export const cdnOrigin = 'https://postika-desktop-application.b-cdn.net';
export function downloadPrefix(version) {
  parseVersion(version);
  return `${cdnOrigin}/downloads/${version}/`;
}

export function websiteRelease(version, { date = new Date().toISOString().slice(0, 10), notes = [], macZipOnly = false } = {}) {
  const prefix = downloadPrefix(version);
  const packagePrefix = `postika-${version}-`;
  return { version, date, notes, downloads: [
    ['windows', 'Windows installer', 'Windows x64 (.exe)', 'windows-amd64-installer.exe'],
    ['windows', 'Windows portable', 'Windows x64 (.zip)', 'windows-amd64-portable.zip'],
    ['macos', 'Apple Silicon', `M-series Macs (${macZipOnly ? '.app.zip' : '.dmg'})`, `darwin-arm64${macZipOnly ? '.app.zip' : '.dmg'}`],
    ['macos', 'Intel Mac', `Intel Macs (${macZipOnly ? '.app.zip' : '.dmg'})`, `darwin-amd64${macZipOnly ? '.app.zip' : '.dmg'}`],
    ['linux', 'AppImage', 'Linux x64 (portable)', 'linux-amd64.AppImage'],
    ['linux', 'Debian / Ubuntu', 'Linux x64 (.deb)', 'linux-amd64.deb'],
    ['linux', 'Fedora / RPM', 'Linux x64 (.rpm)', 'linux-amd64.rpm'],
    ['linux', 'Arch Linux', 'Linux x64 (.pkg.tar.zst)', 'linux-amd64.pkg.tar.zst'],
  ].map(([os, name, detail, suffix]) => ({ os, name, detail, url: prefix + packagePrefix + suffix })) };
}
