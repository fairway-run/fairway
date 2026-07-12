package audit

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/store"
)

const (
	AuditExportManifestSchema  = "fairway.sovereign-audit-export.v1"
	AuditExportRecordSchema    = "fairway.sovereign-audit-record.v1"
	AuditExportSignatureSchema = "fairway.sovereign-audit-signature.v1"
	AuditVerificationSchema    = "fairway.sovereign-audit-verification.v1"
	maxAuditExportFileBytes    = 32 << 20
)

type AuditExportOptions struct {
	Records                   []store.AuditRecord
	OutputDirectory           string
	GeneratedAt               time.Time
	PolicyID                  string
	SourceVersion             string
	TrustedTimeSource         string
	TrustedTimeEvidence       string
	TrustedTimeEvidenceSHA256 string
	RetentionPolicy           string
	LegalHold                 string
	ExternalTarget            string
	SigningKeyBase64          string
	Genesis                   bool
	Previous                  *AuditExportManifest
	PreviousManifestSHA256    string
}

type AuditExportManifest struct {
	Schema                    string `json:"schema"`
	ExportVersion             string `json:"export_version"`
	GeneratedAt               string `json:"generated_at"`
	ProjectID                 string `json:"project_id"`
	PolicyID                  string `json:"policy_id"`
	SourceVersion             string `json:"source_version"`
	TrustedTimeSource         string `json:"trusted_time_source"`
	TrustedTimeEvidence       string `json:"trusted_time_evidence"`
	TrustedTimeEvidenceSHA256 string `json:"trusted_time_evidence_sha256"`
	RetentionPolicy           string `json:"retention_policy"`
	LegalHold                 string `json:"legal_hold"`
	ExternalTarget            string `json:"external_target"`
	RecordCount               int    `json:"record_count"`
	FirstRecordID             int64  `json:"first_record_id"`
	LastRecordID              int64  `json:"last_record_id"`
	ChainAlgorithm            string `json:"chain_algorithm"`
	ChainHead                 string `json:"chain_head"`
	RecordsSHA256             string `json:"records_sha256"`
	RecordsBytes              int    `json:"records_bytes"`
	SigningKeyID              string `json:"signing_key_id"`
	Genesis                   bool   `json:"genesis"`
	PreviousManifestSHA256    string `json:"previous_manifest_sha256,omitempty"`
	PreviousChainHead         string `json:"previous_chain_head,omitempty"`
	PreviousRecordCount       int    `json:"previous_record_count,omitempty"`
	PreviousLastRecordID      int64  `json:"previous_last_record_id,omitempty"`
	AuthorityBoundary         string `json:"authority_boundary"`
}

type AuditExportRecord struct {
	Schema         string `json:"schema"`
	Sequence       int    `json:"sequence"`
	SourceRecordID int64  `json:"source_record_id"`
	ProjectID      string `json:"project_id"`
	Actor          string `json:"actor"`
	Action         string `json:"action"`
	TaskID         string `json:"task_id,omitempty"`
	DetailSHA256   string `json:"detail_sha256"`
	CreatedAt      string `json:"created_at"`
	PreviousHash   string `json:"previous_hash"`
	RecordHash     string `json:"record_hash"`
}

type AuditExportSignature struct {
	Schema         string `json:"schema"`
	Algorithm      string `json:"algorithm"`
	ManifestSHA256 string `json:"manifest_sha256"`
	PublicKey      string `json:"public_key"`
	Signature      string `json:"signature"`
}

type AuditVerifyOptions struct {
	Directory                      string
	TrustedPublicKeyBase64         string
	PreviousDirectory              string
	PreviousTrustedPublicKeyBase64 string
}

type AuditVerificationReport struct {
	Schema            string              `json:"schema"`
	OK                bool                `json:"ok"`
	ProjectID         string              `json:"project_id,omitempty"`
	RecordCount       int                 `json:"record_count"`
	LastRecordID      int64               `json:"last_record_id"`
	ChainHead         string              `json:"chain_head,omitempty"`
	ManifestSHA256    string              `json:"manifest_sha256,omitempty"`
	SignatureStatus   string              `json:"signature_status"`
	ContinuityStatus  string              `json:"continuity_status"`
	TrustedTimeStatus string              `json:"trusted_time_status"`
	Privacy           AuditExportPrivacy  `json:"privacy"`
	Issues            []string            `json:"issues,omitempty"`
	AuthorityBoundary string              `json:"authority_boundary"`
	Manifest          AuditExportManifest `json:"manifest"`
}

type AuditExportPrivacy struct {
	DetailContentIncluded bool `json:"detail_content_included"`
	RawPromptsIncluded    bool `json:"raw_prompts_included"`
	TranscriptsIncluded   bool `json:"transcripts_included"`
	CredentialsIncluded   bool `json:"credentials_included"`
}

func LocalAuditEvidenceDigest(root, reference string) (string, string, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" || strings.Contains(reference, "://") || strings.HasPrefix(reference, "urn:") {
		return "", "", errors.New("trusted time evidence must be a local file reference")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", errors.New("resolve project root")
	}
	path := reference
	if !filepath.IsAbs(path) {
		path = filepath.Join(absRoot, filepath.Clean(path))
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", errors.New("resolve trusted time evidence")
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", errors.New("trusted time evidence must remain inside the project root")
	}
	current := absRoot
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", "", errors.New("trusted time evidence path is missing or contains a symlink")
		}
	}
	info, err := os.Stat(absPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxAuditExportFileBytes {
		return "", "", errors.New("trusted time evidence must be a bounded regular file")
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", "", errors.New("read trusted time evidence")
	}
	digest := sha256.Sum256(data)
	return filepath.ToSlash(rel), hex.EncodeToString(digest[:]), nil
}

func VerifiedAuditCheckpoint(directory, trustedPublicKey string) (AuditExportManifest, string, error) {
	report, _, manifestBytes := verifyAuditDirectory(directory, trustedPublicKey)
	if !report.OK {
		return AuditExportManifest{}, "", errors.New("previous audit checkpoint failed integrity, signature, or trusted-time verification")
	}
	digest := sha256.Sum256(manifestBytes)
	return report.Manifest, hex.EncodeToString(digest[:]), nil
}

func ExportAuditPackage(opts AuditExportOptions) (AuditExportManifest, error) {
	if opts.GeneratedAt.IsZero() {
		return AuditExportManifest{}, errors.New("audit export generation time is required")
	}
	if len(opts.Records) == 0 {
		return AuditExportManifest{}, errors.New("audit export requires at least one audit record")
	}
	for _, field := range []struct{ label, value string }{
		{"policy id", opts.PolicyID}, {"source version", opts.SourceVersion}, {"trusted time source", opts.TrustedTimeSource},
		{"trusted time evidence", opts.TrustedTimeEvidence}, {"trusted time evidence digest", opts.TrustedTimeEvidenceSHA256},
		{"retention policy", opts.RetentionPolicy}, {"external target", opts.ExternalTarget},
	} {
		if err := validateAuditMetadata(field.label, field.value); err != nil {
			return AuditExportManifest{}, err
		}
	}
	if strings.TrimSpace(opts.LegalHold) == "" {
		return AuditExportManifest{}, errors.New("audit export legal-hold status is required")
	}
	if opts.LegalHold != "none" && opts.LegalHold != "active" {
		return AuditExportManifest{}, errors.New("audit export legal-hold status must be none or active")
	}
	if !isSHA256(opts.TrustedTimeEvidenceSHA256) {
		return AuditExportManifest{}, errors.New("audit export trusted time evidence digest must be sha256")
	}
	privateKey, err := decodeAuditSigningKey(opts.SigningKeyBase64)
	if err != nil {
		return AuditExportManifest{}, err
	}
	if opts.Genesis == (opts.Previous != nil) {
		return AuditExportManifest{}, errors.New("audit export requires exactly one continuity mode: genesis or previous verified export")
	}
	rows, recordsBytes, heads, err := buildAuditExportRecords(opts.Records)
	if err != nil {
		return AuditExportManifest{}, err
	}
	for _, record := range opts.Records {
		createdAt, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
		if createdAt.After(opts.GeneratedAt.UTC()) {
			return AuditExportManifest{}, errors.New("audit record created_at is after the export generation time")
		}
	}
	_ = rows
	manifest := AuditExportManifest{
		Schema: AuditExportManifestSchema, ExportVersion: "v1", GeneratedAt: opts.GeneratedAt.UTC().Format(time.RFC3339Nano),
		ProjectID: opts.Records[0].ProjectID, PolicyID: opts.PolicyID, SourceVersion: opts.SourceVersion,
		TrustedTimeSource: opts.TrustedTimeSource, TrustedTimeEvidence: opts.TrustedTimeEvidence,
		TrustedTimeEvidenceSHA256: opts.TrustedTimeEvidenceSHA256, RetentionPolicy: opts.RetentionPolicy,
		LegalHold: opts.LegalHold, ExternalTarget: opts.ExternalTarget, RecordCount: len(opts.Records),
		FirstRecordID: opts.Records[0].ID, LastRecordID: opts.Records[len(opts.Records)-1].ID,
		ChainAlgorithm: "sha256-canonical-json-chain", ChainHead: heads[len(heads)-1], Genesis: opts.Genesis,
		AuthorityBoundary: "tamper-evident audit evidence only; not certification, compliance, authorization, approval, risk acceptance, or proof of external retention",
	}
	recordsDigest := sha256.Sum256(recordsBytes)
	manifest.RecordsSHA256 = hex.EncodeToString(recordsDigest[:])
	manifest.RecordsBytes = len(recordsBytes)
	if len(recordsBytes) > maxAuditExportFileBytes {
		return AuditExportManifest{}, errors.New("audit export records exceed the supported package size")
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	keyDigest := sha256.Sum256(publicKey)
	manifest.SigningKeyID = "sha256:" + hex.EncodeToString(keyDigest[:])
	if opts.Previous != nil {
		if !isSHA256(opts.PreviousManifestSHA256) {
			return AuditExportManifest{}, errors.New("previous audit manifest digest must be sha256")
		}
		if opts.Previous.ProjectID != manifest.ProjectID {
			return AuditExportManifest{}, errors.New("previous audit export project does not match")
		}
		previousTime, err := time.Parse(time.RFC3339Nano, opts.Previous.GeneratedAt)
		if err != nil || !opts.GeneratedAt.UTC().After(previousTime.UTC()) {
			return AuditExportManifest{}, errors.New("audit export generation time must be after the previous checkpoint")
		}
		if opts.Previous.RecordCount > len(heads) || opts.Previous.RecordCount == 0 {
			return AuditExportManifest{}, errors.New("current audit state is behind the previous checkpoint")
		}
		if heads[opts.Previous.RecordCount-1] != opts.Previous.ChainHead || opts.Records[opts.Previous.RecordCount-1].ID != opts.Previous.LastRecordID {
			return AuditExportManifest{}, errors.New("current audit history does not extend the previous checkpoint")
		}
		manifest.PreviousManifestSHA256 = opts.PreviousManifestSHA256
		manifest.PreviousChainHead = opts.Previous.ChainHead
		manifest.PreviousRecordCount = opts.Previous.RecordCount
		manifest.PreviousLastRecordID = opts.Previous.LastRecordID
	}
	if err := validateAuditManifest(manifest); err != nil {
		return AuditExportManifest{}, errors.New("audit export manifest is incomplete")
	}
	directory, err := prepareAuditExportDirectory(opts.OutputDirectory)
	if err != nil {
		return AuditExportManifest{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(directory)
		}
	}()
	if err := os.WriteFile(filepath.Join(directory, "records.jsonl"), recordsBytes, 0o600); err != nil {
		return AuditExportManifest{}, errors.New("write audit records")
	}
	manifestBytes, err := stableAuditJSON(manifest)
	if err != nil {
		return AuditExportManifest{}, err
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), manifestBytes, 0o600); err != nil {
		return AuditExportManifest{}, errors.New("write audit manifest")
	}
	signature := signAuditManifest(manifestBytes, privateKey)
	signatureBytes, _ := stableAuditJSON(signature)
	if err := os.WriteFile(filepath.Join(directory, "signature.json"), signatureBytes, 0o600); err != nil {
		return AuditExportManifest{}, errors.New("write audit signature")
	}
	cleanup = false
	return manifest, nil
}

func VerifyAuditPackage(opts AuditVerifyOptions) (AuditVerificationReport, error) {
	report, records, manifestBytes := verifyAuditDirectory(opts.Directory, opts.TrustedPublicKeyBase64)
	if !report.OK {
		return report, nil
	}
	if report.Manifest.Genesis {
		if strings.TrimSpace(opts.PreviousDirectory) != "" {
			report.OK = false
			report.Issues = append(report.Issues, "genesis audit export must not have a previous export")
		}
		report.ContinuityStatus = "genesis"
		return report, nil
	}
	if strings.TrimSpace(opts.PreviousDirectory) == "" {
		report.OK = false
		report.ContinuityStatus = "previous_required"
		report.Issues = append(report.Issues, "non-genesis audit export requires the previous externally retained export for continuity verification")
		return report, nil
	}
	previous, _, previousManifestBytes := verifyAuditDirectory(opts.PreviousDirectory, opts.PreviousTrustedPublicKeyBase64)
	if !previous.OK {
		report.OK = false
		report.ContinuityStatus = "previous_invalid"
		report.Issues = append(report.Issues, "previous audit export failed verification")
		return report, nil
	}
	previousDigest := sha256.Sum256(previousManifestBytes)
	if report.Manifest.PreviousManifestSHA256 != hex.EncodeToString(previousDigest[:]) ||
		report.Manifest.PreviousChainHead != previous.ChainHead ||
		report.Manifest.PreviousRecordCount != previous.RecordCount ||
		report.Manifest.PreviousLastRecordID != previous.LastRecordID ||
		report.ProjectID != previous.ProjectID ||
		len(records) < previous.RecordCount || records[previous.RecordCount-1].RecordHash != previous.ChainHead {
		report.OK = false
		report.ContinuityStatus = "mismatch"
		report.Issues = append(report.Issues, "audit export does not extend the previous verified checkpoint")
		return report, nil
	}
	_ = manifestBytes
	report.ContinuityStatus = "verified_previous"
	return report, nil
}

func verifyAuditDirectory(directory, trustedPublicKey string) (AuditVerificationReport, []AuditExportRecord, []byte) {
	report := AuditVerificationReport{Schema: AuditVerificationSchema, SignatureStatus: "invalid", ContinuityStatus: "not_checked", TrustedTimeStatus: "missing",
		Privacy: AuditExportPrivacy{}, AuthorityBoundary: "integrity and continuity verification only; not certification, compliance, authorization, approval, risk acceptance, or proof that external WORM/SIEM retention occurred"}
	clean, err := existingAuditExportDirectory(directory)
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, nil, nil
	}
	if err := rejectUnknownAuditFiles(clean); err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, nil, nil
	}
	manifestBytes, err := readBoundedAuditFile(clean, "manifest.json")
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, nil, nil
	}
	if err := strictAuditJSON(manifestBytes, &report.Manifest); err != nil || validateAuditManifest(report.Manifest) != nil {
		report.Issues = append(report.Issues, "audit manifest is invalid")
		return report, nil, manifestBytes
	}
	report.ProjectID = report.Manifest.ProjectID
	report.RecordCount = report.Manifest.RecordCount
	report.LastRecordID = report.Manifest.LastRecordID
	report.ChainHead = report.Manifest.ChainHead
	manifestDigest := sha256.Sum256(manifestBytes)
	report.ManifestSHA256 = hex.EncodeToString(manifestDigest[:])
	recordsBytes, err := readBoundedAuditFile(clean, "records.jsonl")
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, nil, manifestBytes
	}
	recordsDigest := sha256.Sum256(recordsBytes)
	if subtle.ConstantTimeCompare([]byte(report.Manifest.RecordsSHA256), []byte(hex.EncodeToString(recordsDigest[:]))) != 1 || len(recordsBytes) != report.Manifest.RecordsBytes {
		report.Issues = append(report.Issues, "audit records digest or size mismatch")
		return report, nil, manifestBytes
	}
	records, err := parseAndVerifyAuditRecords(recordsBytes, report.Manifest)
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, nil, manifestBytes
	}
	signatureBytes, err := readBoundedAuditFile(clean, "signature.json")
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return report, records, manifestBytes
	}
	var signature AuditExportSignature
	if err := strictAuditJSON(signatureBytes, &signature); err != nil || signature.Schema != AuditExportSignatureSchema || signature.Algorithm != "ed25519-sha256" || signature.ManifestSHA256 != report.ManifestSHA256 {
		report.Issues = append(report.Issues, "audit signature metadata is invalid")
		return report, records, manifestBytes
	}
	publicKey, err := base64.StdEncoding.DecodeString(signature.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		report.Issues = append(report.Issues, "audit signature public key is invalid")
		return report, records, manifestBytes
	}
	keyDigest := sha256.Sum256(publicKey)
	if report.Manifest.SigningKeyID != "sha256:"+hex.EncodeToString(keyDigest[:]) {
		report.Issues = append(report.Issues, "audit signing key identity mismatch")
		return report, records, manifestBytes
	}
	sig, err := base64.StdEncoding.DecodeString(signature.Signature)
	if err != nil || !ed25519.Verify(publicKey, manifestDigest[:], sig) {
		report.Issues = append(report.Issues, "audit signature verification failed")
		return report, records, manifestBytes
	}
	trustedKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(trustedPublicKey))
	if err != nil || len(trustedKey) != ed25519.PublicKeySize || subtle.ConstantTimeCompare(publicKey, trustedKey) != 1 {
		report.SignatureStatus = "untrusted"
		report.Issues = append(report.Issues, "audit signing key does not match the pinned trusted public key")
		return report, records, manifestBytes
	}
	report.SignatureStatus = "verified_pinned"
	if report.Manifest.TrustedTimeSource != "" && isSHA256(report.Manifest.TrustedTimeEvidenceSHA256) {
		report.TrustedTimeStatus = "evidence_bound"
	}
	report.OK = report.SignatureStatus == "verified_pinned" && report.TrustedTimeStatus == "evidence_bound"
	return report, records, manifestBytes
}

func buildAuditExportRecords(records []store.AuditRecord) ([]AuditExportRecord, []byte, []string, error) {
	var out []AuditExportRecord
	var data bytes.Buffer
	var heads []string
	previous := strings.Repeat("0", sha256.Size*2)
	projectID := records[0].ProjectID
	var lastID int64
	var lastCreatedAt time.Time
	for i, source := range records {
		if source.ProjectID == "" || source.ProjectID != projectID || source.ID <= lastID || source.Actor == "" || source.Action == "" ||
			containsPrivateAuditValue(source.ProjectID) || containsPrivateAuditValue(source.Actor) || containsPrivateAuditValue(source.Action) || containsPrivateAuditValue(source.TaskID) ||
			len(source.Actor) > 2048 || len(source.Action) > 2048 || len(source.TaskID) > 2048 || strings.ContainsAny(source.Actor+source.Action+source.TaskID, "\r\n") {
			return nil, nil, nil, errors.New("audit records are not one ordered project history")
		}
		createdAt, err := time.Parse(time.RFC3339Nano, source.CreatedAt)
		if err != nil {
			return nil, nil, nil, errors.New("audit record has invalid created_at")
		}
		if !lastCreatedAt.IsZero() && createdAt.Before(lastCreatedAt) {
			return nil, nil, nil, errors.New("audit record created_at is not monotonic")
		}
		detailDigest := sha256.Sum256([]byte(source.Detail))
		row := AuditExportRecord{Schema: AuditExportRecordSchema, Sequence: i + 1, SourceRecordID: source.ID, ProjectID: source.ProjectID,
			Actor: source.Actor, Action: source.Action, TaskID: source.TaskID, DetailSHA256: hex.EncodeToString(detailDigest[:]), CreatedAt: source.CreatedAt, PreviousHash: previous}
		input, _ := canonicalAuditRecord(row)
		digest := sha256.Sum256(input)
		row.RecordHash = hex.EncodeToString(digest[:])
		line, _ := json.Marshal(row)
		data.Write(line)
		data.WriteByte('\n')
		previous = row.RecordHash
		heads = append(heads, row.RecordHash)
		out = append(out, row)
		lastID = source.ID
		lastCreatedAt = createdAt
	}
	return out, data.Bytes(), heads, nil
}

func parseAndVerifyAuditRecords(data []byte, manifest AuditExportManifest) ([]AuditExportRecord, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	var records []AuditExportRecord
	previous := strings.Repeat("0", sha256.Size*2)
	var lastID int64
	generatedAt, _ := time.Parse(time.RFC3339Nano, manifest.GeneratedAt)
	var lastCreatedAt time.Time
	for scanner.Scan() {
		var row AuditExportRecord
		if err := strictAuditJSON(scanner.Bytes(), &row); err != nil || row.Schema != AuditExportRecordSchema {
			return nil, errors.New("audit record JSON is invalid")
		}
		if row.Sequence != len(records)+1 || row.SourceRecordID <= lastID || row.ProjectID != manifest.ProjectID || row.PreviousHash != previous || !isSHA256(row.DetailSHA256) {
			return nil, errors.New("audit record scope, order, or chain metadata mismatch")
		}
		createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
		if err != nil {
			return nil, errors.New("audit record created_at is invalid")
		}
		if createdAt.After(generatedAt) || (!lastCreatedAt.IsZero() && createdAt.Before(lastCreatedAt)) {
			return nil, errors.New("audit record created_at is outside the ordered export time boundary")
		}
		input, _ := canonicalAuditRecord(row)
		digest := sha256.Sum256(input)
		if subtle.ConstantTimeCompare([]byte(row.RecordHash), []byte(hex.EncodeToString(digest[:]))) != 1 {
			return nil, errors.New("audit record hash mismatch")
		}
		previous = row.RecordHash
		lastID = row.SourceRecordID
		lastCreatedAt = createdAt
		records = append(records, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("read audit records")
	}
	if len(records) != manifest.RecordCount || len(records) == 0 || records[0].SourceRecordID != manifest.FirstRecordID || records[len(records)-1].SourceRecordID != manifest.LastRecordID || records[len(records)-1].RecordHash != manifest.ChainHead {
		return nil, errors.New("audit record count or chain head mismatch")
	}
	return records, nil
}

func canonicalAuditRecord(row AuditExportRecord) ([]byte, error) {
	row.RecordHash = ""
	return json.Marshal(row)
}

func signAuditManifest(manifest []byte, privateKey ed25519.PrivateKey) AuditExportSignature {
	digest := sha256.Sum256(manifest)
	return AuditExportSignature{Schema: AuditExportSignatureSchema, Algorithm: "ed25519-sha256", ManifestSHA256: hex.EncodeToString(digest[:]),
		PublicKey: base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, digest[:]))}
}

func decodeAuditSigningKey(value string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, errors.New("decode audit signing key")
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, errors.New("audit signing key must be an Ed25519 seed or private key")
	}
}

func validateAuditMetadata(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("audit export %s is required", label)
	}
	if len(value) > 2048 || strings.ContainsAny(value, "\r\n") || strings.Contains(value, "://") || containsPrivateAuditValue(value) {
		return fmt.Errorf("audit export %s contains invalid or secret-like metadata", label)
	}
	return nil
}

func validateAuditManifest(manifest AuditExportManifest) error {
	if manifest.Schema != AuditExportManifestSchema || manifest.ExportVersion != "v1" || manifest.ChainAlgorithm != "sha256-canonical-json-chain" ||
		manifest.RecordCount <= 0 || manifest.FirstRecordID <= 0 || manifest.LastRecordID < manifest.FirstRecordID || !isSHA256(manifest.ChainHead) ||
		!isSHA256(manifest.RecordsSHA256) || manifest.RecordsBytes <= 0 || !strings.HasPrefix(manifest.SigningKeyID, "sha256:") || !isSHA256(strings.TrimPrefix(manifest.SigningKeyID, "sha256:")) ||
		manifest.AuthorityBoundary != "tamper-evident audit evidence only; not certification, compliance, authorization, approval, risk acceptance, or proof of external retention" {
		return errors.New("invalid manifest structure")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.GeneratedAt); err != nil {
		return errors.New("invalid manifest generated_at")
	}
	for _, field := range []struct{ label, value string }{
		{"project id", manifest.ProjectID}, {"policy id", manifest.PolicyID}, {"source version", manifest.SourceVersion},
		{"trusted time source", manifest.TrustedTimeSource}, {"trusted time evidence", manifest.TrustedTimeEvidence},
		{"trusted time evidence digest", manifest.TrustedTimeEvidenceSHA256}, {"retention policy", manifest.RetentionPolicy}, {"external target", manifest.ExternalTarget},
	} {
		if err := validateAuditMetadata(field.label, field.value); err != nil {
			return err
		}
	}
	if manifest.LegalHold != "none" && manifest.LegalHold != "active" {
		return errors.New("invalid legal hold status")
	}
	if !isSHA256(manifest.TrustedTimeEvidenceSHA256) {
		return errors.New("invalid trusted time evidence digest")
	}
	if manifest.Genesis {
		if manifest.PreviousManifestSHA256 != "" || manifest.PreviousChainHead != "" || manifest.PreviousRecordCount != 0 || manifest.PreviousLastRecordID != 0 {
			return errors.New("genesis manifest contains previous checkpoint fields")
		}
	} else if !isSHA256(manifest.PreviousManifestSHA256) || !isSHA256(manifest.PreviousChainHead) || manifest.PreviousRecordCount <= 0 || manifest.PreviousLastRecordID <= 0 {
		return errors.New("non-genesis manifest is missing previous checkpoint fields")
	}
	return nil
}

func containsPrivateAuditValue(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"-----begin private key", "authorization: bearer", "api_key=", "access_token=", "refresh_token=", "client_secret=", "password=", "secret="} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isSHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func prepareAuditExportDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return "", errors.New("audit export output must be a local directory")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", errors.New("resolve audit export output")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", errors.New("audit export parent directory must exist")
	}
	abs = filepath.Join(parent, filepath.Base(abs))
	if _, err := os.Lstat(abs); !os.IsNotExist(err) {
		return "", errors.New("audit export output must not already exist")
	}
	if err := os.Mkdir(abs, 0o700); err != nil {
		return "", errors.New("create audit export output")
	}
	return abs, nil
}

func existingAuditExportDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return "", errors.New("audit export directory must be local")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", errors.New("resolve audit export directory")
	}
	info, err := os.Lstat(abs)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("audit export path is not a non-symlink directory")
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", errors.New("resolve audit export directory")
	}
	return resolved, nil
}

func rejectUnknownAuditFiles(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return errors.New("read audit export directory")
	}
	allowed := map[string]bool{"manifest.json": true, "records.jsonl": true, "signature.json": true}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
		if !allowed[entry.Name()] || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return errors.New("audit export contains unknown or unsafe files")
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "manifest.json,records.jsonl,signature.json" {
		return errors.New("audit export is incomplete")
	}
	return nil
}

func readBoundedAuditFile(directory, name string) ([]byte, error) {
	path := filepath.Join(directory, name)
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxAuditExportFileBytes {
		return nil, fmt.Errorf("audit export file %s is missing or unsafe", name)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open audit export file %s", name)
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maxAuditExportFileBytes+1))
}

func strictAuditJSON(data []byte, target any) error {
	if err := rejectDuplicateAuditObjectKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON content")
	}
	return nil
}

func rejectDuplicateAuditObjectKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return errors.New("JSON value must be an object")
	}
	seen := map[string]bool{}
	for decoder.More() {
		rawKey, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := rawKey.(string)
		if !ok || seen[key] {
			return errors.New("JSON object contains a duplicate or invalid key")
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return errors.New("JSON object is malformed")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON content")
	}
	return nil
}

func stableAuditJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
