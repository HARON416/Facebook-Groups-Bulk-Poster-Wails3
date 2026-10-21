import confettiUrl from '../../confetti.lottie?url';

export interface ConfettiOrigin { x: number; y: number }

// Keyboard activation has no pointer coordinates; use the button's center.
export function clickOrigin(event: MouseEvent): ConfettiOrigin {
  if (event.detail > 0) return { x: event.clientX, y: event.clientY };
  return elementOrigin(event.currentTarget as HTMLElement);
}

export function elementOrigin(element: HTMLElement): ConfettiOrigin {
  const bounds = element.getBoundingClientRect();
  return { x: bounds.left + bounds.width / 2, y: bounds.top + bounds.height / 2 };
}

interface ConfettiAnimation { w: number; h: number; fr: number; ip: number; op: number }
let player: Promise<{ lottie: typeof import('lottie-web')['default']; animation: ConfettiAnimation }> | undefined;
const active = new Set<() => void>();
const reducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

function loadPlayer() {
  if (!player) {
    player = Promise.all([import('lottie-web/build/player/lottie_light'), import('fflate'), fetch(confettiUrl).then(async response => {
      if (!response.ok) throw new Error('Confetti asset unavailable');
      return new Uint8Array(await response.arrayBuffer());
    })]).then(([module, { unzipSync, strFromU8 }, bytes]) => {
      const archive = unzipSync(bytes);
      const manifest = JSON.parse(strFromU8(archive['manifest.json']));
      const animation: ConfettiAnimation = JSON.parse(strFromU8(archive[`animations/${manifest.animations[0].id}.json`]));
      if (!(animation.w > 0 && animation.h > 0 && animation.fr > 0 && animation.op > animation.ip)) throw new Error('Invalid confetti dimensions or timing');
      return { lottie: module.default, animation };
    }).catch(error => { player = undefined; throw error; });
  }
  return player;
}

export async function confetti(origin: ConfettiOrigin) {
  if (reducedMotion()) return;
  let host: HTMLDivElement | undefined;
  try {
    const { lottie, animation } = await loadPlayer();
    if (reducedMotion() || document.hidden) return;
    // Keep rapid clicks bounded; overlays never intercept clicks or alter layout.
    if (active.size >= 3) active.values().next().value?.();
    host = document.createElement('div');
    host.className = 'click-confetti';
    host.setAttribute('aria-hidden', 'true');
    const scale = Math.min(520 / Math.max(animation.w, animation.h), window.innerWidth / animation.w);
    Object.assign(host.style, { position: 'fixed', left: `${origin.x}px`, top: `${origin.y}px`, width: `${animation.w * scale}px`, height: `${animation.h * scale}px`, transform: 'translate(-50%, -50%)', pointerEvents: 'none', zIndex: '9999' });
    document.body.append(host);
    const item = lottie.loadAnimation({ container: host, renderer: 'svg', loop: false, autoplay: false, animationData: structuredClone(animation), rendererSettings: { progressiveLoad: true, focusable: false } });
    let timer: ReturnType<typeof setTimeout>;
    const cleanup = () => { clearTimeout(timer); active.delete(cleanup); item.destroy(); host?.remove(); };
    timer = setTimeout(cleanup, Math.max(5000, (animation.op - animation.ip) / animation.fr * 1000 + 1000));
    active.add(cleanup);
    item.addEventListener('complete', cleanup);
    item.addEventListener('data_failed', cleanup);
    let started = false;
    const start = () => {
      if (started || !active.has(cleanup)) return;
      started = true;
      try {
        // Measure the opening particles: an exported canvas may have padding
        // or an emitter away from its center. Keep that burst at the click.
        const svg = host?.querySelector('svg');
        const bounds = svg?.getBBox();
        if (bounds && bounds.width > 0 && bounds.height > 0 && host) {
          host.style.transform = `translate(${-((bounds.x + bounds.width / 2) * scale)}px, ${-((bounds.y + bounds.height / 2) * scale)}px)`;
        }
      } catch { /* Fall back to the canvas center if measurement is unavailable. */ }
      item.play();
    };
    item.addEventListener('DOMLoaded', start);
    if (item.isLoaded) start();
  } catch {
    host?.remove();
    // Decorative feedback must never interrupt the user's action.
  }
}

window.addEventListener('pagehide', () => { for (const cleanup of active) cleanup(); });
