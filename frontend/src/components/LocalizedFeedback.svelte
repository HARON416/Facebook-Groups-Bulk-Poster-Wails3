<script lang="ts">
  import { feedbackMessage, logMessage } from './LogMessage.svelte';
  import { hasLogDetails } from '../lib/log-message';
  let { message, technical = true }: { message: string; technical?: boolean } = $props();
</script>

{technical ? feedbackMessage(message) : logMessage(message)}{#if hasLogDetails(message)}<details><summary>Debug details</summary><p dir="auto">{message}</p></details>{/if}
