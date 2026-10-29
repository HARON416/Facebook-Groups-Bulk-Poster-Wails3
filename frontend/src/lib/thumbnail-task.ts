// Bound the complete operation, including file reading and native IPC. A stalled
// native promise must not leave the listing form permanently busy.
export function thumbnailTask<T>(run: (signal: AbortSignal) => Promise<T>, signal: AbortSignal, timeoutMs = 15000): Promise<T> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) { reject(new DOMException('Aborted', 'AbortError')); return; }
    const controller = new AbortController();
    let settled = false;
    const finish = (error: unknown, result?: T) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer); signal.removeEventListener('abort', abort);
      if (error) { controller.abort(); reject(error); } else resolve(result as T);
    };
    const abort = () => finish(new DOMException('Aborted', 'AbortError'));
    const timer = setTimeout(() => finish(new Error('thumbnail-timeout')), timeoutMs);
    signal.addEventListener('abort', abort, { once: true });
    Promise.resolve().then(() => run(controller.signal)).then(result => finish(null, result), error => finish(error));
  });
}
