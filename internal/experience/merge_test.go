package experience

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUnionForMergeScreenshotPair(t *testing.T) {
	keep := Atom{
		ID:         uuid.New(),
		Claim:      "Built a Chromium plugin with custom audio transcription pipeline using faster-whisper models, with configurable profiles for live and batch processing",
		Provenance: ProvenanceAgentInferred,
		Skills:     []string{"nlp", "python"},
	}
	lose := Atom{
		ID:         uuid.New(),
		Claim:      "Built a Chromium plugin that transcribes audio using faster-whisper models with configurable profiles and VAD-based filtering",
		Context:    "small/medium/large-v3 model profiles",
		Metrics:    []string{"VAD filtering"},
		Provenance: ProvenanceAgentInferred,
		Skills:     []string{"python"},
	}
	got := unionForMerge(keep, lose)
	if got.Claim != keep.Claim {
		t.Errorf("claim changed: %q", got.Claim)
	}
	if got.Context != lose.Context {
		t.Errorf("context = %q, want the non-empty one", got.Context)
	}
	if len(got.Metrics) != 1 || got.Metrics[0] != "VAD filtering" {
		t.Errorf("metrics = %q", got.Metrics)
	}
	if got.Provenance != ProvenanceAgentInferred {
		t.Errorf("provenance = %q, want agent_inferred for two unconfirmed", got.Provenance)
	}
	// Both already carried python/nlp; Sanitize keeps canonicals. Length ≥ 1 is enough —
	// the point of this case is context/metrics union, not the skill dictionary.
	if len(got.Skills) == 0 {
		t.Error("skills emptied")
	}
}

func TestMergeProvenancePublishableSibling(t *testing.T) {
	keep := Atom{Claim: "a", Provenance: ProvenanceAgentInferred}
	lose := Atom{Claim: "b", Provenance: ProvenanceStatedInChat, Context: "situation"}
	got := unionForMerge(keep, lose)
	if got.Provenance != ProvenanceStatedInChat {
		t.Errorf("provenance = %q, want stated_in_chat", got.Provenance)
	}
	if !got.Provenance.Publishable() {
		t.Error("merged atom should be publishable")
	}
}

func TestValidateMergePair(t *testing.T) {
	idA, idB := uuid.New(), uuid.New()
	roleA, roleB := uuid.New(), uuid.New()
	if err := validateMergePair(idA, idA, Atom{ID: idA}, Atom{ID: idA}); err != ErrInvalidMerge {
		t.Errorf("same id: %v", err)
	}
	left := Atom{ID: idA, EmploymentID: &roleA}
	right := Atom{ID: idB, EmploymentID: &roleB}
	if err := validateMergePair(idA, idB, left, right); err != ErrMergeCrossEmployment {
		t.Errorf("cross employment: %v", err)
	}
	unplacedA := Atom{ID: idA}
	unplacedB := Atom{ID: idB}
	if err := validateMergePair(idA, idB, unplacedA, unplacedB); err != nil {
		t.Errorf("two unplaced: %v", err)
	}
}

func TestChooseKeepRicherAndOlder(t *testing.T) {
	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	thin := mergeCandidate{Atom: Atom{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Claim: "x", Provenance: ProvenanceAgentInferred}, CreatedAt: older}
	rich := mergeCandidate{Atom: Atom{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Claim: "x", Context: "how", Metrics: []string{"1"}, Provenance: ProvenanceManual}, CreatedAt: newer}
	if chooseKeep(thin, rich) {
		t.Error("should keep the richer atom even if newer")
	}
	a := mergeCandidate{Atom: Atom{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Claim: "x", Provenance: ProvenanceManual}, CreatedAt: older}
	b := mergeCandidate{Atom: Atom{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Claim: "x", Provenance: ProvenanceManual}, CreatedAt: newer}
	if !chooseKeep(a, b) {
		t.Error("equal richness: keep the older")
	}
}
