package identityproof

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	RevocationSchema = "fairway.sovereign-revocations.v1"
	ProofType        = "JWT"
	ProofAlgorithm   = "EdDSA"
	maxProofBytes    = 16 * 1024
)

type Config struct {
	PublicKeyEnv         string
	KeyID                string
	Issuer               string
	Audience             string
	Project              string
	RevocationFile       string
	SessionMaxSeconds    int
	ClockSkewSeconds     int
	BreakGlassMaxSeconds int
}

type Claims struct {
	Issuer         string `json:"iss"`
	Audience       string `json:"aud"`
	Subject        string `json:"sub"`
	Project        string `json:"project_id"`
	Role           string `json:"role"`
	Purpose        string `json:"purpose"`
	ProofID        string `json:"jti"`
	IssuedAt       int64  `json:"iat"`
	NotBefore      int64  `json:"nbf"`
	ExpiresAt      int64  `json:"exp"`
	Command        string `json:"command,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	PrimaryProofID string `json:"primary_jti,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	PayloadSHA256  string `json:"payload_sha256,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

type Proof struct {
	Claims Claims
	KeyID  string
}

type Revocations struct {
	Schema          string   `json:"schema"`
	RevokedProofIDs []string `json:"revoked_proof_ids,omitempty"`
	RevokedSubjects []string `json:"revoked_subjects,omitempty"`
	RevokedKeyIDs   []string `json:"revoked_key_ids,omitempty"`
}

type proofHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

func ValidateConfig(cfg Config, lookupEnv func(string) (string, bool), readFile func(string) ([]byte, error), stat func(string) (os.FileInfo, error)) error {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	if readFile == nil {
		readFile = os.ReadFile
	}
	if stat == nil {
		stat = os.Lstat
	}
	for label, value := range map[string]string{
		"public_key_env":  cfg.PublicKeyEnv,
		"key_id":          cfg.KeyID,
		"issuer":          cfg.Issuer,
		"audience":        cfg.Audience,
		"project":         cfg.Project,
		"revocation_file": cfg.RevocationFile,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("sovereign identity %s is required", label)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("sovereign identity %s must be a single line", label)
		}
	}
	if cfg.SessionMaxSeconds < 60 || cfg.SessionMaxSeconds > 24*60*60 {
		return errors.New("sovereign identity session_max_seconds must be between 60 and 86400")
	}
	if cfg.ClockSkewSeconds < 0 || cfg.ClockSkewSeconds > 300 {
		return errors.New("sovereign identity clock_skew_seconds must be between 0 and 300")
	}
	if cfg.BreakGlassMaxSeconds < 30 || cfg.BreakGlassMaxSeconds > 15*60 || cfg.BreakGlassMaxSeconds > cfg.SessionMaxSeconds {
		return errors.New("sovereign identity break_glass_max_seconds must be between 30 and 900 and no greater than session_max_seconds")
	}
	encodedKey, ok := lookupEnv(cfg.PublicKeyEnv)
	if !ok || strings.TrimSpace(encodedKey) == "" {
		return fmt.Errorf("sovereign identity public key environment variable %s is not set", cfg.PublicKeyEnv)
	}
	if _, err := DecodePublicKey(encodedKey); err != nil {
		return errors.New("sovereign identity public key is invalid")
	}
	if !filepath.IsAbs(cfg.RevocationFile) {
		return errors.New("sovereign identity revocation_file must be an absolute local path")
	}
	info, err := stat(cfg.RevocationFile)
	if err != nil {
		return errors.New("sovereign identity revocation_file is unavailable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("sovereign identity revocation_file must be a private regular file with no group or other permissions")
	}
	data, err := readFile(cfg.RevocationFile)
	if err != nil {
		return errors.New("sovereign identity revocation_file is unavailable")
	}
	if _, err := ParseRevocations(data); err != nil {
		return fmt.Errorf("sovereign identity revocation_file is invalid: %w", err)
	}
	return nil
}

func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("invalid Ed25519 public key")
	}
	return ed25519.PublicKey(raw), nil
}

func ParseRevocations(data []byte) (Revocations, error) {
	var revocations Revocations
	if err := decodeStrict(data, &revocations); err != nil {
		return Revocations{}, errors.New("invalid revocation document")
	}
	if revocations.Schema != RevocationSchema {
		return Revocations{}, errors.New("unsupported revocation schema")
	}
	for label, values := range map[string][]string{
		"revoked_proof_ids": revocations.RevokedProofIDs,
		"revoked_subjects":  revocations.RevokedSubjects,
		"revoked_key_ids":   revocations.RevokedKeyIDs,
	} {
		seen := map[string]bool{}
		for _, value := range values {
			if strings.TrimSpace(value) == "" || len(value) > 512 || strings.ContainsAny(value, "\r\n") {
				return Revocations{}, fmt.Errorf("%s contains an invalid value", label)
			}
			if seen[value] {
				return Revocations{}, fmt.Errorf("%s contains a duplicate value", label)
			}
			seen[value] = true
		}
	}
	return revocations, nil
}

func Verify(token string, cfg Config, now time.Time, readFile func(string) ([]byte, error)) (Proof, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > maxProofBytes {
		return Proof{}, errors.New("invalid_identity: signed proof is missing or too large")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Proof{}, errors.New("invalid_identity: signed proof is malformed")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Proof{}, errors.New("invalid_identity: signed proof is malformed")
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Proof{}, errors.New("invalid_identity: signed proof is malformed")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return Proof{}, errors.New("invalid_identity: signed proof is malformed")
	}
	var header proofHeader
	var claims Claims
	if decodeStrict(headerBytes, &header) != nil || decodeStrict(claimsBytes, &claims) != nil {
		return Proof{}, errors.New("invalid_identity: signed proof schema is invalid")
	}
	if header.Algorithm != ProofAlgorithm || header.Type != ProofType || header.KeyID != cfg.KeyID {
		return Proof{}, errors.New("invalid_identity: signed proof header is invalid")
	}
	encodedKey, ok := os.LookupEnv(cfg.PublicKeyEnv)
	if !ok {
		return Proof{}, errors.New("invalid_identity: verification key is unavailable")
	}
	publicKey, err := DecodePublicKey(encodedKey)
	if err != nil || !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return Proof{}, errors.New("invalid_identity: signed proof verification failed")
	}
	if err := validateClaims(claims, cfg, now); err != nil {
		return Proof{}, err
	}
	var revocationBytes []byte
	if readFile == nil {
		revocationBytes, err = readPrivateRegularFile(cfg.RevocationFile)
	} else {
		revocationBytes, err = readFile(cfg.RevocationFile)
	}
	if err != nil {
		return Proof{}, errors.New("invalid_identity: revocation state is unavailable")
	}
	revocations, err := ParseRevocations(revocationBytes)
	if err != nil {
		return Proof{}, errors.New("invalid_identity: revocation state is invalid")
	}
	if contains(revocations.RevokedProofIDs, claims.ProofID) || contains(revocations.RevokedSubjects, claims.Subject) || contains(revocations.RevokedKeyIDs, header.KeyID) {
		return Proof{}, errors.New("invalid_identity: signed proof is revoked")
	}
	return Proof{Claims: claims, KeyID: header.KeyID}, nil
}

func validateClaims(claims Claims, cfg Config, now time.Time) error {
	if claims.Issuer != cfg.Issuer || claims.Audience != cfg.Audience || claims.Project != cfg.Project {
		return errors.New("invalid_identity: signed proof scope mismatch")
	}
	for _, value := range []string{claims.Subject, claims.Role, claims.Purpose, claims.ProofID} {
		if strings.TrimSpace(value) == "" || len(value) > 512 || strings.ContainsAny(value, "\r\n") {
			return errors.New("invalid_identity: signed proof claims are invalid")
		}
	}
	if claims.IssuedAt <= 0 || claims.NotBefore <= 0 || claims.ExpiresAt <= 0 || claims.ExpiresAt <= claims.IssuedAt {
		return errors.New("invalid_identity: signed proof time claims are invalid")
	}
	skew := int64(cfg.ClockSkewSeconds)
	nowUnix := now.UTC().Unix()
	if claims.IssuedAt > nowUnix+skew || claims.NotBefore > nowUnix+skew {
		return errors.New("invalid_identity: signed proof is not active")
	}
	if claims.ExpiresAt <= nowUnix-skew {
		return errors.New("invalid_identity: signed proof is expired")
	}
	maxSeconds := int64(cfg.SessionMaxSeconds)
	switch claims.Purpose {
	case "session":
		if claims.Command != "" || claims.TaskID != "" || claims.PrimaryProofID != "" || claims.IdempotencyKey != "" || claims.PayloadSHA256 != "" || claims.Reason != "" {
			return errors.New("invalid_identity: session proof contains unsupported authority claims")
		}
	case "break_glass":
		maxSeconds = int64(cfg.BreakGlassMaxSeconds)
		if claims.Role != "admin" || strings.TrimSpace(claims.Reason) == "" || claims.Command != "" || claims.TaskID != "" || claims.PrimaryProofID != "" || claims.IdempotencyKey != "" || claims.PayloadSHA256 != "" {
			return errors.New("invalid_identity: break-glass proof claims are invalid")
		}
	case "dual_control":
		maxSeconds = int64(cfg.BreakGlassMaxSeconds)
		if claims.Role != "authorizer" || strings.TrimSpace(claims.Command) == "" || strings.TrimSpace(claims.TaskID) == "" || strings.TrimSpace(claims.PrimaryProofID) == "" || strings.TrimSpace(claims.IdempotencyKey) == "" || !validSHA256(claims.PayloadSHA256) || claims.Reason != "" {
			return errors.New("invalid_identity: dual-control proof claims are invalid")
		}
	default:
		return errors.New("invalid_identity: signed proof purpose is unsupported")
	}
	if claims.ExpiresAt-claims.IssuedAt > maxSeconds || nowUnix-claims.IssuedAt > maxSeconds+skew {
		return errors.New("invalid_identity: signed proof exceeds its bounded session lifetime")
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func decodeStrict(data []byte, dest any) error {
	if err := rejectDuplicateObjectKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}

func rejectDuplicateObjectKeys(data []byte) error {
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
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}

func readPrivateRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 1024*1024 {
		return nil, errors.New("revocation file is not a bounded private regular file")
	}
	return os.ReadFile(path)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
