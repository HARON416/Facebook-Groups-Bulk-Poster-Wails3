import { Call } from '@wailsio/runtime';
import { thumbnailTask } from './thumbnail-task';
import { encodeMediaFile } from './media-upload';

export function videoThumbnail(file: File, profileID: string | undefined, signal: AbortSignal): Promise<string> {
  return thumbnailTask(async taskSignal => {
    if (!profileID) throw new Error('thumbnail-unavailable');
    const encoded = await encodeMediaFile(file, taskSignal);
    if (taskSignal.aborted) throw new DOMException('Aborted', 'AbortError');
    // Always use software decoding: some webviews report successful canvas
    // capture while returning corrupted or black pixels for valid H.264 video.
    const request = Call.ByID(1437725365, profileID, encoded);
    const abort = () => { void request.cancel().catch(() => {}); };
    taskSignal.addEventListener('abort', abort, { once: true });
    try {
      const result = await request;
      if (typeof result !== 'string' || !result.startsWith('data:image/jpeg;base64,')) throw new Error('thumbnail-unavailable');
      return result;
    } finally { taskSignal.removeEventListener('abort', abort); }
  }, signal);
}
