package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCryptoReadinessRequiresEveryBoundaryAndEvidence(t *testing.T) {
	root := t.TempDir()
	cfg := Defaults(root)
	cfg.Runtime.Profile = RuntimeProfileSovereignOffline
	report := BuildCryptoReadiness(cfg, root)
	if report.Ready || report.MissingBoundaries != len(RequiredSovereignCryptoBoundaries) || report.FIPSPosture != "not_claimed" {
		t.Fatalf("empty report = %+v", report)
	}

	cfg.SovereignCryptoBoundaries = readyCryptoBoundaries(t, root, "customer_approved")
	if err := validateSovereignCryptoBoundaries(cfg.SovereignCryptoBoundaries); err != nil {
		t.Fatalf("validate boundaries: %v", err)
	}
	report = BuildCryptoReadiness(cfg, root)
	if !report.Ready || report.MissingBoundaries != 0 || report.MissingEvidence != 0 || report.FIPSPosture != "not_claimed" || report.CustomerOwned != len(RequiredSovereignCryptoBoundaries) {
		t.Fatalf("ready customer-approved report = %+v", report)
	}
	for _, row := range report.Boundaries {
		if row.Status != "ready" || len(row.Evidence) != 4 {
			t.Fatalf("boundary row = %+v", row)
		}
	}
}

func TestBuildCryptoReadinessRequiresExactValidatedModuleEvidence(t *testing.T) {
	root := t.TempDir()
	cfg := Defaults(root)
	cfg.Runtime.Profile = RuntimeProfileSovereignOffline
	cfg.SovereignCryptoBoundaries = readyCryptoBoundaries(t, root, "fips_140_3_validated")
	report := BuildCryptoReadiness(cfg, root)
	if report.Ready || report.FIPSPosture != "not_claimed" || !containsCryptoMissing(report, "validation_certificate") || !containsCryptoMissing(report, "validated_configuration") {
		t.Fatalf("missing validation evidence report = %+v", report)
	}

	certificate := writeCryptoEvidence(t, root, "module-certificate.json")
	configuration := writeCryptoEvidence(t, root, "validated-configuration.json")
	for i := range cfg.SovereignCryptoBoundaries {
		cfg.SovereignCryptoBoundaries[i].ValidationCertificate = certificate
		cfg.SovereignCryptoBoundaries[i].ValidatedConfiguration = configuration
	}
	report = BuildCryptoReadiness(cfg, root)
	if !report.Ready || report.FIPSPosture != "external_module_evidence_recorded" || !strings.Contains(report.ProductClaim, "not FIPS") {
		t.Fatalf("validated module evidence report = %+v", report)
	}
}

func TestBuildCryptoReadinessRejectsUnsafeOrMissingEvidence(t *testing.T) {
	root := t.TempDir()
	cfg := Defaults(root)
	cfg.SovereignCryptoBoundaries = readyCryptoBoundaries(t, root, "customer_approved")
	cfg.SovereignCryptoBoundaries[0].CustodyEvidence = "https://example.test/private-proof"
	cfg.SovereignCryptoBoundaries[1].RotationEvidence = "../outside.json"
	target := writeCryptoEvidence(t, root, "target.json")
	symlink := filepath.Join(root, "symlink.json")
	if err := os.Symlink(filepath.Join(root, target), symlink); err != nil {
		t.Fatal(err)
	}
	cfg.SovereignCryptoBoundaries[2].RecoveryEvidence = "symlink.json"
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}
	cfg.SovereignCryptoBoundaries[3].ApprovalEvidence = "linked-dir/outside.json"
	report := BuildCryptoReadiness(cfg, root)
	if report.Ready || report.MissingEvidence < 4 {
		t.Fatalf("unsafe evidence report = %+v", report)
	}

	cfg.SovereignCryptoBoundaries[0].KeyReference = "secret=SHOULD_NOT_RENDER"
	if err := validateSovereignCryptoBoundaries(cfg.SovereignCryptoBoundaries); err == nil || !strings.Contains(err.Error(), "secret-like") || strings.Contains(err.Error(), "SHOULD_NOT_RENDER") {
		t.Fatalf("secret-like boundary error = %v", err)
	}
}

func TestValidateSovereignCryptoBoundaryShape(t *testing.T) {
	root := t.TempDir()
	boundaries := readyCryptoBoundaries(t, root, "customer_approved")
	duplicate := append(append([]SovereignCryptoBoundary(nil), boundaries...), boundaries[0])
	if err := validateSovereignCryptoBoundaries(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate boundary error = %v", err)
	}
	badOwner := append([]SovereignCryptoBoundary(nil), boundaries...)
	badOwner[0].Owner = "vendor"
	if err := validateSovereignCryptoBoundaries(badOwner); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("owner error = %v", err)
	}
	badAssurance := append([]SovereignCryptoBoundary(nil), boundaries...)
	badAssurance[0].ModuleAssurance = "fips"
	if err := validateSovereignCryptoBoundaries(badAssurance); err == nil || !strings.Contains(err.Error(), "module_assurance") {
		t.Fatalf("assurance error = %v", err)
	}
}

func readyCryptoBoundaries(t *testing.T, root, assurance string) []SovereignCryptoBoundary {
	t.Helper()
	approval := writeCryptoEvidence(t, root, "approval.json")
	custody := writeCryptoEvidence(t, root, "custody.json")
	rotation := writeCryptoEvidence(t, root, "rotation.json")
	recovery := writeCryptoEvidence(t, root, "recovery.json")
	rows := make([]SovereignCryptoBoundary, 0, len(RequiredSovereignCryptoBoundaries))
	for _, name := range RequiredSovereignCryptoBoundaries {
		rows = append(rows, SovereignCryptoBoundary{Name: name, Owner: "customer", Custodian: "customer-security", KeyReference: "pkcs11:customer-key-" + name, Algorithm: "customer-approved", ModuleName: "customer-module", ModuleVersion: "1.0", ModuleAssurance: assurance, ApprovalEvidence: approval, CustodyEvidence: custody, RotationEvidence: rotation, RecoveryEvidence: recovery})
	}
	return rows
}

func writeCryptoEvidence(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func containsCryptoMissing(report CryptoReadinessReport, target string) bool {
	for _, row := range report.Boundaries {
		for _, missing := range row.Missing {
			if missing == target {
				return true
			}
		}
	}
	return false
}
