<script lang="ts">
  import { profileStore } from '$lib/profile.svelte';
  import { loadSkillDistribution } from '$lib/skillDictionary';
  import type { FacetOption } from '$lib/facets';
  import RemoteSearchSelect from './facets/RemoteSearchSelect.svelte';

  // Skills and Skills to avoid, autosaving on every toggle (no Save control here) — the
  // same store mutators JobMatch.svelte already uses for its "I have it" / "Avoid" chips.
  // No local buffer: both lists read straight off the profile, so a write's effect on the
  // chips comes purely from the store's own reactive state changing.
  const skills = $derived(profileStore.profile?.skills ?? []);
  const excludedSkills = $derived(profileStore.profile?.excluded_skills ?? []);

  // The universe of skills (canonical tokens with job counts) for the typeahead, from the
  // same source ProfileForm's set-up form and the job-search filter panel use.
  let skillDist = $state.raw<FacetOption[]>([]);
  $effect(() => {
    void loadSkillDistribution().then((dist) => (skillDist = dist));
  });

  function searchSkillsExcept(query: string, avoid: string[]): Promise<FacetOption[]> {
    const q = query.trim().toLowerCase();
    const pool = skillDist.filter((o) => !avoid.includes(o.value));
    const matches = q ? pool.filter((o) => o.label.toLowerCase().includes(q)) : pool;
    return Promise.resolve(matches.slice(0, q ? 50 : 8));
  }

  const searchSkills = (query: string) => searchSkillsExcept(query, excludedSkills);
  const searchExcludedSkills = (query: string) => searchSkillsExcept(query, skills);

  // One write in flight at a time (mirrors JobMatch.svelte): a second toggle started before
  // the first settles would race against a profile row it hasn't seen yet.
  let pending = $state(false);
  let failed = $state<string | null>(null);

  async function toggleSkill(skill: string) {
    if (pending) return;
    pending = true;
    failed = null;
    try {
      await (skills.includes(skill) ? profileStore.removeSkill(skill) : profileStore.addSkill(skill));
    } catch {
      failed = skill;
    } finally {
      pending = false;
    }
  }

  async function toggleExcludedSkill(skill: string) {
    if (pending) return;
    pending = true;
    failed = null;
    try {
      await (excludedSkills.includes(skill)
        ? profileStore.unavoidSkill(skill)
        : profileStore.avoidSkill(skill));
    } catch {
      failed = skill;
    } finally {
      pending = false;
    }
  }
</script>

<div class="flex flex-col gap-6 rounded-xl border border-border bg-card p-5 sm:p-6">
  <div class="flex flex-col gap-2 {pending ? 'pointer-events-none opacity-60' : ''}">
    <div class="flex items-baseline justify-between">
      <span class="text-sm font-medium">Skills</span>
      <span class="text-xs tabular-nums text-muted-foreground">{skills.length}</span>
    </div>
    <RemoteSearchSelect
      search={searchSkills}
      include={skills}
      placeholder="Search skills"
      onToggle={toggleSkill}
      fallbackLabel={(v) => v}
      clearOnSelect
    />
  </div>

  <div class="flex flex-col gap-2 {pending ? 'pointer-events-none opacity-60' : ''}">
    <div class="flex items-baseline justify-between">
      <span class="text-sm font-medium">Skills to avoid</span>
      <span class="text-xs tabular-nums text-muted-foreground">{excludedSkills.length}</span>
    </div>
    <RemoteSearchSelect
      search={searchExcludedSkills}
      include={[]}
      exclude={excludedSkills}
      placeholder="Search skills to exclude"
      onToggle={toggleExcludedSkill}
      fallbackLabel={(v) => v}
      clearOnSelect
    />
    <span class="text-xs text-muted-foreground">
      Filtered out when you apply your profile to the job filters.
    </span>
  </div>

  {#if failed}
    <p class="text-sm text-destructive">Could not update {failed} in your profile. Try again.</p>
  {/if}
</div>
