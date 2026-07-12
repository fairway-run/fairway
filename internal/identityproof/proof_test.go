package identityproof

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifySovereignIdentityProof(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	revocationFile := filepath.Join(t.TempDir(), "revocations.json")
	writeRevocations(t, revocationFile, Revocations{Schema: RevocationSchema})
	t.Setenv("FAIRWAY_TEST_IDENTITY_PUBLIC_KEY", base64.StdEncoding.EncodeToString(public))
	cfg := testProofConfig(revocationFile)
	claims := validSessionClaims(now)
	token := signProof(t, private, cfg.KeyID, claims)
	proof, err := Verify(token, cfg, now, nil)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if proof.Claims.Subject != claims.Subject || proof.Claims.Project != cfg.Project || proof.KeyID != cfg.KeyID {
		t.Fatalf("Verify() proof = %+v", proof)
	}

	tests := []struct {
		name   string
		mutate func(*Claims)
		want   string
	}{
		{name: "wrong issuer", mutate: func(c *Claims) { c.Issuer = "other" }, want: "scope mismatch"},
		{name: "wrong project", mutate: func(c *Claims) { c.Project = "other" }, want: "scope mismatch"},
		{name: "future", mutate: func(c *Claims) { c.NotBefore = now.Add(time.Minute).Unix() }, want: "not active"},
		{name: "expired", mutate: func(c *Claims) {
			c.IssuedAt = now.Add(-10 * time.Minute).Unix()
			c.NotBefore = c.IssuedAt
			c.ExpiresAt = now.Add(-time.Minute).Unix()
		}, want: "expired"},
		{name: "stale", mutate: func(c *Claims) {
			c.IssuedAt = now.Add(-2 * time.Hour).Unix()
			c.NotBefore = c.IssuedAt
			c.ExpiresAt = now.Add(time.Minute).Unix()
		}, want: "bounded session"},
		{name: "unknown purpose", mutate: func(c *Claims) { c.Purpose = "root" }, want: "purpose"},
		{name: "break glass role", mutate: func(c *Claims) { c.Purpose = "break_glass"; c.Reason = "recovery"; c.Role = "operator" }, want: "break-glass"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := claims
			tc.mutate(&candidate)
			_, err := Verify(signProof(t, private, cfg.KeyID, candidate), cfg, now, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Verify() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestVerifySovereignIdentityProofFailsClosed(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	revocationFile := filepath.Join(t.TempDir(), "revocations.json")
	writeRevocations(t, revocationFile, Revocations{Schema: RevocationSchema})
	t.Setenv("FAIRWAY_TEST_IDENTITY_PUBLIC_KEY", base64.StdEncoding.EncodeToString(public))
	cfg := testProofConfig(revocationFile)
	claims := validSessionClaims(now)
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}

	otherPublic, otherPrivate, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = otherPublic
	for name, token := range map[string]string{
		"malformed":           "not-a-proof",
		"bad signature":       signProof(t, otherPrivate, cfg.KeyID, claims),
		"bad key id":          signProof(t, private, "other", claims),
		"algorithm confusion": signHeaderPayload(t, private, []byte(`{"alg":"none","typ":"JWT","kid":"customer-key-1"}`), claimsJSON),
		"unknown claim":       signRawProof(t, private, cfg.KeyID, map[string]any{"iss": cfg.Issuer, "unexpected": "value"}),
		"duplicate claim":     signRawJSONProof(t, private, cfg.KeyID, []byte(`{"iss":"one","iss":"two"}`)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Verify(token, cfg, now, nil); err == nil || strings.Contains(err.Error(), token) {
				t.Fatalf("Verify() error = %v", err)
			}
		})
	}

	writeRevocations(t, revocationFile, Revocations{Schema: RevocationSchema, RevokedProofIDs: []string{claims.ProofID}})
	if _, err := Verify(signProof(t, private, cfg.KeyID, claims), cfg, now, nil); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("revoked proof error = %v", err)
	}
	if err := os.WriteFile(revocationFile, []byte(`{"schema":"wrong"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(signProof(t, private, cfg.KeyID, claims), cfg, now, nil); err == nil || !strings.Contains(err.Error(), "revocation state") {
		t.Fatalf("invalid revocation state error = %v", err)
	}
	writeRevocations(t, revocationFile, Revocations{Schema: RevocationSchema})
	symlink := filepath.Join(t.TempDir(), "runtime-revocations-link.json")
	if err := os.Symlink(revocationFile, symlink); err != nil {
		t.Fatal(err)
	}
	symlinkCfg := cfg
	symlinkCfg.RevocationFile = symlink
	if _, err := Verify(signProof(t, private, cfg.KeyID, claims), symlinkCfg, now, nil); err == nil || !strings.Contains(err.Error(), "revocation state") {
		t.Fatalf("symlink revocation state error = %v", err)
	}
}

func TestVerifyDualControlAndBreakGlassProofs(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	revocationFile := filepath.Join(t.TempDir(), "revocations.json")
	writeRevocations(t, revocationFile, Revocations{Schema: RevocationSchema})
	t.Setenv("FAIRWAY_TEST_IDENTITY_PUBLIC_KEY", base64.StdEncoding.EncodeToString(public))
	cfg := testProofConfig(revocationFile)

	dual := Claims{Issuer: cfg.Issuer, Audience: cfg.Audience, Subject: "control@example.test", Project: cfg.Project, Role: "authorizer", Purpose: "dual_control", ProofID: "dual-1", IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), Command: "set:status", TaskID: "T-001", PrimaryProofID: "session-1", IdempotencyKey: "status-1", PayloadSHA256: strings.Repeat("a", 64)}
	if _, err := Verify(signProof(t, private, cfg.KeyID, dual), cfg, now, nil); err != nil {
		t.Fatalf("dual control Verify() error = %v", err)
	}
	dual.IdempotencyKey = ""
	if _, err := Verify(signProof(t, private, cfg.KeyID, dual), cfg, now, nil); err == nil || !strings.Contains(err.Error(), "dual-control") {
		t.Fatalf("unbound dual control error = %v", err)
	}

	breakGlass := validSessionClaims(now)
	breakGlass.Purpose = "break_glass"
	breakGlass.Role = "admin"
	breakGlass.Reason = "customer incident recovery"
	breakGlass.ExpiresAt = now.Add(2 * time.Minute).Unix()
	if _, err := Verify(signProof(t, private, cfg.KeyID, breakGlass), cfg, now, nil); err != nil {
		t.Fatalf("break-glass Verify() error = %v", err)
	}
	if _, err := Verify(signProof(t, private, cfg.KeyID, breakGlass), cfg, now.Add(10*time.Minute), nil); err == nil || (!strings.Contains(err.Error(), "expired") && !strings.Contains(err.Error(), "bounded")) {
		t.Fatalf("expired break-glass error = %v", err)
	}
}

func TestValidateConfigAndRevocationFile(t *testing.T) {
	public, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	revocationFile := filepath.Join(t.TempDir(), "revocations.json")
	writeRevocations(t, revocationFile, Revocations{Schema: RevocationSchema})
	cfg := testProofConfig(revocationFile)
	lookup := func(name string) (string, bool) {
		if name == cfg.PublicKeyEnv {
			return base64.StdEncoding.EncodeToString(public), true
		}
		return "", false
	}
	if err := ValidateConfig(cfg, lookup, os.ReadFile, os.Lstat); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}

	bad := cfg
	bad.RevocationFile = "relative.json"
	if err := ValidateConfig(bad, lookup, os.ReadFile, os.Lstat); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative revocation error = %v", err)
	}
	if err := os.Chmod(revocationFile, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(cfg, lookup, os.ReadFile, os.Lstat); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("public revocation permissions error = %v", err)
	}
	if err := os.Chmod(revocationFile, 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(t.TempDir(), "revocations-link.json")
	if err := os.Symlink(revocationFile, symlink); err != nil {
		t.Fatal(err)
	}
	bad = cfg
	bad.RevocationFile = symlink
	if err := ValidateConfig(bad, lookup, os.ReadFile, os.Lstat); err == nil || !strings.Contains(err.Error(), "private regular") {
		t.Fatalf("symlink revocation error = %v", err)
	}
}

func testProofConfig(revocationFile string) Config {
	return Config{PublicKeyEnv: "FAIRWAY_TEST_IDENTITY_PUBLIC_KEY", KeyID: "customer-key-1", Issuer: "https://identity.example.test", Audience: "fairway", Project: "fairway-test", RevocationFile: revocationFile, SessionMaxSeconds: 900, ClockSkewSeconds: 0, BreakGlassMaxSeconds: 300}
}

func validSessionClaims(now time.Time) Claims {
	return Claims{Issuer: "https://identity.example.test", Audience: "fairway", Subject: "operator@example.test", Project: "fairway-test", Role: "viewer", Purpose: "session", ProofID: "session-1", IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix()}
}

func signProof(t *testing.T, private ed25519.PrivateKey, keyID string, claims Claims) string {
	t.Helper()
	return signRawProof(t, private, keyID, claims)
}

func signRawProof(t *testing.T, private ed25519.PrivateKey, keyID string, claims any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return signRawJSONProof(t, private, keyID, payload)
}

func signRawJSONProof(t *testing.T, private ed25519.PrivateKey, keyID string, payload []byte) string {
	t.Helper()
	header, err := json.Marshal(proofHeader{Algorithm: ProofAlgorithm, Type: ProofType, KeyID: keyID})
	if err != nil {
		t.Fatal(err)
	}
	return signHeaderPayload(t, private, header, payload)
}

func signHeaderPayload(t *testing.T, private ed25519.PrivateKey, header, payload []byte) string {
	t.Helper()
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte(signingInput)))
}

func writeRevocations(t *testing.T, path string, revocations Revocations) {
	t.Helper()
	data, err := json.Marshal(revocations)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
