// Native close can precede Svelte destruction. Stop timers explicitly on both
// page exit and the retained auxiliary window's hide/show lifecycle.
export function pollWhileWindowOpen(callback: () => Promise<void>, interval: number) {
  let timer: ReturnType<typeof setInterval> | undefined;
  let disposed = false;
  let running = false;
  const poll = async () => {
    if (disposed || timer === undefined || running) return;
    running = true;
    try { await callback(); }
    finally { running = false; }
  };
  const pause = () => {
    clearInterval(timer);
    timer = undefined;
  };
  const resume = () => {
    if (disposed || timer !== undefined) return;
    timer = setInterval(() => { void poll(); }, interval);
    void poll();
  };
  window.addEventListener('pagehide', pause);
  window.addEventListener('beforeunload', pause);
  window.addEventListener('pageshow', resume);
  window.addEventListener('postika:window-hidden', pause);
  window.addEventListener('postika:window-shown', resume);
  resume();
  return () => {
    disposed = true;
    pause();
    window.removeEventListener('pagehide', pause);
    window.removeEventListener('beforeunload', pause);
    window.removeEventListener('pageshow', resume);
    window.removeEventListener('postika:window-hidden', pause);
    window.removeEventListener('postika:window-shown', resume);
  };
}
