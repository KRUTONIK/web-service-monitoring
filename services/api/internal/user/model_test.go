package user

import "testing"

func TestNormalizeEmail(t *testing.T) {
	if actual := NormalizeEmail("  User@Example.COM  "); actual != "user@example.com" {
		t.Fatalf("unexpected normalized email: %q", actual)
	}
}
