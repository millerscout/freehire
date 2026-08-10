package experience

import (
	"testing"

	"github.com/google/uuid"
)

func TestSeedHistoryFromBankSplitsJobsAndProjects(t *testing.T) {
	jobID := uuid.New()
	projectID := uuid.New()
	employments := []Employment{
		{ID: jobID, Kind: KindJob, Company: "RingCentral", Role: "SWE", Location: "Remote"},
		{ID: projectID, Kind: KindProject, Company: "telagon.io", Link: "https://telagon.io"},
	}
	atoms := []Atom{
		{EmploymentID: &jobID, Claim: "Cut latency", Provenance: ProvenanceCVImport},
		{EmploymentID: &projectID, Claim: "1.4M+ channels", Provenance: ProvenanceStatedInChat},
		{EmploymentID: &projectID, Claim: "model guess", Provenance: ProvenanceAgentInferred},
	}

	got := seedHistoryFromBank(employments, atoms)

	if !got.HasEmployments || !got.HasProjectEmployments {
		t.Fatalf("flags = employments:%v projects:%v, want both true", got.HasEmployments, got.HasProjectEmployments)
	}
	if len(got.Experience) != 1 || got.Experience[0].Company != "RingCentral" {
		t.Errorf("experience = %+v, want the job only", got.Experience)
	}
	if len(got.Experience[0].Highlights) != 1 || got.Experience[0].Highlights[0] != "Cut latency" {
		t.Errorf("job highlights = %+v", got.Experience[0].Highlights)
	}
	if len(got.Projects) != 1 {
		t.Fatalf("projects = %+v, want one banked project", got.Projects)
	}
	p := got.Projects[0]
	if p.Name != "telagon.io" || p.Link != "https://telagon.io" {
		t.Errorf("project identity = %+v, want name and link", p)
	}
	if len(p.Highlights) != 1 || p.Highlights[0] != "1.4M+ channels" {
		t.Errorf("project highlights = %+v, want publishable only", p.Highlights)
	}
}

func TestSeedHistoryFromBankJobsOnly(t *testing.T) {
	jobID := uuid.New()
	got := seedHistoryFromBank(
		[]Employment{{ID: jobID, Kind: KindJob, Company: "Acme", Role: "Dev"}},
		nil,
	)
	if !got.HasEmployments || got.HasProjectEmployments {
		t.Errorf("flags = %+v, want employments without projects", got)
	}
	if len(got.Projects) != 0 {
		t.Errorf("projects = %+v, want none", got.Projects)
	}
}

func TestSeedHistoryFromBankEmpty(t *testing.T) {
	got := seedHistoryFromBank(nil, nil)
	if got.HasEmployments || got.HasProjectEmployments {
		t.Errorf("empty bank flags = %+v", got)
	}
	if len(got.Experience) != 0 || len(got.Projects) != 0 {
		t.Errorf("empty bank rows = %+v", got)
	}
}

func TestSeedHistoryProjectWithoutPublishableAtomsStillListed(t *testing.T) {
	id := uuid.New()
	got := seedHistoryFromBank(
		[]Employment{{ID: id, Kind: KindProject, Company: "opensched", Link: "https://opensched.dev"}},
		[]Atom{{EmploymentID: &id, Claim: "inferred", Provenance: ProvenanceAgentInferred}},
	)
	if len(got.Projects) != 1 || got.Projects[0].Link != "https://opensched.dev" {
		t.Errorf("projects = %+v, want identity even without publishable highlights", got.Projects)
	}
	if len(got.Projects[0].Highlights) != 0 {
		t.Errorf("highlights = %+v, want none from agent_inferred", got.Projects[0].Highlights)
	}
}

// WorkHistory still flattens projects into experience-shaped rows for fit analysis.
func TestWorkHistoryStillFlattensProjects(t *testing.T) {
	jobID := uuid.New()
	projectID := uuid.New()
	flat := experienceFromBank(
		[]Employment{
			{ID: jobID, Kind: KindJob, Company: "Acme", Role: "Dev"},
			{ID: projectID, Kind: KindProject, Company: "opensched", Link: "https://opensched.dev"},
		},
		nil,
	)
	if len(flat) != 2 {
		t.Fatalf("WorkHistory-shaped rows = %d, want job and project flattened", len(flat))
	}
	var companies []string
	for _, e := range flat {
		companies = append(companies, e.Company)
	}
	if companies[0] != "Acme" || companies[1] != "opensched" {
		t.Errorf("companies = %v, want Acme then opensched", companies)
	}
}
