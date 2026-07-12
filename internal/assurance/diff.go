package assurance

import (
	"fmt"
	"reflect"
	"sort"
)

const ProfileDiffSchema = "fairway.assurance-profile-diff.v1"

type ProfileDiffReport struct {
	Schema         string              `json:"schema"`
	FromProfileID  string              `json:"from_profile_id"`
	ToProfileID    string              `json:"to_profile_id"`
	FromVersion    string              `json:"from_version"`
	ToVersion      string              `json:"to_version"`
	Classification string              `json:"classification"`
	Compatible     bool                `json:"compatible"`
	RequiresReview bool                `json:"requires_review"`
	Changes        []ProfileDiffChange `json:"changes"`
	Boundary       string              `json:"authority_boundary"`
}

type ProfileDiffChange struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	Compatibility string `json:"compatibility"`
	Summary       string `json:"summary"`
}

// DiffProfiles compares two versioned profile contracts. It does not infer
// framework equivalence or approve a profile update.
func DiffProfiles(from, to Profile) (ProfileDiffReport, error) {
	if err := Validate(from); err != nil {
		return ProfileDiffReport{}, fmt.Errorf("validate from profile: %w", err)
	}
	if err := Validate(to); err != nil {
		return ProfileDiffReport{}, fmt.Errorf("validate to profile: %w", err)
	}
	report := ProfileDiffReport{
		Schema:        ProfileDiffSchema,
		FromProfileID: from.ID,
		ToProfileID:   to.ID,
		FromVersion:   from.Version,
		ToVersion:     to.Version,
		Compatible:    true,
		Changes:       []ProfileDiffChange{},
		Boundary:      "profile differences require accountable review; this report does not approve mappings or grant certification authority",
	}
	add := func(path, kind, compatibility, summary string) {
		report.Changes = append(report.Changes, ProfileDiffChange{Path: path, Kind: kind, Compatibility: compatibility, Summary: summary})
	}
	if from.ID != to.ID {
		add("id", "changed", "breaking", "profile identity changed")
	}
	if from.Version != to.Version {
		add("version", "changed", "informational", "profile version changed")
	}
	if from.Title != to.Title {
		add("title", "changed", "informational", "profile title changed")
	}
	if from.Description != to.Description {
		add("description", "changed", "informational", "profile description changed")
	}
	if from.Schema != to.Schema {
		add("schema", "changed", "breaking", "profile schema changed")
	}
	if !reflect.DeepEqual(from.Framework, to.Framework) {
		add("framework", "changed", "breaking", "framework identity, version, title, or authoritative source changed")
	}
	if !reflect.DeepEqual(from.Applicability, to.Applicability) {
		add("applicability", "changed", "breaking", "profile applicability changed")
	}
	appendSetDiff(&report.Changes, "scope.types", from.Scope.Types, to.Scope.Types, "additive")
	appendSetDiff(&report.Changes, "prohibited_claims", from.ProhibitedClaims, to.ProhibitedClaims, "additive")
	if from.Authority.Mode != to.Authority.Mode {
		add("authority.mode", "changed", "breaking", "authority mode changed")
	}
	appendSetDiff(&report.Changes, "authority.prohibited_actions", from.Authority.ProhibitedActions, to.Authority.ProhibitedActions, "additive")

	fromControls := controlsByID(from.Controls)
	toControls := controlsByID(to.Controls)
	controlIDs := make(map[string]bool, len(fromControls)+len(toControls))
	for id := range fromControls {
		controlIDs[id] = true
	}
	for id := range toControls {
		controlIDs[id] = true
	}
	for _, id := range sortedKeys(controlIDs) {
		before, hadBefore := fromControls[id]
		after, hasAfter := toControls[id]
		switch {
		case !hadBefore:
			add("controls."+id, "added", "additive", "control added")
		case !hasAfter:
			add("controls."+id, "removed", "breaking", "control removed")
		case !reflect.DeepEqual(before, after):
			add("controls."+id, "changed", "breaking", "control objective, responsibility, assessment, or evidence contract changed")
		}
	}

	semanticChange := false
	for _, change := range report.Changes {
		if change.Path != "version" {
			semanticChange = true
			break
		}
	}
	if semanticChange && from.Version == to.Version {
		add("version", "reused", "breaking", "profile content changed without a version change")
	}
	sort.Slice(report.Changes, func(i, j int) bool {
		if report.Changes[i].Path != report.Changes[j].Path {
			return report.Changes[i].Path < report.Changes[j].Path
		}
		if report.Changes[i].Kind != report.Changes[j].Kind {
			return report.Changes[i].Kind < report.Changes[j].Kind
		}
		return report.Changes[i].Summary < report.Changes[j].Summary
	})

	report.RequiresReview = len(report.Changes) > 0
	report.Classification = "unchanged"
	for _, change := range report.Changes {
		switch change.Compatibility {
		case "breaking":
			report.Classification = "breaking"
			report.Compatible = false
		case "additive":
			if report.Classification != "breaking" {
				report.Classification = "additive"
			}
		case "informational":
			if report.Classification == "unchanged" {
				report.Classification = "metadata_only"
			}
		}
	}
	return report, nil
}

func appendSetDiff(changes *[]ProfileDiffChange, path string, before, after []string, addedCompatibility string) {
	left := stringSet(before...)
	right := stringSet(after...)
	keys := make(map[string]bool, len(left)+len(right))
	for value := range left {
		keys[value] = true
	}
	for value := range right {
		keys[value] = true
	}
	for _, value := range sortedKeys(keys) {
		switch {
		case !left[value]:
			*changes = append(*changes, ProfileDiffChange{Path: path + "." + value, Kind: "added", Compatibility: addedCompatibility, Summary: "value added"})
		case !right[value]:
			*changes = append(*changes, ProfileDiffChange{Path: path + "." + value, Kind: "removed", Compatibility: "breaking", Summary: "value removed"})
		}
	}
}

func controlsByID(controls []Control) map[string]Control {
	result := make(map[string]Control, len(controls))
	for _, control := range controls {
		result[control.ID] = control
	}
	return result
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}
