package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

func TestRunBackup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "proxy.db")
	s, err := storage.New(src)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	_ = s.Close()
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	var out, errOut bytes.Buffer

	dest := filepath.Join(dir, "backup.db")
	if code := runBackup([]string{dest}, env(map[string]string{"DATABASE_PATH": src}), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), dest) {
		t.Errorf("stdout = %q", out.String())
	}
	if code := runBackup([]string{dest}, env(map[string]string{"DATABASE_PATH": src}), &out, &errOut); code != 1 {
		t.Errorf("existing target: exit %d, want 1", code)
	}
	if code := runBackup(nil, env(nil), &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage") {
		t.Errorf("no target: exit %d, stderr %q", code, errOut.String())
	}
	// Default path /data/proxy.db does not exist in tests: a clear failure.
	if code := runBackup([]string{filepath.Join(dir, "x.db")}, env(nil), &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "/data/proxy.db") {
		t.Errorf("default path: exit %d, stderr %q", code, errOut.String())
	}
}
