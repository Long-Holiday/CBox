package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestColabCredentialsPresent(t *testing.T) {
	homeDir := t.TempDir()
	if colabCredentialsPresent(homeDir) {
		t.Fatal("missing token file reported as present")
	}
	tokenDir := filepath.Join(homeDir, ".config", "colab-cli")
	if err := os.MkdirAll(tokenDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		data string
		want bool
	}{
		{`{"refresh_token":"test-refresh-token"}`, true},
		{`{"token":"test-access-token"}`, true},
		{`{}`, false},
		{`not json`, false},
	} {
		if err := os.WriteFile(filepath.Join(tokenDir, "token.json"), []byte(tc.data), 0600); err != nil {
			t.Fatal(err)
		}
		if got := colabCredentialsPresent(homeDir); got != tc.want {
			t.Fatalf("got %v, want %v for %s", got, tc.want, tc.data)
		}
	}
}
