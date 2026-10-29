<script lang="ts">
  import { logMessage } from './components/LogMessage.svelte';
  import LocalizedFeedback from './components/LocalizedFeedback.svelte';
  import StepProgress from './components/StepProgress.svelte';
  import PostComposer, { type PostImage } from './components/PostComposer.svelte';
  import { onDestroy, onMount, tick, untrack } from 'svelte';
  import type { AccountProfile, FacebookPagePost } from '../bindings/postika';
  import { createAccountService } from './lib/accounts';
  import { Events } from '@wailsio/runtime';
  import { readDraft, writeDraft, type AccountDraft } from './lib/drafts';
  let { profile, active = true, connect = false, tourStep = null, tourActive = false, onTourReadyChange } = $props<{ profile: AccountProfile; active?: boolean; connect?: boolean; tourStep?: number | null; tourActive?: boolean; onTourReadyChange?: (ready: boolean) => void }>();
  const GroupsService = untrack(() => createAccountService(profile.id));
  const { PublishRichPostWithOptions: publish } = GroupsService;
  let draftReady = $state(false), draftError = $state(''), imageDropMessage = $state<string | string[]>('');
  import PostingOptionsForm from './components/PostingOptionsForm.svelte';
  import DateTimePicker from './components/DateTimePicker.svelte';
  import QueueReview from './components/QueueReview.svelte';
  import { confetti, clickOrigin } from './lib/confetti';
  import { pollWhileWindowOpen } from './lib/window-polling';

  const steps = [
    { id: 'create', title: 'Create post', description: 'Write your post and add images' },
    { id: 'groups', title: 'Select groups', description: 'Load groups and choose where to post' },
    { id: 'settings', title: 'Post or schedule', description: 'Choose the time between posts, then post now or schedule for later.' },
  ];

  let currentStep = $state(0);
  const displayStep = $derived(tourStep ?? currentStep);
  let minimumInterval = $state(60), maximumInterval = $state(120);
  const validSettings = $derived(Number.isInteger(minimumInterval) && minimumInterval >= 60 && Number.isInteger(maximumInterval) && maximumInterval >= minimumInterval && maximumInterval <= 86400);
  let body = $state('');
  let bodyHTML = $state(''), bodyState = $state('');
  let images = $state<PostImage[]>([]);
  let fetchedPosts = $state<FacebookPagePost[] | null>(null);
  let groupLoadMessage = $state('');
  let openingGroups = $state(false);
  let groupLoadFailed = $state(false);
  let groupsBrowserOpened = $state(false);
  let checkingLogin = false;
  let checkingPostStatus = false;
  let sortMenuOpened = false;
  let sortOptionSelected = false;
  let loadGeneration = 0;
  let groups = $state<{ id: string; name: string; url: string; picture: string }[]>([]);
  let selectedGroupIds = $state<string[]>([]);
  let openingPost = $state(false);
  let backendPostingActive = $state(false);
  let stopRequested = false;
  let scheduleOpen = $state(false);
  let scheduleTime = $state('');
  let scheduleValid = $state(false);
  let scheduledFor = $state('');
  function localDateTime(date: Date) {
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}T${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
  }
  function showSchedule() {
    if (!scheduleTime || new Date(scheduleTime).getTime() <= Date.now()) scheduleTime = localDateTime(new Date(Date.now() + 5 * 60000));
    scheduleOpen = true;
    void tick().then(() => document.getElementById('schedule-time')?.focus());
  }

  const postingActive = $derived(openingPost || backendPostingActive);
  let postMessage = $state('');
  let postFailed = $state(false);
  let postingFinished = $state(false);
  let postingCompleted = $state(false);
  let submittedDraft = $state('');
  const currentDraft = $derived(JSON.stringify([body, bodyState, images.map((image) => image.id), fetchedPosts, selectedGroupIds]));
  $effect(() => {
    if (!openingPost && postingFinished && currentDraft !== submittedDraft) {
      postingFinished = false;
      postingCompleted = false;
      postMessage = '';
      postFailed = false;
    }
  });
  let groupQuery = $state('');
  let searchHits = $state<{ id: string; parts: { text: string; matched: boolean }[] }[] | null>(null);
  let searchingGroups = $state(false);
  let searchError = $state('');
  let searchVersion = 0;
  let searchInput = $state<HTMLInputElement>();
  const visibleGroups = $derived.by(() => {
    if (searchHits === null) return groups.map((group) => ({ ...group, parts: [{ text: group.name, matched: false }] }));
    const byId = new Map(groups.map((group) => [group.id, group]));
    return searchHits.flatMap((hit) => { const group = byId.get(hit.id); return group ? [{ ...group, parts: hit.parts }] : []; });
  });
  const allGroupsSelected = $derived(groups.length > 0 && selectedGroupIds.length === groups.length);
  const reviewGroups = $derived.by(() => {
    const byId = new Map(groups.map(group => [group.id, group.name]));
    return selectedGroupIds.map(id => ({ id, name: byId.get(id) ?? id }));
  });

  $effect(() => {
    const query = groupQuery.trim();
    const currentGroups = groups;
    const version = ++searchVersion;
    searchError = '';
    if (!query || currentGroups.length === 0) {
      searchHits = null;
      searchingGroups = false;
      return;
    }
    searchHits = [];
    searchingGroups = true;
    const timer = window.setTimeout(async () => {
      try {
        const hits = await GroupsService.SearchGroups(query);
        if (version === searchVersion) searchHits = (hits ?? []).map((hit) => ({ id: hit.id, parts: hit.parts ?? [] }));
      } catch (error) {
        if (version === searchVersion) searchError = error instanceof Error ? error.message : String(error);
      } finally {
        if (version === searchVersion) searchingGroups = false;
      }
    }, 200);
    return () => { window.clearTimeout(timer); searchVersion++; };
  });

  function clearGroupSearch() {
    groupQuery = '';
    searchInput?.focus();
  }
  let groupsLoadState = $state('idle');
  let loginState = $state('checking');
  const loginLabels: Record<string, { title: string; detail: string }> = {
    session_captured: { title: 'Facebook session captured', detail: 'The browser is closed. It will reopen when you click Post.' },
    checking: { title: 'Checking Facebook login', detail: 'Finish signing in or any verification prompts in Chrome. Your login will be detected automatically.' },
    logged_out: { title: 'Login required', detail: 'Please log in to Facebook in the Chrome window. Postika will detect when you’re signed in.' },
    login_required: { title: 'Complete Facebook verification', detail: 'Complete the login or verification prompts in Chrome to continue.' },
    logged_in: { title: 'Facebook connected', detail: 'You’re logged in. Your session is ready.' },
    browser_closed: { title: 'Facebook window closed', detail: 'Click Load groups to reopen Facebook and check your login.' },
  };
  const loginInfo = $derived(loginLabels[loginState] ?? loginLabels.checking);
  let stepHeading = $state<HTMLHeadingElement>();
  const groupsBusy = $derived(openingGroups || groupsLoadState === 'loading' || (groupsLoadState === 'waiting' && loginState !== 'browser_closed'));
  const incompleteFetchedPost = $derived(fetchedPosts?.findIndex(post => !post.text.trim()) ?? -1);
  const hasCompletePost = $derived(fetchedPosts !== null
    ? fetchedPosts.length > 0 && incompleteFetchedPost === -1
    : body.trim().length > 0 && images.length > 0 && images.every(image => !image.unavailable));
  const canContinue = $derived(draftReady && (currentStep === 0
    ? hasCompletePost
    : currentStep === 1 && selectedGroupIds.length > 0));

  async function restoreGroups() {
    const generation = loadGeneration;
    try {
      const status = await GroupsService.GetGroupsLoadStatus();
      if (generation !== loadGeneration || openingGroups || groupsBrowserOpened) return;
      groups = status.groups ?? [];
      groupsLoadState = status.state;
      if (status.state === 'waiting' || status.state === 'loading') groupsBrowserOpened = true;
      groupLoadFailed = status.state === 'error';
      groupLoadMessage = status.message;
    } catch {
      if (generation === loadGeneration && !openingGroups && !groupsBrowserOpened) {
        groupLoadMessage = /* @wc-ignore */ 'Could not restore saved groups. Load groups to refresh the list.';
      }
    }
  }

  async function loadGroups() {
    if (postingActive || groupsBusy) return;
    loadGeneration++;
    openingGroups = true;
    groupLoadFailed = false;
    groupLoadMessage = '';
    groupsBrowserOpened = false;
    loginState = 'checking';
    sortMenuOpened = false;
    sortOptionSelected = false;
    groupQuery = '';
    groupsLoadState = 'idle';
    try {
      await GroupsService.OpenJoinedGroups();
      groupsBrowserOpened = true;
    } catch (error) {
      groupLoadFailed = true;
      groupLoadMessage = error instanceof Error ? error.message : String(error);
    } finally {
      openingGroups = false;
    }
    await checkLogin();
  }

  async function checkLogin() {
    if (!groupsBrowserOpened || openingGroups || checkingLogin || postingActive || groupsLoadState === 'complete' || groupsLoadState === 'error') return;
    checkingLogin = true;
    const generation = loadGeneration;
    try {
      const state = await GroupsService.GetLoginState();
      if (generation !== loadGeneration) return;
      loginState = state;
      if (!sortMenuOpened && (loginState === 'logged_in' || loginState === 'checking')) {
        try {
          const opened = await GroupsService.OpenGroupsSortMenu();
          if (generation !== loadGeneration) return;
          sortMenuOpened = opened;
        } catch (error) {
          groupLoadFailed = true;
          groupLoadMessage = error instanceof Error ? error.message : String(error);
        }
      }
      if (sortMenuOpened && !sortOptionSelected && (loginState === 'logged_in' || loginState === 'checking')) {
        try {
          const selected = await GroupsService.SelectUnselectedGroupsSortOption();
          if (generation !== loadGeneration) return;
          sortOptionSelected = selected;
        } catch (error) {
          groupLoadFailed = true;
          groupLoadMessage = error instanceof Error ? error.message : String(error);
        }
      }
      const status = await GroupsService.GetGroupsLoadStatus();
      if (generation !== loadGeneration) return;
      const nextGroups = status.groups ?? [];
      if ((nextGroups.length > 0 || status.state === 'complete') && (nextGroups.length !== groups.length || nextGroups.some((group, index) => group.id !== groups[index].id || group.name !== groups[index].name || group.picture !== groups[index].picture))) {
        groups = nextGroups;
      }
      if (status.state === 'complete') {
        const currentIds = new Set(groups.map((group) => group.id));
        selectedGroupIds = selectedGroupIds.filter((id) => currentIds.has(id));
      }
      groupsLoadState = status.state;
      if (status.state !== 'idle') {
        groupLoadFailed = status.state === 'error';
        groupLoadMessage = status.message;
      }
    } catch {
      if (generation === loadGeneration) loginState = 'checking';
    } finally {
      checkingLogin = false;
    }
  }

  async function openPost(event: Event, scheduled = false) {
    if (postingActive || groupsBusy || selectedGroupIds.length === 0 || !hasCompletePost || !validSettings) return;
    const celebrationOrigin = event instanceof MouseEvent ? clickOrigin(event) : null;
    let scheduledAt = '';
    if (scheduled) {
      const date = new Date(scheduleTime);
      if (!Number.isFinite(date.getTime()) || date.getTime() <= Date.now()) {
        postFailed = true;
        postMessage = /* @wc-ignore */ 'Choose a future date and time for your post.';
        return;
      }
      scheduledAt = date.toISOString();
    }
    openingPost = true;
    scheduledFor = scheduledAt;
    postingCompleted = false;
    stopRequested = false;
    postFailed = false;
    postMessage = scheduled ? /* @wc-ignore */ 'Saving your scheduled queue…' : /* @wc-ignore */ 'Opening a selected group to capture fresh posting requests…';
    try {
      await GroupsService.OpenPostingLogs();
      if (stopRequested) { postMessage = /* @wc-ignore */ 'Posting stopped.'; return; }
      const postsForRun = fetchedPosts?.map(post => ({id: post.id, text: post.text, url: post.url, images: [...post.images ?? []]})) ?? null;
      const uploads = await Promise.all((postsForRun === null ? images : []).map((image) => new Promise<{ name: string; data: string }>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => {
          if (typeof reader.result !== 'string') { reject(new Error(/* @wc-ignore */ 'Could not read a selected image')); return; }
          resolve({ name: image.file.name, data: reader.result.slice(reader.result.indexOf(',') + 1) });
        };
        reader.onerror = () => reject(new Error(/* @wc-ignore */ `Could not read ${image.file.name}`));
        reader.readAsDataURL(image.file);
      })));
      if (stopRequested) { postMessage = /* @wc-ignore */ 'Posting stopped.'; return; }
      scheduleOpen = false;
      const options = { minimumInterval, maximumInterval, rewrite: false, instructions: '' };
      const result = postsForRun !== null
        ? scheduled
          ? await GroupsService.ScheduleFacebookPostsWithOptions([...selectedGroupIds], postsForRun, options, scheduledAt)
          : await GroupsService.PublishFacebookPostsWithOptions([...selectedGroupIds], postsForRun, options)
        : scheduled
        ? await GroupsService.ScheduleRichPostWithOptions([...selectedGroupIds], body, bodyHTML, uploads, options, scheduledAt)
        : await publish([...selectedGroupIds], body, bodyHTML, uploads, options);
      postMessage = result?.message ?? /* @wc-ignore */ 'Posting finished.';
      postFailed = result?.state === 'stopped';
      postingCompleted = result?.state === 'complete';
      postingFinished = postingCompleted || (result?.results?.length ?? 0) > 0;
      submittedDraft = currentDraft;
      if (active && celebrationOrigin && result?.state === 'complete' && result.results?.length && result.results.every(entry => entry.state === 'posted')) void confetti(celebrationOrigin);
    } catch (error) {
      postFailed = true;
      postMessage = error instanceof Error ? error.message : String(error);
    } finally {
      openingPost = false;
      backendPostingActive = false;
      scheduledFor = '';
    }
  }

  async function stopPosting() {
    stopRequested = true;
    await GroupsService.CancelPosting();
  }

  async function checkPostStatus() {
    if (checkingPostStatus) return;
    checkingPostStatus = true;
    try {
      const result = await GroupsService.GetPostingStatus();
      backendPostingActive = result.active;
      if (stopRequested && result.active && result.state !== 'stopping') await GroupsService.CancelPosting();
      if (result.active) {
        scheduledFor = result.state === 'scheduled' ? result.scheduledAt : '';
        postMessage = result.message;
      }
    } catch { /* The main posting call reports browser and connection errors. */ }
    finally { checkingPostStatus = false; }
  }

  onMount(() => {
    let disposed = false;
    async function initialize() {
      try {
        const draft = await readDraft(profile.id);
        if (disposed) return;
        if (draft) {
          body = draft.body; bodyHTML = draft.bodyHTML; bodyState = draft.bodyState;
          fetchedPosts = draft.fetchedPosts ?? null;
          images = draft.images.map((image) => ({ ...image, url: image.unavailable ? '' : URL.createObjectURL(image.file) }));
          const missing = images.filter(image => image.unavailable);
          const missingNames = missing.map(image => image.file.name).join(', ');
          if (missing.length) imageDropMessage = /* @wc-ignore */ `Could not read ${missingNames}`;
          selectedGroupIds = draft.selectedGroupIds; currentStep = missing.length ? 0 : draft.currentStep;
          minimumInterval = draft.minimumInterval; maximumInterval = draft.maximumInterval;
          scheduleTime = draft.scheduleTime;
        }
      } catch { draftError = /* @wc-ignore */ 'Could not restore this account’s draft. New edits will stay available while Postika is open.'; }
      if (disposed) return;
      await restoreGroups();
      draftReady = true;
      if (connect) { currentStep = 1; await loadGroups(); }
    }
    void initialize();
    const stopImageDrops = Events.On('postika:images-dropped', (event) => {
      if (tourActive) return;
      const result = event.data as { target: string; profileID?: string; images: { name: string; type: string; data: string; lastModified: number }[]; errors: string[] };
      if (result.target !== 'post-images' || (result.profileID ? result.profileID !== profile.id : !active)) return;
      try {
        for (const image of result.images) {
          const bytes = Uint8Array.from(atob(image.data), (char) => char.charCodeAt(0));
          const file = new File([bytes], image.name, { type: image.type, lastModified: image.lastModified });
          if (images.some((entry) => !entry.unavailable && entry.file.name === file.name && entry.file.size === file.size && entry.file.lastModified === file.lastModified)) continue;
          const replacement = images.findIndex(image => image.unavailable && image.file.name === file.name);
          const addedImage = { id: replacement < 0 ? crypto.randomUUID() : images[replacement].id, file, url: URL.createObjectURL(file) };
          images = replacement < 0 ? [...images, addedImage] : images.map((entry, i) => i === replacement ? addedImage : entry);
        }
        imageDropMessage = result.errors;
      } catch { imageDropMessage = /* @wc-ignore */ 'Could not add the dropped images. Try browsing for them instead.'; }
    });
    const stopLoginPolling = pollWhileWindowOpen(checkLogin, 1500);
    const stopPostingPolling = pollWhileWindowOpen(checkPostStatus, 500);
    return () => { disposed = true; stopImageDrops(); stopLoginPolling(); stopPostingPolling(); };
  });

  $effect(() => {
    if (active) onTourReadyChange?.(draftReady && !groupsBusy && !postingActive && !openingPost);
  });

  async function navigate(step: number) {
    if (postingActive || (groupsBusy && currentStep === 1 && step > currentStep)) return;
    currentStep = step;
    await tick();
    stepHeading?.focus();
  }

  // Previewing the composer can normalize Lexical markup on mount. Preserve
  // the original rich-text state and suppress draft writes throughout the tour.
  let beforeTour: { body: string; bodyHTML: string; bodyState: string } | undefined;
  $effect.pre(() => {
    if (tourActive) {
      beforeTour ??= untrack(() => ({ body, bodyHTML, bodyState }));
    } else if (beforeTour) {
      const saved = beforeTour;
      beforeTour = undefined;
      body = saved.body; bodyHTML = saved.bodyHTML; bodyState = saved.bodyState;
    }
  });

  $effect(() => {
    if (!draftReady || tourActive) return;
    // Read reactive fields now; draft storage copies attachment bytes to IndexedDB.
    const draft: AccountDraft = {
      body, bodyHTML, bodyState, images: images.map(({ id, file, unavailable, recoveryKey }) => ({ id, file, unavailable, recoveryKey })),
      fetchedPosts: fetchedPosts?.map(({ id, text, url, images }) => ({ id, text, url, images: [...images ?? []] })) ?? null,
      selectedGroupIds: [...selectedGroupIds], currentStep, minimumInterval, maximumInterval,
      scheduleTime,
    };
    // Queue each edit immediately so a quick switch or close cannot lose a debounce.
    void writeDraft(profile.id, draft).then(() => { draftError = ''; }).catch(() => {
      draftError = /* @wc-ignore */ 'Could not save this account’s draft to disk. Keep Postika open to retain your edits.';
    });
  });

  onDestroy(() => images.forEach((image) => URL.revokeObjectURL(image.url)));
</script>

{#if active}
<div class="app-shell">
  <header>
    <StepProgress {steps} currentStep={displayStep} completed={postingCompleted} />
  </header>
  <main aria-labelledby="step-heading" data-step-scroll>
    {#if draftError}<p class="group-status error" role="alert">{logMessage(draftError)}</p>{/if}
    <div class="step-content">
      <div class="step-heading">
        <p class="eyebrow">STEP {displayStep + 1} OF {steps.length}</p>
        <h1 class="ui-title" id="step-heading" bind:this={stepHeading} tabindex="-1">{steps[displayStep].title}</h1>
        <p class="step-intro">
          {displayStep === 0 ? fetchedPosts === null ? 'Start with your message. Add images to bring it to life.' : 'Review the posts fetched from your Facebook page.' : displayStep === 1 ? 'Choose the groups you want to share your post with.' : 'Choose the time between posts, then post now or schedule for later.'}
        </p>
      </div>

      {#if !draftReady}
        <p class="group-status" role="status">Restoring account workspace…</p>
      {:else if displayStep === 0}
        <PostComposer profileID={profile.id} groupsLoading={groupsBusy} bind:message={imageDropMessage} bind:body bind:bodyHTML bind:bodyState bind:images bind:fetchedPosts />
      {:else if displayStep === 1}
        <div class="groups-toolbar" data-tour="groups">
          <h2>Groups</h2>
          <button type="button" class="load-groups" disabled={postingActive || groupsBusy} aria-busy={groupsBusy} onclick={loadGroups}>
            {openingGroups ? 'Opening…' : groupsBusy ? 'Loading…' : 'Load groups'}
          </button>
        </div>
        {#if groupsBrowserOpened}
          <div class="login-status" class:connected={loginState === 'logged_in' || loginState === 'session_captured'} role="status" aria-live="polite" aria-atomic="true">
            <div>
              <h3>{loginInfo.title}</h3>
              <p>{loginInfo.detail}</p>
            </div>
          </div>
        {/if}
        {#if groups.length > 0}
          <div class="group-status" class:error={groupLoadFailed} role="status"><LocalizedFeedback message={groupLoadMessage} technical={groupLoadFailed} /></div>
          <div class="group-search">
            <label class="visuallyhidden" for="group-search">Search groups</label>
            <input id="group-search" disabled={groupsBusy} type="text" placeholder="Search groups…" autocomplete="off" bind:value={groupQuery} bind:this={searchInput} onkeydown={(event) => { if (event.key === 'Escape') clearGroupSearch(); }} />
            {#if groupQuery}
              <button class="clear-search" disabled={groupsBusy} type="button" aria-label="Clear group search" onclick={clearGroupSearch}>
                <svg xmlns="http://www.w3.org/2000/svg" fill="currentColor" viewBox="0 0 256 256" aria-hidden="true"><path d="M205.66,194.34a8,8,0,0,1-11.32,11.32L128,139.31,61.66,205.66a8,8,0,0,1-11.32-11.32L116.69,128,50.34,61.66A8,8,0,0,1,61.66,50.34L128,116.69l66.34-66.35a8,8,0,0,1,11.32,11.32L139.31,128Z" /></svg>
              </button>
            {/if}
          </div>
          <div class="group-selection-tools">
            <p class="selection-count" aria-live="polite"><span>{selectedGroupIds.length} selected</span>{#if groupQuery.trim()} · <span>{visibleGroups.length} shown</span>{/if}</p>
            <div class="bulk-actions">
              <button type="button" disabled={groupsBusy || allGroupsSelected} onclick={() => { selectedGroupIds = groups.map((group) => group.id); }}>Select all groups</button>
              <button type="button" disabled={groupsBusy || selectedGroupIds.length === 0} onclick={() => { selectedGroupIds = []; }}>Unselect all groups</button>
            </div>
          </div>
          {#if searchingGroups}<p class="search-message" role="status">Searching…</p>
          {:else if searchError}<div class="search-message error" role="status"><LocalizedFeedback message={searchError} /></div>
          {:else if groupQuery.trim() && visibleGroups.length === 0}<p class="search-message" role="status">No groups found. Try another search.</p>{/if}
          <ul class="groups-list" aria-label="Joined groups" aria-busy={searchingGroups || groupsLoadState === 'loading'}>
            {#each visibleGroups as group (group.id)}
              <li class="checkbox-wrapper-33" class:selected={selectedGroupIds.includes(group.id)}>
                <label class="checkbox">
                  <input class="checkbox__trigger visuallyhidden" type="checkbox" disabled={groupsBusy} value={group.id} bind:group={selectedGroupIds} />
                  <span class="checkbox__symbol">
                    <svg aria-hidden="true" class="icon-checkbox" viewBox="0 0 28 28" fill="none"><path d="M4 14l8 7L24 7" /></svg>
                  </span>
                  {#if group.picture}<img src={group.picture} alt="" loading="lazy" referrerpolicy="no-referrer" />{/if}
                  <p class="checkbox__textwrapper">{#each group.parts as part}{#if part.matched}<mark class="search-match">{part.text}</mark>{:else}{part.text}{/if}{/each}</p>
                </label>
              </li>
            {/each}
          </ul>
        {:else}
        <div class="empty-step">
          <h2>No groups loaded yet.</h2>
          <p>Open your Facebook groups to get started.</p>
          <div class="group-status" class:error={groupLoadFailed} role="status"><LocalizedFeedback message={groupLoadMessage} technical={groupLoadFailed} /></div>
        </div>
        {/if}
      {:else}
        <QueueReview {profile} {body} imageCount={images.length} {fetchedPosts} groups={reviewGroups} disabled={postingActive} oneditpost={() => navigate(0)} oneditgroups={() => navigate(1)} />
        <section class="run-settings" data-tour="timing" aria-label="Posting settings">
          <PostingOptionsForm bind:minimumInterval bind:maximumInterval disabled={postingActive} />
        </section>
        {#if scheduleOpen && !postingActive}
          <form id="schedule-form" class="schedule-panel" onsubmit={(event) => { event.preventDefault(); void openPost(event, true); }}>
            <h2>Schedule your queue</h2>
            <DateTimePicker bind:value={scheduleTime} bind:valid={scheduleValid} />
            <p>Time zone: {Intl.DateTimeFormat().resolvedOptions().timeZone}. Keep Postika open and your computer awake. Facebook may ask you to sign in when posting starts.</p>
          </form>
        {/if}
        {#if postMessage}<div class="group-status post-feedback" class:error={postFailed} role="status"><LocalizedFeedback message={postMessage} technical={postFailed} /></div>{/if}
      {/if}

    </div>
  </main>

  <footer class="step-actions" data-tour="schedule">
        {#if displayStep === 0 && draftReady && incompleteFetchedPost !== -1}
        <p id="next-blocker" class="next-blocker" role="status"><LocalizedFeedback message={`Add text to fetched post ${incompleteFetchedPost + 1} before posting`} /></p>
        {:else if displayStep < steps.length - 1 && postingActive && postMessage}
        <p id="next-blocker" class="next-blocker" role="status"><LocalizedFeedback message={postMessage} /></p>
        {/if}
        {#if displayStep === steps.length - 1 && scheduleOpen && !postingActive}
        <button type="button" class="previous action-button action-button--secondary" onclick={() => { scheduleOpen = false; }}>Cancel</button>
        <button type="submit" form="schedule-form" class="post action-button action-button--primary" disabled={groupsBusy || !scheduleValid || selectedGroupIds.length === 0 || !hasCompletePost || !validSettings}>Confirm schedule</button>
        {:else}
        <button type="button" class="previous action-button action-button--secondary" disabled={displayStep === 0 || postingActive} onclick={() => navigate(displayStep - 1)}>
          <svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m14 6-6 6 6 6" /></svg>
          Previous
        </button>
        {#if displayStep < steps.length - 1}
        <button type="button" class="next action-button action-button--primary" aria-describedby={displayStep === 0 && incompleteFetchedPost !== -1 || postingActive && postMessage ? 'next-blocker' : undefined} disabled={!canContinue || postingActive || (groupsBusy && displayStep === 1)} onclick={() => navigate(displayStep + 1)}>
          Next
          <svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m10 6 6 6-6 6" /></svg>
        </button>
        {:else}
        {#if postMessage}<button type="button" class="previous action-button action-button--secondary" onclick={() => GroupsService.OpenPostingLogs()}>View logs</button>{/if}
        {#if postingActive}<button type="button" class="previous action-button action-button--secondary" onclick={stopPosting}>Stop</button>{/if}
        <button type="button" class="post action-button action-button--primary" disabled={postingActive || groupsBusy || selectedGroupIds.length === 0 || !hasCompletePost || !validSettings} onclick={openPost} aria-busy={postingActive}>
          {postingActive ? scheduledFor ? 'Scheduled' : 'Posting…' : 'Post now'}
        </button>
        <button type="button" class="previous action-button action-button--secondary" disabled={postingActive || groupsBusy || selectedGroupIds.length === 0 || !hasCompletePost || !validSettings} onclick={showSchedule}>Schedule</button>
        {/if}
        {/if}
  </footer>
</div>
{/if}

<style>
  .schedule-panel { margin-top: 2.4rem; padding: 2rem 2.4rem; border: 1px solid var(--border-accent); border-radius: 1rem; background: var(--surface-active); }
  .schedule-panel h2 { font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); color: var(--text-strong); margin-bottom: 1.6rem; }
  .schedule-panel p { margin-top: 1.2rem; font-size: 1.3rem; line-height: 1.6; color: var(--text-secondary); }

  .run-settings { margin: 2rem 0; }
  .app-shell {
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100%;
    padding: 1.5rem;
    overflow: hidden;
    overflow: clip;
  }

  header {
    flex-shrink: 0;
  }

  main {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior-y: contain;
    padding-top: 3.2rem;
    padding-bottom: 3.2rem;
  }

  .step-content {
    max-width: 72rem;
    margin: 0 auto;
  }

  .step-heading { margin-bottom: 3rem; }
  .eyebrow { margin-bottom: 0.8rem; color: var(--accent); font-size: var(--ui-small-size); font-weight: var(--ui-label-weight); letter-spacing: normal; }
  h1:focus { outline: none; }
  .step-intro { margin-top: 0.9rem; color: var(--text-muted); font-size: 1.3rem; line-height: 1.6; }
  .step-actions { display: flex; flex: 0 0 auto; min-height: 8rem; flex-wrap: wrap; padding-block: 1rem; align-items: center; justify-content: flex-end; width: 100%; max-width: 72rem; gap: 1rem; margin: 0 auto; border-top: 0.1rem solid var(--border); }
  .next-blocker { flex: 1 1 25rem; margin: 0; color: var(--text); font-size: var(--ui-small-size); line-height: 1.5; overflow-wrap: anywhere; }
  .step-actions svg { width: 1.5rem; height: 1.5rem; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
  :global(html[dir="rtl"]) .step-actions svg { transform: scaleX(-1); }
  .empty-step { padding: 3.2rem 0; }
  .empty-step h2 { margin-bottom: 1rem; font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); }
  .empty-step p { color: var(--text-muted); font-size: 1.3rem; line-height: 1.7; }
  .groups-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 1.6rem; padding-bottom: 1.6rem; border-bottom: 0.1rem solid var(--border); }
  .groups-toolbar h2 { color: var(--text-strong); font-size: var(--ui-title-size); font-weight: var(--ui-title-weight); }
  .load-groups { display: flex; align-items: center; justify-content: center; gap: 0.8rem; min-height: 4rem; padding: 0.9rem 1.6rem; border: 0.1rem solid var(--accent); border-radius: 0.7rem; background: var(--accent); color: var(--on-accent); font-size: 1.3rem; font-weight: 550; cursor: pointer; }
  .load-groups:hover { background: var(--accent-hover); }
  .load-groups:disabled { opacity: 0.6; cursor: wait; }
  .load-groups:focus-visible { outline: 0.2rem solid var(--accent); outline-offset: 0.3rem; }
  .group-status:not(:empty) { margin-top: 1.2rem; }
  .empty-step .group-status.error { color: var(--danger); }
  .group-status { color: var(--text-secondary); font-size: 1.3rem; line-height: 1.7; }
  .group-status.error { color: var(--danger); }
  .group-status.error { color: var(--danger); }
  .groups-list { display: grid; gap: 1rem; margin-top: 1.6rem; list-style: none; }
  .selection-count { margin-top: 0.6rem; color: var(--text-secondary); font-size: 1.3rem; }
  .group-search { position: relative; margin-top: 1.6rem; }
  .group-search input { width: 100%; min-height: 4.8rem; padding-block: 1.2rem; padding-inline: 1.4rem 4.8rem; border: 0.2rem solid var(--border-strong); border-radius: 0.8rem; background: var(--surface); color: var(--text); font-size: 1.5rem; }
  .group-search input::placeholder { color: var(--text-muted); }
  .group-search input:focus { outline: none; border-color: var(--accent); box-shadow: 0 0 0 0.3rem var(--accent-faint); }
  .clear-search { position: absolute; top: 50%; inset-inline-end: 0.6rem; display: grid; place-items: center; width: 3.6rem; height: 3.6rem; transform: translateY(-50%); border: 0; border-radius: 0.5rem; background: transparent; color: var(--text-secondary); cursor: pointer; }
  .clear-search:hover { background: var(--surface-hover); }
  .clear-search svg { width: 2rem; height: 2rem; }
  .clear-search:focus-visible, .bulk-actions button:focus-visible { outline: 0.2rem solid var(--accent); outline-offset: 0.2rem; }
  .group-selection-tools { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem; margin-top: 1.2rem; }
  .group-selection-tools .selection-count { margin: 0; }
  .bulk-actions { display: flex; flex-wrap: wrap; gap: 0.8rem; }
  .bulk-actions button { min-height: 3.6rem; padding: 0.6rem 1rem; border: 0.1rem solid var(--border); border-radius: 0.6rem; background: var(--surface); color: var(--accent); font-size: 1.3rem; font-weight: 550; cursor: pointer; }
  .bulk-actions button:hover:not(:disabled) { border-color: var(--accent); background: var(--surface-active); }
  .bulk-actions button:disabled { opacity: 0.45; cursor: not-allowed; }
  .search-message { margin-top: 1.6rem; color: var(--text-secondary); font-size: 1.3rem; }
  .search-message.error { color: var(--danger); }
  .groups-list li { border: 0.1rem solid var(--border); border-radius: 0.8rem; background: var(--surface); color: var(--text); font-size: 1.4rem; line-height: 1.5; transition: border-color var(--motion-state) var(--motion-ease), background-color var(--motion-state) var(--motion-ease); }
  .groups-list li.selected { border-color: var(--accent); background: var(--surface-active); }
  .checkbox-wrapper-33 { --c-primary: var(--accent); --c-primary-20-percent-opacity: var(--accent-soft); --c-primary-10-percent-opacity: var(--accent-faint); --t-base: var(--motion-state); --t-fast: var(--motion-fast); --e-in: var(--motion-ease); --e-out: var(--motion-ease); }
  .visuallyhidden { position: absolute; width: 0.1rem; height: 0.1rem; padding: 0; margin: -0.1rem; border: 0; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .checkbox { position: relative; display: flex; align-items: center; gap: 1.2rem; min-height: 6.4rem; padding: 1rem 1.2rem; cursor: pointer; }
  .checkbox__symbol { position: relative; display: flex; flex-shrink: 0; width: 2.4rem; height: 2.4rem; border: 0.15rem solid var(--c-primary); border-radius: 0.3rem; box-shadow: 0 0 0 0 var(--c-primary-10-percent-opacity); transition: box-shadow var(--t-base) var(--e-out), background-color var(--t-base); }
  .checkbox__symbol::after { content: ''; position: absolute; top: 50%; left: 50%; width: 0.4rem; height: 0.4rem; margin: -0.2rem; border-radius: 50%; background: var(--c-primary-20-percent-opacity); opacity: 0; pointer-events: none; }
  .icon-checkbox { width: 1.8rem; height: 1.8rem; margin: auto; color: var(--on-accent); stroke: currentColor; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
  .icon-checkbox path { stroke-dasharray: 30 31; stroke-dashoffset: 31; transition: stroke-dashoffset var(--t-fast) var(--e-in); }
  .checkbox__trigger:checked + .checkbox__symbol { background: var(--c-primary); }
  .checkbox__trigger:checked + .checkbox__symbol::after { animation: ripple-33 var(--motion-state) var(--motion-ease); }
  .checkbox__trigger:checked + .checkbox__symbol .icon-checkbox path { stroke-dashoffset: 0; transition: stroke-dashoffset var(--t-base) var(--e-out); }
  .checkbox__trigger:focus-visible + .checkbox__symbol { box-shadow: 0 0 0 0.4rem var(--c-primary-20-percent-opacity); }
  .search-match { color: var(--accent); background: transparent; font-weight: 600; }
  .checkbox__textwrapper { flex: 1; min-width: 0; margin: 0; overflow-wrap: anywhere; }
  @keyframes ripple-33 { from { transform: scale(0); opacity: 1; } to { transform: scale(20); opacity: 0; } }
  @media (prefers-reduced-motion: reduce) { .groups-list li, .checkbox__symbol, .icon-checkbox path, .checkbox__trigger:checked + .checkbox__symbol .icon-checkbox path { transition: none; } .checkbox__trigger:checked + .checkbox__symbol::after { animation: none; } }
  .groups-list img { width: 4.4rem; height: 4.4rem; flex-shrink: 0; border-radius: 0.6rem; object-fit: cover; }
  .login-status { display: flex; align-items: flex-start; gap: 1.2rem; margin-top: 1.6rem; padding: 1.6rem; border: 0.1rem solid var(--border); border-radius: 0.8rem; background: var(--surface); }
  .login-status.connected { border-color: var(--accent); background: var(--surface-active); }
  .login-status h3 { color: var(--text); font-size: 1.4rem; font-weight: 600; }
  .login-status p { margin-top: 0.5rem; color: var(--text-secondary); font-size: 1.3rem; line-height: 1.6; }
  @media (max-width: 540px) { .groups-toolbar { flex-wrap: wrap; } }
</style>
