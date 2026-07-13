package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSovereignCustomerKeyRehearsalExercisesAndDestroysKeys(t *testing.T) {
	workspace := t.TempDir()
	outParent := t.TempDir()
	out := filepath.Join(outParent, "retained")
	report, err := runSovereignCustomerKeyRehearsal(context.Background(), sovereignRehearsalRunOptions{
		Workspace: workspace, Output: out, Project: "fairway-rehearsal", GeneratedAt: time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC), RequireTmpfs: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || !report.PrivateFilesMode0600 || !report.PrivateKeysDestroyed || !report.IdentitySubstitutionRejected || !report.AuditSubstitutionRejected {
		t.Fatalf("report = %+v", report)
	}
	if report.PositiveAuthorization != "pass" || report.RoleRejection != "pass" || report.RevocationRejection != "pass" || report.KeyLossRejection != "pass" || report.RecoveryAuthorization != "pass" || report.AuditSignatureStatus != "verified_pinned" {
		t.Fatalf("rehearsal checks = %+v", report)
	}
	if report.IdentityKeyFingerprint == report.RecoveryKeyFingerprint || report.IdentityKeyFingerprint == report.AuditKeyFingerprint || report.RecoveryKeyFingerprint == report.AuditKeyFingerprint {
		t.Fatalf("rehearsal roots are not distinct: %+v", report)
	}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("private rehearsal workspace remains: %+v", entries)
	}
	for _, name := range []string{"report.json", "identity-public.key", "identity-recovery-public.key", "audit-public.key", "audit-export/manifest.json", "audit-export/signature.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("missing retained file %s: %v", name, err)
		}
	}
	reportBytes, err := os.ReadFile(filepath.Join(out, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded sovereignRehearsalReport
	if err := json.Unmarshal(reportBytes, &decoded); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(reportBytes)), "private.key") || strings.Contains(string(reportBytes), "Authorization") {
		t.Fatalf("retained report exposes private path or credential header: %s", reportBytes)
	}
}

func TestCLI_SecurityRehearsalHelpAndTmpfsBoundary(t *testing.T) {
	help := runCapture(t, "security", "rehearsal", "--help")
	assertContains(t, help, "fairway security rehearsal run")
	assertContains(t, help, "private keys remain in tmpfs")
	if _, err := captureRun("security", "rehearsal", "run", "--workspace", t.TempDir(), "--out", filepath.Join(t.TempDir(), "retained"), "--project", "fairway", "--at", time.Now().UTC().Format(time.RFC3339)); err == nil || (!strings.Contains(err.Error(), "tmpfs") && !strings.Contains(err.Error(), "/proc/self/mountinfo")) {
		t.Fatalf("non-tmpfs rehearsal error = %v", err)
	}
}

func TestSovereignCustomerKeyRehearsalRejectsOutputInsideWorkspaceThroughSymlink(t *testing.T) {
	workspace := t.TempDir()
	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "workspace-link")
	if err := os.Symlink(workspace, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := runSovereignCustomerKeyRehearsal(context.Background(), sovereignRehearsalRunOptions{
		Workspace: workspace, Output: filepath.Join(link, "retained"), Project: "fairway-rehearsal", GeneratedAt: time.Now().UTC(), RequireTmpfs: false,
	})
	if err == nil || !strings.Contains(err.Error(), "separate paths") {
		t.Fatalf("symlink-contained output error = %v", err)
	}
}

func TestMountForPathFromDataUsesMostSpecificMount(t *testing.T) {
	mountInfo := []byte("36 25 0:32 / / rw,relatime - overlay overlay rw\n37 36 0:33 / /run rw,nosuid,nodev - tmpfs tmpfs rw\n38 36 0:34 / /artifacts rw,relatime - ext4 /dev/test rw\n")
	mount, fsType, err := mountForPathFromData("/run/fairway/private", mountInfo)
	if err != nil {
		t.Fatal(err)
	}
	if mount != "/run" || fsType != "tmpfs" {
		t.Fatalf("private mount = %q %q", mount, fsType)
	}
	mount, fsType, err = mountForPathFromData("/artifacts/report", mountInfo)
	if err != nil {
		t.Fatal(err)
	}
	if mount != "/artifacts" || fsType != "ext4" {
		t.Fatalf("retained mount = %q %q", mount, fsType)
	}
	if err := validateRehearsalMounts("/run", "tmpfs", "/run", "tmpfs"); err == nil || !strings.Contains(err.Error(), "separate") {
		t.Fatalf("same mount error = %v", err)
	}
	if err := validateRehearsalMounts("/run", "tmpfs", "/artifacts", "tmpfs"); err == nil || !strings.Contains(err.Error(), "non-tmpfs") {
		t.Fatalf("second tmpfs error = %v", err)
	}
	if err := validateRehearsalMounts("/run", "tmpfs", "/artifacts", "ext4"); err != nil {
		t.Fatalf("separate mount error = %v", err)
	}
}
