package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const CryptoReadinessSchema = "fairway.sovereign-crypto-readiness.v1"

var RequiredSovereignCryptoBoundaries = []string{
	"in_transit",
	"at_rest",
	"backup",
	"evidence_export",
	"signing",
}

type SovereignCryptoBoundary struct {
	Name                   string `toml:"name" json:"name"`
	Owner                  string `toml:"owner" json:"owner"`
	Custodian              string `toml:"custodian" json:"custodian"`
	KeyReference           string `toml:"key_reference" json:"key_reference"`
	Algorithm              string `toml:"algorithm" json:"algorithm"`
	ModuleName             string `toml:"module_name" json:"module_name"`
	ModuleVersion          string `toml:"module_version" json:"module_version"`
	ModuleAssurance        string `toml:"module_assurance" json:"module_assurance"`
	ApprovalEvidence       string `toml:"approval_evidence" json:"approval_evidence"`
	ValidationCertificate  string `toml:"validation_certificate" json:"validation_certificate,omitempty"`
	ValidatedConfiguration string `toml:"validated_configuration" json:"validated_configuration,omitempty"`
	CustodyEvidence        string `toml:"custody_evidence" json:"custody_evidence"`
	RotationEvidence       string `toml:"rotation_evidence" json:"rotation_evidence"`
	RecoveryEvidence       string `toml:"recovery_evidence" json:"recovery_evidence"`
}

type CryptoBoundaryReadiness struct {
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	Owner           string   `json:"owner,omitempty"`
	Custodian       string   `json:"custodian,omitempty"`
	KeyReference    string   `json:"key_reference,omitempty"`
	Algorithm       string   `json:"algorithm,omitempty"`
	ModuleName      string   `json:"module_name,omitempty"`
	ModuleVersion   string   `json:"module_version,omitempty"`
	ModuleAssurance string   `json:"module_assurance,omitempty"`
	Missing         []string `json:"missing,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`
}

type CryptoReadinessReport struct {
	Schema            string                    `json:"schema"`
	RuntimeProfile    string                    `json:"runtime_profile"`
	Ready             bool                      `json:"ready"`
	FIPSPosture       string                    `json:"fips_posture"`
	ProductClaim      string                    `json:"product_claim"`
	Boundaries        []CryptoBoundaryReadiness `json:"boundaries"`
	MissingBoundaries int                       `json:"missing_boundaries"`
	MissingEvidence   int                       `json:"missing_evidence"`
	CustomerOwned     int                       `json:"customer_owned"`
	ProductOwned      int                       `json:"product_owned"`
	SharedOwned       int                       `json:"shared_owned"`
	ProhibitedClaims  []string                  `json:"prohibited_claims"`
}

func validateSovereignCryptoBoundaries(boundaries []SovereignCryptoBoundary) error {
	seen := map[string]bool{}
	for _, boundary := range boundaries {
		name := strings.TrimSpace(boundary.Name)
		if !containsString(RequiredSovereignCryptoBoundaries, name) {
			return fmt.Errorf("[[sovereign_crypto_boundaries]] name %q is unsupported", boundary.Name)
		}
		if seen[name] {
			return fmt.Errorf("[[sovereign_crypto_boundaries]] contains duplicate name %q", name)
		}
		seen[name] = true
		if boundary.Owner != "" && boundary.Owner != "customer" && boundary.Owner != "product" && boundary.Owner != "shared" {
			return fmt.Errorf("[[sovereign_crypto_boundaries]] owner %q for %s is unsupported", boundary.Owner, name)
		}
		if boundary.ModuleAssurance != "" && boundary.ModuleAssurance != "customer_approved" && boundary.ModuleAssurance != "fips_140_3_validated" && boundary.ModuleAssurance != "not_assessed" {
			return fmt.Errorf("[[sovereign_crypto_boundaries]] module_assurance %q for %s is unsupported", boundary.ModuleAssurance, name)
		}
		for _, value := range cryptoBoundaryValues(boundary) {
			if len(value) > 2048 || strings.ContainsAny(value, "\r\n") {
				return fmt.Errorf("[[sovereign_crypto_boundaries]] %s contains an invalid multiline or oversized value", name)
			}
			if containsSecretLikeCryptoValue(value) {
				return fmt.Errorf("[[sovereign_crypto_boundaries]] %s contains a secret-like value; store only metadata references", name)
			}
		}
	}
	return nil
}

func BuildCryptoReadiness(cfg Config, root string) CryptoReadinessReport {
	configured := map[string]SovereignCryptoBoundary{}
	for _, boundary := range cfg.SovereignCryptoBoundaries {
		configured[strings.TrimSpace(boundary.Name)] = boundary
	}
	report := CryptoReadinessReport{
		Schema:         CryptoReadinessSchema,
		RuntimeProfile: RuntimeProfile(cfg),
		Ready:          true,
		FIPSPosture:    "not_claimed",
		ProductClaim:   "Fairway is not FIPS 140-3 validated; recorded external module evidence applies only to the named module and configuration.",
		ProhibitedClaims: []string{
			"fairway_is_fips_validated",
			"customer_deployment_is_certified",
			"module_evidence_authorizes_operation",
		},
	}
	allExternallyValidated := true
	for _, name := range RequiredSovereignCryptoBoundaries {
		boundary, ok := configured[name]
		row := CryptoBoundaryReadiness{Name: name, Status: "missing"}
		if !ok {
			row.Missing = []string{"boundary_definition"}
			report.MissingBoundaries++
			report.Ready = false
			allExternallyValidated = false
			report.Boundaries = append(report.Boundaries, row)
			continue
		}
		row.Owner = boundary.Owner
		row.Custodian = boundary.Custodian
		row.KeyReference = boundary.KeyReference
		row.Algorithm = boundary.Algorithm
		row.ModuleName = boundary.ModuleName
		row.ModuleVersion = boundary.ModuleVersion
		row.ModuleAssurance = boundary.ModuleAssurance
		for field, value := range map[string]string{
			"owner":             boundary.Owner,
			"custodian":         boundary.Custodian,
			"key_reference":     boundary.KeyReference,
			"algorithm":         boundary.Algorithm,
			"module_name":       boundary.ModuleName,
			"module_version":    boundary.ModuleVersion,
			"module_assurance":  boundary.ModuleAssurance,
			"approval_evidence": boundary.ApprovalEvidence,
			"custody_evidence":  boundary.CustodyEvidence,
			"rotation_evidence": boundary.RotationEvidence,
			"recovery_evidence": boundary.RecoveryEvidence,
		} {
			if strings.TrimSpace(value) == "" {
				row.Missing = append(row.Missing, field)
			}
		}
		if boundary.ModuleAssurance == "not_assessed" {
			row.Missing = append(row.Missing, "approved_module_evidence")
		}
		if boundary.ModuleAssurance == "fips_140_3_validated" {
			if strings.TrimSpace(boundary.ValidationCertificate) == "" {
				row.Missing = append(row.Missing, "validation_certificate")
			}
			if strings.TrimSpace(boundary.ValidatedConfiguration) == "" {
				row.Missing = append(row.Missing, "validated_configuration")
			}
		} else {
			allExternallyValidated = false
		}
		for label, reference := range map[string]string{
			"approval_evidence":       boundary.ApprovalEvidence,
			"custody_evidence":        boundary.CustodyEvidence,
			"rotation_evidence":       boundary.RotationEvidence,
			"recovery_evidence":       boundary.RecoveryEvidence,
			"validation_certificate":  boundary.ValidationCertificate,
			"validated_configuration": boundary.ValidatedConfiguration,
		} {
			if reference == "" {
				continue
			}
			resolved, ok := localCryptoEvidence(root, reference)
			if !ok {
				row.Missing = append(row.Missing, label+"_local_proof")
				continue
			}
			row.Evidence = append(row.Evidence, resolved)
		}
		sort.Strings(row.Missing)
		sort.Strings(row.Evidence)
		if len(row.Missing) == 0 {
			row.Status = "ready"
		} else {
			report.Ready = false
			report.MissingEvidence += len(row.Missing)
		}
		switch boundary.Owner {
		case "customer":
			report.CustomerOwned++
		case "product":
			report.ProductOwned++
		case "shared":
			report.SharedOwned++
		}
		report.Boundaries = append(report.Boundaries, row)
	}
	if allExternallyValidated && report.Ready {
		report.FIPSPosture = "external_module_evidence_recorded"
	}
	return report
}

func localCryptoEvidence(root, reference string) (string, bool) {
	reference = strings.TrimSpace(reference)
	if reference == "" || strings.Contains(reference, "://") || strings.HasPrefix(reference, "urn:") {
		return "", false
	}
	path := reference
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.Clean(path))
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	current := absRoot
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", false
		}
	}
	info, err := os.Lstat(absPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func cryptoBoundaryValues(boundary SovereignCryptoBoundary) []string {
	return []string{boundary.Name, boundary.Owner, boundary.Custodian, boundary.KeyReference, boundary.Algorithm, boundary.ModuleName, boundary.ModuleVersion, boundary.ModuleAssurance, boundary.ApprovalEvidence, boundary.ValidationCertificate, boundary.ValidatedConfiguration, boundary.CustodyEvidence, boundary.RotationEvidence, boundary.RecoveryEvidence}
}

func containsSecretLikeCryptoValue(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"-----begin private key", "authorization: bearer", "api_key=", "access_token=", "refresh_token=", "client_secret=", "password=", "secret="} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
