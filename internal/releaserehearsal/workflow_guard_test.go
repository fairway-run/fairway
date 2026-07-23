package releaserehearsal

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseWorkflowBindsManualRecoveryThroughPromotion(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(content)
	required := []string{
		"workflow_dispatch:",
		"version:\n        description: Existing immutable annotated release tag to promote",
		"ref: ${{ github.event_name == 'workflow_dispatch' && inputs.version || github.ref }}",
		`tag_ref=$(gh api "repos/$GITHUB_REPOSITORY/git/ref/tags/$RELEASE_VERSION")`,
		`tag_object=$(gh api "repos/$GITHUB_REPOSITORY/git/tags/$tag_oid")`,
		`"$(git rev-parse HEAD)" == "$source_sha"`,
		"tag_oid: ${{ steps.rehearsal.outputs.tag_oid }}",
		"source_sha: ${{ steps.rehearsal.outputs.source_sha }}",
		"VERIFIED_TAG_OID: ${{ needs.verify.outputs.tag_oid }}",
		"VERIFIED_SOURCE_SHA: ${{ needs.verify.outputs.source_sha }}",
		`current_tag_ref=$(gh api "repos/$GITHUB_REPOSITORY/git/ref/tags/$RELEASE_VERSION")`,
		`"$(jq -r '.object.sha' <<<"$current_tag_ref")" == "$VERIFIED_TAG_OID"`,
		`current_tag_object=$(gh api "repos/$GITHUB_REPOSITORY/git/tags/$VERIFIED_TAG_OID")`,
		`"$(jq -r '.object.sha' <<<"$current_tag_object")" == "$VERIFIED_SOURCE_SHA"`,
		`gh release create "$RELEASE_VERSION"`,
	}
	for _, want := range required {
		if !strings.Contains(workflow, want) {
			t.Fatalf("release workflow is missing required recovery binding %q", want)
		}
	}

	ordered := []string{
		`tag_ref=$(gh api "repos/$GITHUB_REPOSITORY/git/ref/tags/$RELEASE_VERSION")`,
		`"$(git rev-parse HEAD)" == "$source_sha"`,
		`printf 'tag_oid=%s\n' "$tag_oid"`,
		"VERIFIED_TAG_OID: ${{ needs.verify.outputs.tag_oid }}",
		`current_tag_ref=$(gh api "repos/$GITHUB_REPOSITORY/git/ref/tags/$RELEASE_VERSION")`,
		`current_tag_object=$(gh api "repos/$GITHUB_REPOSITORY/git/tags/$VERIFIED_TAG_OID")`,
		`gh release create "$RELEASE_VERSION"`,
	}
	previous := -1
	for _, marker := range ordered {
		index := strings.Index(workflow, marker)
		if index <= previous {
			t.Fatalf("release workflow marker %q is missing or out of order", marker)
		}
		previous = index
	}
}
