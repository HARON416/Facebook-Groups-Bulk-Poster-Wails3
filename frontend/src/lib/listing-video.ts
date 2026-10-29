import { mp4Metadata } from './mp4-metadata';

// Read local metadata without uploading the seller's file. Always release the
// temporary player and URL, including on cancellation or unreadable media.
export async function inspectListingVideo(file: File, signal: AbortSignal): Promise<void> {
  if (signal.aborted) throw new DOMException('Aborted', 'AbortError');
  if (/\.(mp4|m4v|mov)$/i.test(file.name) || ['video/mp4', 'video/quicktime', 'video/x-m4v'].includes(file.type)) {
    const metadata = await mp4Metadata(file, signal);
    if (metadata.duration > 60) throw new Error('invalid-video');
    return;
  }
  return new Promise((resolve, reject) => {
    if (signal.aborted) { reject(new DOMException('Aborted', 'AbortError')); return; }
    const player = document.createElement('video');
    if (!file.type.startsWith('video/') || !player.canPlayType(file.type)) { reject(new Error('invalid-video')); return; }
    const url = URL.createObjectURL(file);
    let timer: ReturnType<typeof setTimeout>;
    const finish = (error?: Error) => {
      clearTimeout(timer);
      signal.removeEventListener('abort', abort);
      player.onloadedmetadata = null; player.onerror = null;
      player.removeAttribute('src'); player.load(); URL.revokeObjectURL(url);
      if (error) reject(error); else resolve();
    };
    const abort = () => finish(new DOMException('Aborted', 'AbortError'));
    signal.addEventListener('abort', abort, { once: true });
    player.preload = 'metadata';
    player.onloadedmetadata = () => finish(Number.isFinite(player.duration) && player.videoWidth > 0 && player.videoHeight > 0 && player.duration > 0 && player.duration <= 60 ? undefined : new Error('invalid-video'));
    player.onerror = () => finish(new Error('invalid-video'));
    timer = setTimeout(() => finish(new Error('invalid-video')), 20000);
    player.src = url;
  });
}
