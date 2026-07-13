package assurance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDocumentationClaimsRejectsUnsupportedAssertions(t *testing.T) {
	for _, content := range []string{
		"Fairway is ISO 27001 certified.\n",
		"This release is SOC 2 compliant.\n",
		"The deployment is CUI authorized.\n",
		"Fairway is EU CRA conformant.\n",
		"This product is Common Criteria certified at EAL 4.\n",
		"This configuration is approved for the national cloud.\n",
		"Draft: Fairway is ISO 27001 certified.\n",
		"Example: This deployment is SOC 2 compliant.\n",
		"> Fairway is EU CRA conformant.\n",
		"Input only: This package is CUI authorized.\n",
		"Fairway is not ISO 27001 certified, but this deployment is SOC 2 compliant.\n",
		"Fairway is not ISO 27001 certified and this deployment is SOC 2 compliant.\n",
		"Without external assessment, Fairway is Common Criteria certified.\n",
		"Fairway is FIPS validated only if a customer selects a validated module.\n",
		"Fairway must not log secrets while this deployment is ISO 27001 certified.\n",
		"Fairway never exposes tokens whereas this product is SOC 2 compliant.\n",
		"Fairway is not documented as experimental, this deployment is CUI authorized.\n",
		"Fairway is not only ISO 27001 certified.\n",
		"Fairway is not merely SOC 2 compliant.\n",
		"Fairway is not necessarily CUI authorized.\n",
		"Fairway is not un-Common Criteria certified.\n",
	} {
		path := filepath.Join(t.TempDir(), "claim.md")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		report, err := ValidateDocumentationClaims(path)
		if err == nil || report.Valid || !strings.Contains(err.Error(), "unsupported certification or regulatory assertion") {
			t.Fatalf("unsupported claim passed content=%q report=%+v err=%v", content, report, err)
		}
		if strings.Contains(err.Error(), "ISO") || strings.Contains(err.Error(), "CUI") || strings.Contains(err.Error(), "Fairway") {
			t.Fatalf("claim error echoed document content: %v", err)
		}
	}
}

func TestValidateDocumentationClaimsAllowsExplicitBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bounded.md")
	content := `# Assessment input

Fairway is not ISO 27001 certified or SOC 2 compliant.
This draft provides EU CRA input only and does not assert conformity.
Common Criteria certified wording is prohibited without an external certificate.
CUI authorization requires external customer and assessment authority.
Fairway must not be represented as FIPS validated.
This product is not EU CRA conformant or Common Criteria certified.
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := ValidateDocumentationClaims(path)
	if err != nil || !report.Valid || report.Schema != DocumentationClaimValidationSchema || report.LinesChecked == 0 {
		t.Fatalf("bounded document validation=%+v err=%v", report, err)
	}
}

func TestValidateDocumentationClaimsRejectsUnsafeInput(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.md")
	if err := os.WriteFile(target, []byte("bounded input\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateDocumentationClaims(link); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("symlink validation error=%v", err)
	}
	jsonPath := filepath.Join(dir, "claim.json")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateDocumentationClaims(jsonPath); err == nil || !strings.Contains(err.Error(), ".md or .txt") {
		t.Fatalf("unsupported extension error=%v", err)
	}
}
