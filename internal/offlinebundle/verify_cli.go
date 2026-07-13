package offlinebundle

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// RunVerifierCLI implements the standalone verifier without loading Fairway
// configuration or state. The caller controls output and environment lookup so
// the command can be tested without exposing key material.
func RunVerifierCLI(args []string, out io.Writer, lookupEnv func(string) (string, bool)) error {
	if out == nil {
		out = io.Discard
	}
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	fs := flag.NewFlagSet("fairway-offline-verify", flag.ContinueOnError)
	fs.SetOutput(out)
	dir := fs.String("dir", "", "offline distribution bundle directory")
	keyEnv := fs.String("trusted-public-key-env", "", "environment variable containing the pinned Ed25519 public key")
	format := fs.String("format", "text", "output format: text or json")
	current := bindExpectedFlags(fs, "expected-")
	rollback := bindExpectedFlags(fs, "expected-rollback-")
	fs.Usage = func() {
		fmt.Fprintln(out, "usage: fairway-offline-verify --dir <path> --trusted-public-key-env <name> --expected-version <version> --expected-source-sha <sha> --expected-builder-id <id> --expected-policy-version <id> --expected-rollback-version <version> --expected-rollback-source-sha <sha> --expected-rollback-builder-id <id> --expected-rollback-policy-version <id> [--format text|json]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected offline verifier arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported offline verifier format %q", *format)
	}
	if strings.TrimSpace(*dir) == "" || strings.TrimSpace(*keyEnv) == "" {
		return errors.New("offline verifier requires --dir and --trusted-public-key-env")
	}
	key, ok := lookupEnv(*keyEnv)
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("trusted public key environment variable %s is not set", *keyEnv)
	}
	report, err := Verify(VerifyOptions{Directory: *dir, TrustedPublicKeyBase64: key, CurrentExpected: current.value(), RollbackExpected: rollback.value()})
	if err != nil {
		return err
	}
	if *format == "json" {
		if err := writeJSON(out, report); err != nil {
			return err
		}
	} else {
		writeText(out, report)
	}
	if !report.OK {
		return errors.New("offline bundle verification failed")
	}
	return nil
}

type expectedFlags struct {
	version *string
	source  *string
	builder *string
	policy  *string
}

func bindExpectedFlags(fs *flag.FlagSet, prefix string) expectedFlags {
	return expectedFlags{
		version: fs.String(prefix+"version", "", "exact release version"),
		source:  fs.String(prefix+"source-sha", "", "exact release source revision"),
		builder: fs.String(prefix+"builder-id", "", "exact release builder identity"),
		policy:  fs.String(prefix+"policy-version", "", "exact release policy version"),
	}
}

func (f expectedFlags) value() ReleaseIdentity {
	return ReleaseIdentity{Version: *f.version, SourceSHA: *f.source, BuilderID: *f.builder, PolicyVersion: *f.policy}
}

func writeJSON(out io.Writer, value any) error {
	data, err := stableJSON(value)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func writeText(out io.Writer, report Verification) {
	fmt.Fprintf(out, "offline_bundle_verify: %t\ncurrent_version: %s\ncurrent_source_sha: %s\nrollback_version: %s\nrollback_source_sha: %s\nsignature_status: %s\ninventory_status: %s\ncurrent_assurance_status: %s\nrollback_assurance_status: %s\nrequired_asset_class_status: %s\nfiles: %d\nauthority_boundary: %s\n", report.OK, report.Current.Version, report.Current.SourceSHA, report.Rollback.Version, report.Rollback.SourceSHA, report.SignatureStatus, report.InventoryStatus, report.CurrentAssuranceStatus, report.RollbackAssuranceStatus, report.RequiredAssetClassStatus, report.FileCount, report.AuthorityBoundary)
	for _, issue := range report.Issues {
		fmt.Fprintf(out, "issue: %s\n", issue)
	}
}
