## Context

`/my/profile`'s Settings tab renders `ProfileForm.svelte`, a two-tab form
("Skills & role" / "Location & work") sharing one "Save changes" button that PUTs
the whole profile row. Skills and Skills to avoid currently sit in the "Skills &
role" tab next to Role/specializations, CV upload, and headshot.

`profileStore` (`web/src/lib/profile.svelte.ts`) already exposes single-skill
autosave mutators — `addSkill`/`removeSkill`/`avoidSkill`/`unavoidSkill` — each
issuing its own `PUT /me/profile` through a serial queue (so two edits made a
moment apart don't clobber each other, since the endpoint replaces the whole
row). These are used today by `JobMatch.svelte`'s "I have it" / "Avoid" chips on
a job posting — proof the autosave pattern already works end-to-end, just not
yet exposed as a standalone profile-editing surface.

Top-level page tabs on `/my/profile` (`web/src/routes/my/profile/+page.svelte`)
only render once a profile exists; before that, the bare `ProfileForm` is the
entire page (no tab strip). The server also requires at least one skill to
create a profile (`skills` must be non-empty on `PUT /me/profile`).

## Goals / Non-Goals

**Goals:**
- Give Skills / Skills to avoid their own top-level tab, edited with immediate
  per-toggle autosave (no shared Save button).
- Keep first-time profile creation working unchanged (Role + Skills entered
  together, one Save, since no profile — and no tab strip — exists yet).
- Keep CV-driven skill extraction working for an existing profile without a
  Settings-tab skills field to merge into.

**Non-Goals:**
- No change to the `PUT /api/v1/me/profile` contract or `user_profiles` schema.
- No change to Role/specializations, location preferences, headshot, or CV
  storage/readiness behavior.
- No new "review before applying" UI for CV-extracted skills (out of scope per
  the resolved design question — extraction autosaves directly).

## Decisions

**Skills tab reads/writes the store directly; no local buffer.**
`SkillsView.svelte` renders straight off `profileStore.profile?.skills` /
`?.excluded_skills` and calls `addSkill`/`removeSkill`/`avoidSkill`/`unavoidSkill`
on each chip toggle — mirroring `JobMatch.svelte`'s existing `avoided` derived-set
pattern. Alternative considered: give the new tab its own buffered
skills/excludedSkills state plus a local Save button, matching `ProfileForm`'s
existing shape. Rejected — it would duplicate the buffer-and-PUT machinery
`ProfileForm` already has, when the per-skill mutators already do exactly this
job and are proven in production by `JobMatch`.

**Skills stays inside `ProfileForm` during creation, leaves it once editing.**
`ProfileForm` keeps its skills/excluded-skills fields when `profile === null`
(`!editing`) because the create flow has no tab strip yet and the server won't
accept a skill-less profile. Once a profile exists, those fields disappear from
the form; the "Skills & role" tab is relabeled "Role". Alternative considered:
always show a placeholder/link in Settings pointing at the new tab. Rejected as
unnecessary indirection — the new top-level tab is one click away in the same
tab strip.

**CV upload against an existing profile autosaves extracted skills via a new
bulk mutator, not N sequential single-skill calls.**
`profileStore.addSkills(skills: string[])` folds `withSkill` over every
extracted skill and issues one `PUT`, queued through the same `#queue` as the
other mutators (so it can't race a manual toggle from the Skills tab). Looping
the existing single-skill `addSkill` was considered and rejected: it would issue
one full-row `PUT` per extracted skill (5-15 typically), all serialized, for no
benefit over one bulk write.

**Shared skill-dictionary loader.**
Both `ProfileForm` (create flow) and `SkillsView` need the same typeahead
universe — `api.facetCounts` → sort by descending job count. Extracted to
`web/src/lib/skillDictionary.ts` (`loadSkillDistribution(): Promise<FacetOption[]>`)
rather than duplicated, since it's now needed in two independent components.

## Risks / Trade-offs

- **[Risk]** A user toggles a skill on the new tab while `ProfileForm`'s
  Settings tab is also mounted (both subscribe to the same `profileStore`,
  possible only in the create-flow's absence of tabs — not reachable once
  editing, since Settings and Skills are mutually exclusive tab panels). →
  **Mitigation**: not reachable in practice; the two are never both mounted for
  an existing profile since `TabRow` shows one panel at a time.
- **[Risk]** Losing the explicit Save affordance could surprise a user used to
  reviewing before committing. → **Mitigation**: this mirrors the existing,
  shipped `JobMatch` skill-editing UX exactly, so the interaction pattern is
  already validated in this codebase, not a novel one.
- **[Trade-off]** CV-extracted skills for an existing profile now write
  immediately with no review step, vs. the "surface as suggestions" alternative
  considered and declined during brainstorming (adds a review UI for a case that
  behaves acceptably today: false extractions can simply be removed on the
  Skills tab, same as an unwanted manual entry).

## Migration Plan

Frontend-only, single deploy, no data migration, no feature flag. Rollback is a
plain revert (no schema/API change to unwind).

## Open Questions

None outstanding — placement, autosave-vs-Save, and CV-extraction behavior were
resolved during brainstorming (see
`docs/superpowers/specs/2026-08-04-profile-skills-tab-design.md`).
