package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/subashram/fairway/internal/audit"
	"github.com/subashram/fairway/internal/config"
	"github.com/subashram/fairway/internal/dashboard"
	"github.com/subashram/fairway/internal/identityproof"
	"github.com/subashram/fairway/internal/store"
)

const sovereignRehearsalSchema = "fairway.sovereign-customer-key-rehearsal.v1"

type sovereignRehearsalReport struct {
	Schema                       string   `json:"schema"`
	OK                           bool     `json:"ok"`
	GeneratedAt                  string   `json:"generated_at"`
	Project                      string   `json:"project"`
	WorkspaceType                string   `json:"workspace_type"`
	IdentityKeyFingerprint       string   `json:"identity_key_fingerprint"`
	RecoveryKeyFingerprint       string   `json:"recovery_key_fingerprint"`
	AuditKeyFingerprint          string   `json:"audit_key_fingerprint"`
	PositiveAuthorization        string   `json:"positive_authorization"`
	RoleRejection                string   `json:"role_rejection"`
	RevocationRejection          string   `json:"revocation_rejection"`
	KeyLossRejection             string   `json:"key_loss_rejection"`
	RecoveryAuthorization        string   `json:"recovery_authorization"`
	IdentitySubstitutionRejected bool     `json:"identity_substitution_rejected"`
	AuditSignatureStatus         string   `json:"audit_signature_status"`
	AuditSubstitutionRejected    bool     `json:"audit_substitution_rejected"`
	PrivateFilesMode0600         bool     `json:"private_files_mode_0600"`
	PrivateKeysDestroyed         bool     `json:"private_keys_destroyed"`
	RetainedFiles                []string `json:"retained_files"`
	NonClaims                    []string `json:"non_claims"`
	AuthorityBoundary            string   `json:"authority_boundary"`
}

type sovereignRehearsalRunOptions struct {
	Workspace    string
	Output       string
	Project      string
	GeneratedAt  time.Time
	RequireTmpfs bool
}

func cmdSecurityRehearsal(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpOnly(args) {
		fmt.Println("fairway security rehearsal run --workspace <tmpfs-dir> --out <new-retained-dir> --project <id> --at <RFC3339> [--format text|json]")
		fmt.Println("  Run an offline customer-key authorization and audit rehearsal; private keys remain in tmpfs and are removed before return.")
		return nil
	}
	if args[0] != "run" {
		return fmt.Errorf("unknown security rehearsal subcommand %q", args[0])
	}
	if isHelpOnly(args[1:]) {
		return cmdSecurityRehearsal(ctx, opts, nil)
	}
	fs := flag.NewFlagSet("security rehearsal run", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "existing operator-controlled Linux tmpfs directory")
	out := fs.String("out", "", "new retained public evidence directory outside tmpfs")
	project := fs.String("project", "", "bounded rehearsal project identity")
	at := fs.String("at", "", "rehearsal time in RFC3339")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*workspace) == "" || strings.TrimSpace(*out) == "" || strings.TrimSpace(*project) == "" || strings.TrimSpace(*at) == "" {
		return errors.New("security rehearsal run requires --workspace, --out, --project, and --at")
	}
	if *format != "text" && *format != "json" {
		return errors.New("--format must be text or json")
	}
	generatedAt, err := time.Parse(time.RFC3339, *at)
	if err != nil {
		return errors.New("--at must be RFC3339")
	}
	if delta := time.Since(generatedAt); delta < -5*time.Minute || delta > 5*time.Minute {
		return errors.New("--at must be within five minutes of the current clock")
	}
	report, err := runSovereignCustomerKeyRehearsal(ctx, sovereignRehearsalRunOptions{Workspace: *workspace, Output: *out, Project: *project, GeneratedAt: generatedAt, RequireTmpfs: true})
	if err != nil {
		return err
	}
	if opts.JSON || *format == "json" {
		return printJSON(report)
	}
	fmt.Printf("sovereign_customer_key_rehearsal: %s\nok: %t\nproject: %s\nidentity_key_fingerprint: %s\nrecovery_key_fingerprint: %s\naudit_key_fingerprint: %s\npositive_authorization: %s\nrole_rejection: %s\nrevocation_rejection: %s\nkey_loss_rejection: %s\nrecovery_authorization: %s\naudit_signature_status: %s\nprivate_keys_destroyed: %t\nauthority_boundary: %s\n",
		filepath.Clean(*out), report.OK, report.Project, report.IdentityKeyFingerprint, report.RecoveryKeyFingerprint, report.AuditKeyFingerprint,
		report.PositiveAuthorization, report.RoleRejection, report.RevocationRejection, report.KeyLossRejection, report.RecoveryAuthorization,
		report.AuditSignatureStatus, report.PrivateKeysDestroyed, report.AuthorityBoundary)
	return nil
}

func runSovereignCustomerKeyRehearsal(ctx context.Context, opts sovereignRehearsalRunOptions) (report sovereignRehearsalReport, err error) {
	workspace, err := filepath.Abs(strings.TrimSpace(opts.Workspace))
	if err != nil || !filepath.IsAbs(workspace) {
		return report, errors.New("resolve rehearsal workspace")
	}
	out, err := filepath.Abs(strings.TrimSpace(opts.Output))
	if err != nil || !filepath.IsAbs(out) {
		return report, errors.New("resolve rehearsal output")
	}
	outParent, err := filepath.EvalSymlinks(filepath.Dir(out))
	if err != nil {
		return report, errors.New("resolve rehearsal output parent")
	}
	out = filepath.Join(outParent, filepath.Base(out))
	if strings.TrimSpace(opts.Project) == "" || strings.ContainsAny(opts.Project, "\r\n") {
		return report, errors.New("rehearsal project is required and must be one line")
	}
	if opts.GeneratedAt.IsZero() {
		return report, errors.New("rehearsal generated time is required")
	}
	info, statErr := os.Stat(workspace)
	if statErr != nil || !info.IsDir() {
		return report, errors.New("rehearsal workspace must be an existing directory")
	}
	workspace, err = filepath.EvalSymlinks(workspace)
	if err != nil {
		return report, errors.New("resolve rehearsal workspace symlinks")
	}
	if pathContains(workspace, out) || pathContains(out, workspace) {
		return report, errors.New("rehearsal workspace and retained output must be separate paths")
	}
	if opts.RequireTmpfs {
		workspaceMount, workspaceType, checkErr := linuxMountForPath(workspace)
		if checkErr != nil {
			return report, checkErr
		}
		outputMount, outputType, checkErr := linuxMountForPath(filepath.Dir(out))
		if checkErr != nil {
			return report, checkErr
		}
		if err := validateRehearsalMounts(workspaceMount, workspaceType, outputMount, outputType); err != nil {
			return report, err
		}
	}
	if err := os.Mkdir(out, 0o700); err != nil {
		return report, errors.New("create new rehearsal output directory")
	}
	cleanupOutput := true
	defer func() {
		if cleanupOutput {
			_ = os.RemoveAll(out)
		}
	}()
	runDir, err := os.MkdirTemp(workspace, "fairway-customer-key-rehearsal.")
	if err != nil {
		return report, errors.New("create private rehearsal directory")
	}
	if err := os.Chmod(runDir, 0o700); err != nil {
		return report, errors.New("restrict private rehearsal directory")
	}
	defer os.RemoveAll(runDir)

	identityPublic, identityPrivate, err := ed25519.GenerateKey(nil)
	if err != nil {
		return report, errors.New("generate identity rehearsal key")
	}
	recoveryPublic, recoveryPrivate, err := ed25519.GenerateKey(nil)
	if err != nil {
		return report, errors.New("generate recovery rehearsal key")
	}
	auditPublic, auditPrivate, err := ed25519.GenerateKey(nil)
	if err != nil {
		return report, errors.New("generate audit rehearsal key")
	}
	if string(identityPublic) == string(recoveryPublic) || string(identityPublic) == string(auditPublic) || string(recoveryPublic) == string(auditPublic) {
		return report, errors.New("rehearsal generated duplicate key roots")
	}
	defer zeroRehearsalKey(identityPrivate)
	defer zeroRehearsalKey(recoveryPrivate)
	defer zeroRehearsalKey(auditPrivate)
	privateFiles := []struct {
		name string
		key  ed25519.PrivateKey
	}{
		{name: "identity-private.key", key: identityPrivate},
		{name: "identity-recovery-private.key", key: recoveryPrivate},
		{name: "audit-private.key", key: auditPrivate},
	}
	privateModeOK := true
	for _, file := range privateFiles {
		path := filepath.Join(runDir, file.name)
		if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(file.key)), 0o600); err != nil {
			return report, errors.New("write private rehearsal key")
		}
		fileInfo, err := os.Lstat(path)
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Mode().Perm() != 0o600 || fileInfo.Mode()&os.ModeSymlink != 0 {
			privateModeOK = false
		}
	}
	revocationFile := filepath.Join(runDir, "revocations.json")
	if err := os.WriteFile(revocationFile, []byte(`{"schema":"fairway.sovereign-revocations.v1"}`+"\n"), 0o600); err != nil {
		return report, errors.New("write rehearsal revocation file")
	}
	trustedTimePath := filepath.Join(runDir, "trusted-time.json")
	trustedTime := []byte(fmt.Sprintf("{\"source\":\"operator-rehearsal-clock\",\"at\":%q}\n", opts.GeneratedAt.UTC().Format(time.RFC3339)))
	if err := os.WriteFile(trustedTimePath, trustedTime, 0o600); err != nil {
		return report, errors.New("write rehearsal trusted-time evidence")
	}

	const identityEnv = "FAIRWAY_REHEARSAL_IDENTITY_PUBLIC_KEY"
	previousEnv, previousSet := os.LookupEnv(identityEnv)
	defer func() {
		if previousSet {
			_ = os.Setenv(identityEnv, previousEnv)
		} else {
			_ = os.Unsetenv(identityEnv)
		}
	}()
	if err := os.Setenv(identityEnv, base64.StdEncoding.EncodeToString(identityPublic)); err != nil {
		return report, errors.New("configure rehearsal public identity key")
	}
	cfg := config.Defaults(runDir)
	cfg.Fairway.ProjectName = opts.Project
	cfg.Runtime.Profile = config.RuntimeProfileSovereignOffline
	cfg.Dashboard.ReadOnly = true
	cfg.Server.Enabled = true
	cfg.Server.Mode = "read_only"
	cfg.Server.ReadOnly = true
	cfg.Server.WriteEnabled = false
	cfg.Server.IdentityMode = "sovereign_signed"
	cfg.Server.AllowedRoles = []string{"viewer", "operator", "authorizer"}
	cfg.Server.SovereignPublicKeyEnv = identityEnv
	cfg.Server.SovereignKeyID = "rehearsal-identity-1"
	cfg.Server.SovereignIssuer = "urn:fairway:sovereign-rehearsal"
	cfg.Server.SovereignAudience = "fairway-rehearsal"
	cfg.Server.SovereignRevocationFile = revocationFile
	cfg.Server.SovereignSessionMaxSeconds = 900
	cfg.Server.SovereignClockSkewSeconds = 0
	cfg.Server.SovereignBreakGlassSeconds = 300

	s, err := store.Open(ctx, filepath.Join(runDir, "state.db"), opts.Project)
	if err != nil {
		return report, errors.New("open rehearsal store")
	}
	defer s.Close()
	handler := dashboard.NewWithRoot(s, cfg, nil, nil, runDir).ReadOnlyAPIHandler()
	reportTime := opts.GeneratedAt.UTC()
	authorizationTime := time.Now().UTC()
	viewerClaims := rehearsalClaims(cfg, authorizationTime, "rehearsal-viewer", "viewer", "rehearsal-viewer-1")
	viewerProof, err := identityproof.Sign(identityPrivate, cfg.Server.SovereignKeyID, viewerClaims)
	if err != nil {
		return report, err
	}
	positiveCode, positiveBody := rehearsalAPIStatus(handler, viewerProof)
	if positiveCode != http.StatusOK || strings.Contains(positiveBody, viewerProof) {
		return report, fmt.Errorf("positive sovereign rehearsal authorization failed with HTTP %d", positiveCode)
	}
	operatorClaims := rehearsalClaims(cfg, authorizationTime, "rehearsal-operator", "operator", "rehearsal-operator-1")
	operatorProof, err := identityproof.Sign(identityPrivate, cfg.Server.SovereignKeyID, operatorClaims)
	if err != nil {
		return report, err
	}
	roleCode, roleBody := rehearsalAPIStatus(handler, operatorProof)
	if roleCode != http.StatusForbidden || strings.Contains(roleBody, operatorProof) {
		return report, errors.New("sovereign rehearsal role rejection failed")
	}
	if err := os.WriteFile(revocationFile, []byte(`{"schema":"fairway.sovereign-revocations.v1","revoked_proof_ids":["rehearsal-viewer-1"]}`+"\n"), 0o600); err != nil {
		return report, errors.New("write rehearsal revocation")
	}
	revokedCode, revokedBody := rehearsalAPIStatus(handler, viewerProof)
	if revokedCode != http.StatusUnauthorized || strings.Contains(revokedBody, viewerProof) {
		return report, errors.New("sovereign rehearsal revocation rejection failed")
	}
	if err := os.Unsetenv(identityEnv); err != nil {
		return report, errors.New("simulate rehearsal identity key loss")
	}
	lossCode, lossBody := rehearsalAPIStatus(handler, viewerProof)
	if lossCode != http.StatusUnauthorized || strings.Contains(lossBody, viewerProof) {
		return report, errors.New("sovereign rehearsal key-loss rejection failed")
	}
	if err := os.Setenv(identityEnv, base64.StdEncoding.EncodeToString(auditPublic)); err != nil {
		return report, errors.New("simulate rehearsal identity substitution")
	}
	substitutionCode, substitutionBody := rehearsalAPIStatus(handler, viewerProof)
	if substitutionCode != http.StatusUnauthorized || strings.Contains(substitutionBody, viewerProof) {
		return report, errors.New("sovereign rehearsal identity substitution was not rejected")
	}
	if err := os.WriteFile(revocationFile, []byte(`{"schema":"fairway.sovereign-revocations.v1"}`+"\n"), 0o600); err != nil {
		return report, errors.New("reset rehearsal revocation state")
	}
	if err := os.Setenv(identityEnv, base64.StdEncoding.EncodeToString(recoveryPublic)); err != nil {
		return report, errors.New("configure rehearsal recovery identity key")
	}
	cfg.Server.SovereignKeyID = "rehearsal-identity-recovery-1"
	recoveryHandler := dashboard.NewWithRoot(s, cfg, nil, nil, runDir).ReadOnlyAPIHandler()
	recoveryProof, err := identityproof.Sign(recoveryPrivate, cfg.Server.SovereignKeyID, rehearsalClaims(cfg, authorizationTime, "rehearsal-recovery", "viewer", "rehearsal-recovery-1"))
	if err != nil {
		return report, err
	}
	recoveryCode, recoveryBody := rehearsalAPIStatus(recoveryHandler, recoveryProof)
	if recoveryCode != http.StatusOK || strings.Contains(recoveryBody, recoveryProof) {
		return report, errors.New("sovereign rehearsal recovery authorization failed")
	}

	if err := s.RecordAudit(ctx, store.AuditEvent{Actor: "sovereign:rehearsal", Action: "rehearsal.identity", Detail: `{"result":"pass","private_content_included":false}`}); err != nil {
		return report, errors.New("record rehearsal audit fact")
	}
	records, err := s.AuditRecords(ctx)
	if err != nil {
		return report, errors.New("read rehearsal audit facts")
	}
	timeDigest := sha256.Sum256(trustedTime)
	auditDir := filepath.Join(out, "audit-export")
	auditGeneratedAt := time.Now().UTC()
	_, err = audit.ExportAuditPackage(audit.AuditExportOptions{
		Records: records, OutputDirectory: auditDir, GeneratedAt: auditGeneratedAt,
		PolicyID: "sovereign-audit-rehearsal-v1", SourceVersion: "rehearsal:" + version,
		TrustedTimeSource: "operator-rehearsal-clock", TrustedTimeEvidence: "tmpfs:trusted-time.json", TrustedTimeEvidenceSHA256: hex.EncodeToString(timeDigest[:]),
		RetentionPolicy: "rehearsal-only", LegalHold: "none", ExternalTarget: "operator-retained-rehearsal-output",
		SigningKeyBase64: base64.StdEncoding.EncodeToString(auditPrivate), Genesis: true,
	})
	if err != nil {
		return report, fmt.Errorf("export signed rehearsal audit package: %w", err)
	}
	auditVerification, err := audit.VerifyAuditPackage(audit.AuditVerifyOptions{Directory: auditDir, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(auditPublic)})
	if err != nil || !auditVerification.OK {
		return report, errors.New("verify signed rehearsal audit package")
	}
	substitutedAudit, err := audit.VerifyAuditPackage(audit.AuditVerifyOptions{Directory: auditDir, TrustedPublicKeyBase64: base64.StdEncoding.EncodeToString(recoveryPublic)})
	if err != nil {
		return report, errors.New("run rehearsal audit substitution check")
	}
	if substitutedAudit.OK {
		return report, errors.New("rehearsal audit key substitution was not rejected")
	}

	publicFiles := map[string]ed25519.PublicKey{
		"identity-public.key":          identityPublic,
		"identity-recovery-public.key": recoveryPublic,
		"audit-public.key":             auditPublic,
	}
	retained := []string{"audit-export/manifest.json", "audit-export/records.jsonl", "audit-export/signature.json", "audit-public.key", "identity-public.key", "identity-recovery-public.key", "report.json"}
	for name, publicKey := range publicFiles {
		if err := os.WriteFile(filepath.Join(out, name), []byte(base64.StdEncoding.EncodeToString(publicKey)+"\n"), 0o644); err != nil {
			return report, errors.New("write retained rehearsal public key")
		}
	}
	for _, file := range privateFiles {
		if err := os.Remove(filepath.Join(runDir, file.name)); err != nil {
			return report, errors.New("destroy rehearsal private key file")
		}
	}
	privateDestroyed := true
	for _, file := range privateFiles {
		if _, err := os.Lstat(filepath.Join(runDir, file.name)); !errors.Is(err, os.ErrNotExist) {
			privateDestroyed = false
		}
	}
	report = sovereignRehearsalReport{
		Schema: sovereignRehearsalSchema, OK: true, GeneratedAt: reportTime.Format(time.RFC3339), Project: opts.Project, WorkspaceType: "operator_tmpfs",
		IdentityKeyFingerprint: keyFingerprint(identityPublic), RecoveryKeyFingerprint: keyFingerprint(recoveryPublic), AuditKeyFingerprint: keyFingerprint(auditPublic),
		PositiveAuthorization: "pass", RoleRejection: "pass", RevocationRejection: "pass", KeyLossRejection: "pass", RecoveryAuthorization: "pass",
		IdentitySubstitutionRejected: true, AuditSignatureStatus: auditVerification.SignatureStatus, AuditSubstitutionRejected: true,
		PrivateFilesMode0600: privateModeOK, PrivateKeysDestroyed: privateDestroyed, RetainedFiles: retained,
		NonClaims:         []string{"not a production key ceremony", "not HSM-backed", "not FIPS validation", "not certification", "not deployment approval", "not risk acceptance"},
		AuthorityBoundary: "offline rehearsal evidence only; no credential, identity-provider, approval, deployment, release, public-exposure, or live-operation authority",
	}
	if !privateModeOK || !privateDestroyed {
		return report, errors.New("rehearsal private key lifecycle verification failed")
	}
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return report, errors.New("encode rehearsal report")
	}
	if err := os.WriteFile(filepath.Join(out, "report.json"), append(reportBytes, '\n'), 0o644); err != nil {
		return report, errors.New("write rehearsal report")
	}
	cleanupOutput = false
	return report, nil
}

func rehearsalClaims(cfg config.Config, now time.Time, subject, role, proofID string) identityproof.Claims {
	return identityproof.Claims{Issuer: cfg.Server.SovereignIssuer, Audience: cfg.Server.SovereignAudience, Subject: subject, Project: cfg.Fairway.ProjectName, Role: role, Purpose: "session", ProofID: proofID, IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix()}
}

func rehearsalAPIStatus(handler http.Handler, proof string) (int, string) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+proof)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func keyFingerprint(publicKey ed25519.PublicKey) string {
	digest := sha256.Sum256(publicKey)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func zeroRehearsalKey(key []byte) {
	for i := range key {
		key[i] = 0
	}
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func linuxMountForPath(path string) (string, string, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return "", "", errors.New("mount verification requires Linux /proc/self/mountinfo")
	}
	return mountForPathFromData(path, data)
}

func validateRehearsalMounts(workspaceMount, workspaceType, outputMount, outputType string) error {
	if workspaceType != "tmpfs" {
		return errors.New("rehearsal workspace must be mounted on tmpfs")
	}
	if workspaceMount == outputMount || outputType == "tmpfs" {
		return errors.New("retained rehearsal output must use a distinct non-tmpfs mount separate from the private tmpfs workspace")
	}
	return nil
}

func mountForPathFromData(path string, data []byte) (string, string, error) {
	path = filepath.Clean(path)
	bestMount := ""
	bestType := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		separator := -1
		for i, field := range fields {
			if field == "-" {
				separator = i
				break
			}
		}
		if separator < 0 || separator+1 >= len(fields) || len(fields) < 5 {
			continue
		}
		mount := strings.ReplaceAll(strings.ReplaceAll(fields[4], `\040`, " "), `\134`, `\`)
		if (path == mount || pathContains(mount, path)) && len(mount) >= len(bestMount) {
			bestMount = mount
			bestType = fields[separator+1]
		}
	}
	if bestMount == "" {
		return "", "", errors.New("cannot resolve rehearsal path mount type")
	}
	return bestMount, bestType, nil
}
