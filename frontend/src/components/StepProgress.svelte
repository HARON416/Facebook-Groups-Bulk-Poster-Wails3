<script lang="ts">
  interface Step {
    id: string;
    title: string;
    description: string;
  }

  interface Props {
    steps: Step[];
    currentStep?: number;
    completed?: boolean;
    label?: string;
  }

  let { steps, currentStep = 0, completed = false, label }: Props = $props();
  const progressLabel = $derived(label ?? /* @wc-include */ 'Post progress');
</script>

<nav aria-label={progressLabel}>
  <ol>
    {#each steps as step, index (step.id)}
      <li
        class:completed={completed || index < currentStep}
        class:current={!completed && index === currentStep}
        aria-current={!completed && index === currentStep ? 'step' : undefined}
      >
        <span class="marker" aria-hidden="true">
          <svg viewBox="0 0 28 28" fill="none">
            <path d="M4 14l8 7L24 7" />
          </svg>
        </span>
        <span class="step-title">{step.title}</span>
        <span class="step-description">{step.description}</span>
        <span class="sr-only">
          Step {index + 1} of {steps.length}:
          {completed || index < currentStep ? 'Completed' : index === currentStep ? 'Current' : 'Upcoming'}
        </span>
      </li>
    {/each}
  </ol>
</nav>

<style>
  nav {
    --step-accent: var(--focus);
    --step-accent-soft: var(--accent-soft);
    --step-accent-faint: var(--accent-faint);
    --ease-out: cubic-bezier(.11, .29, .18, .98);
    --step-line: var(--border);
    padding: 1.6rem 0 2.4rem;
  }

  ol {
    display: flex;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    position: relative;
    display: flex;
    flex: 1;
    min-width: 0;
    flex-direction: column;
    align-items: center;
    text-align: center;
  }

  li::before,
  li::after {
    position: absolute;
    top: 1.05rem;
    width: calc(50% - 2.4rem);
    height: 0.3rem;
    background: var(--step-line);
    content: '';
  }

  li::before {
    left: 0;
  }

  li::after {
    right: 0;
  }

  li.completed::before,
  li.completed::after,
  li.current::before {
    background: var(--step-accent);
  }

  .marker {
    position: relative;
    display: flex;
    width: 2.4rem;
    height: 2.4rem;
    margin-bottom: 1.6rem;
    border: 0.1rem solid var(--step-accent);
    border-radius: 0.16rem;
    background: transparent;
    box-shadow: 0 0 0 0 var(--step-accent-faint);
    transition: box-shadow 0.4s var(--ease-out), background-color 0.4s;
  }

  .marker::after {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 0.4rem;
    height: 0.4rem;
    border-radius: 50%;
    background: var(--step-accent-soft);
    opacity: 0;
    pointer-events: none;
    content: '';
  }

  .current .marker {
    box-shadow: 0 0 0 0.4rem var(--step-accent-soft);
  }

  .completed .marker {
    background: var(--step-accent);
  }

  .completed svg {
    stroke: var(--on-accent);
  }

  .completed .marker::after {
    animation: checkbox-ripple 1.5s var(--ease-out);
  }

  svg {
    width: 1.6rem;
    height: 1.6rem;
    margin: auto;
    stroke: var(--step-accent);
    stroke-width: 3;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-miterlimit: 10;
  }

  svg path {
    stroke-dasharray: 30, 31;
    stroke-dashoffset: 31;
    transition: stroke-dashoffset 0.2s ease-in;
  }

  .completed svg path {
    stroke-dashoffset: 0;
    transition: stroke-dashoffset 0.4s var(--ease-out);
  }

  @keyframes checkbox-ripple {
    from {
      transform: translate(-50%, -50%) scale(0);
      opacity: 1;
    }
    to {
      transform: translate(-50%, -50%) scale(20);
      opacity: 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .marker, svg path {
      transition: none;
    }

    .completed .marker::after {
      animation: none;
    }
  }

  .step-title {
    width: 100%;
    padding: 0 0.8rem;
    color: var(--text-strong);
    font-size: 1.5rem;
    font-weight: 650;
    line-height: 1.5;
  }

  .step-description {
    width: 100%;
    margin-top: 0.5rem;
    padding: 0 0.8rem;
    color: var(--text-muted);
    font-size: 1.3rem;
    line-height: 1.5;
  }

  .sr-only {
    position: absolute;
    width: 0.1rem;
    height: 0.1rem;
    padding: 0;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }

  @media (max-width: 540px) {
    nav {
      padding: 1.6rem 0 2.4rem;
    }

    .step-title {
      font-size: 1.3rem;
    }

    .step-description {
      font-size: 1.1rem;
    }
  }
</style>
