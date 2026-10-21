<script module lang="ts">
  // Translate only the built-in name. IDs and saved/user-defined names stay stable.
  export function accountLabel(profile: { id: string; name: string }): string {
    return profile.id === 'default' && profile.name === /* @wc-ignore */ 'Default profile'
      ? /* @wc-include */ 'Default profile'
      : profile.name;
  }
</script>
