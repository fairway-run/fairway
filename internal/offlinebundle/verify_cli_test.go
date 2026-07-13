package offlinebundle

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestRunVerifierCLIHelpAndMissingKey(t *testing.T) {
	var out bytes.Buffer
	err := RunVerifierCLI([]string{"--help"}, &out, nil)
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(out.String(), "usage: fairway-offline-verify") {
		t.Fatalf("help output=%q err=%v", out.String(), err)
	}
	out.Reset()
	err = RunVerifierCLI([]string{"--dir", "/tmp/bundle", "--trusted-public-key-env", "MISSING"}, &out, func(string) (string, bool) { return "", false })
	if err == nil || !strings.Contains(err.Error(), "environment variable MISSING is not set") {
		t.Fatalf("missing key error=%v", err)
	}
}
