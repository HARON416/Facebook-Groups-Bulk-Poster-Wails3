import { untrack } from 'svelte';

type GeneratePoster = (file: File, profileID: string | undefined, signal: AbortSignal) => Promise<string>;

export function createVideoPoster(file: () => File | null, profileID: () => string | undefined, attempt: () => number, generate: GeneratePoster) {
  const state = $state({ image: '', busy: false, error: false });
  // Profile polling replaces the account object. Only a changed ID should
  // invalidate generation, not a new object for the same account.
  const accountID = $derived(profileID());
  $effect(() => {
    const selected = file(), id = accountID;
    attempt();
    return untrack(() => {
      state.image = ''; state.error = false; state.busy = false;
      if (!selected) return;
      const controller = new AbortController();
      state.busy = true;
      void generate(selected, id, controller.signal).then(image => {
        if (!controller.signal.aborted) state.image = image;
      }).catch(() => {
        if (!controller.signal.aborted) state.error = true;
      }).finally(() => {
        if (!controller.signal.aborted) state.busy = false;
      });
      return () => controller.abort();
    });
  });
  return state;
}
