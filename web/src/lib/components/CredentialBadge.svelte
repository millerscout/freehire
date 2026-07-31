<script lang="ts">
  import { credentialBadges } from '$lib/credentials';
  import { cn } from '$lib/ui';

  // Renders an employer's register-backed credentials — currently the UK and Dutch
  // visa-sponsor licences.
  //
  // Deliberately quiet: a licence is a fact about the employer, not a feature of
  // the role, so it reads as metadata alongside the other chips rather than as a
  // selling point. The disclaimer rides in the tooltip on every badge, because the
  // failure mode is a candidate reading "licensed sponsor" as "they will sponsor
  // me" and spending an application on it.
  let { collections, class: className }: { collections?: string[] | null; class?: string } =
    $props();

  const badges = $derived(credentialBadges(collections));
</script>

{#each badges as badge (badge.slug)}
  <span
    title={badge.tooltip}
    class={cn(
      'inline-flex items-center gap-1.5 rounded-md border border-border px-2 py-0.5 text-xs font-medium text-muted-foreground',
      className,
    )}
  >
    {badge.label}
    <span class="opacity-70">{badge.issuer}</span>
  </span>
{/each}
