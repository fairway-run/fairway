package secretscan

import "testing"

func TestContainsComprehensiveAssignmentsAndHeaders(t *testing.T) {
	for _, value := range []string{
		"secret=topsecret",
		"ssh_private_key: private-value",
		"AWS_SECRET_ACCESS_KEY=aws-value",
		"set-cookie: session=private-value",
		"authorization: Bearer private-value",
		"-----BEGIN OPENSSH PRIVATE KEY-----",
	} {
		if !Contains([]byte(value)) {
			t.Error("secret pattern was not detected")
		}
	}
}

func TestContainsAllowsClosedPlaceholders(t *testing.T) {
	for _, value := range []string{
		"secret=<redacted>",
		"ssh_private_key=${SSH_PRIVATE_KEY}",
		"AWS_SECRET_ACCESS_KEY=unset",
		"set-cookie: <masked>",
	} {
		if Contains([]byte(value)) {
			t.Errorf("closed placeholder was rejected: %q", value)
		}
	}
}
