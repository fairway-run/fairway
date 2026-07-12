package assurance

import "testing"

func TestDiffProfilesClassifiesAdditiveAndBreakingChanges(t *testing.T) {
	base := mustLoadProfileYAML(t, validProfileYAML)
	unchanged, err := DiffProfiles(base, base)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Classification != "unchanged" || !unchanged.Compatible || unchanged.RequiresReview {
		t.Fatalf("unexpected unchanged report: %#v", unchanged)
	}

	additive := base
	additive.Version = "v2"
	additive.Controls = append([]Control(nil), base.Controls...)
	added := base.Controls[0]
	added.ID = "EX-2"
	additive.Controls = append(additive.Controls, added)
	report, err := DiffProfiles(base, additive)
	if err != nil {
		t.Fatal(err)
	}
	if report.Classification != "additive" || !report.Compatible || !report.RequiresReview || !hasDiff(report, "controls.EX-2", "added") {
		t.Fatalf("unexpected additive report: %#v", report)
	}

	breaking := additive
	breaking.Version = "v3"
	breaking.Controls = append([]Control(nil), breaking.Controls[:1]...)
	breaking.Controls[0].Responsibility = "customer"
	report, err = DiffProfiles(additive, breaking)
	if err != nil {
		t.Fatal(err)
	}
	if report.Classification != "breaking" || report.Compatible || !hasDiff(report, "controls.EX-1", "changed") || !hasDiff(report, "controls.EX-2", "removed") {
		t.Fatalf("unexpected breaking report: %#v", report)
	}
}

func TestDiffProfilesRejectsSilentVersionReuse(t *testing.T) {
	base := mustLoadProfileYAML(t, validProfileYAML)
	changed := base
	changed.Description = "Updated bounded evidence description."
	report, err := DiffProfiles(base, changed)
	if err != nil {
		t.Fatal(err)
	}
	if report.Classification != "breaking" || report.Compatible || !hasDiff(report, "version", "reused") {
		t.Fatalf("unexpected version reuse report: %#v", report)
	}
}

func hasDiff(report ProfileDiffReport, path, kind string) bool {
	for _, change := range report.Changes {
		if change.Path == path && change.Kind == kind {
			return true
		}
	}
	return false
}
