package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectService(t *testing.T) {
	cases := map[string]string{
		"postgres_20260517T193045.sql":    "postgres",
		"redis_20260517T193045.json":      "redis",
		"rabbitmq_20260517T193045.json":   "rabbitmq",
		"aistor_20260517T193045.tar":      "aistor",
		"mongodb_20260517T193045.archive": "mongodb",
		"mongodb_20260517T193045.tar":     "", // wrong extension
		"unknown_20260517T193045.bin":     "",
	}
	for name, want := range cases {
		if got := DetectService(name); got != want {
			t.Errorf("DetectService(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestPruneByDaysAndCount(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	files := map[string]time.Time{
		"postgres_new.sql":    now,
		"postgres_middle.sql": now.AddDate(0, 0, -10),
		"postgres_old.sql":    now.AddDate(0, 0, -40),
	}
	for name, modTime := range files {
		path := filepath.Join(appDir, name)
		if err := os.WriteFile(path, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := Prune(dir, PruneOptions{
		AppName:   "demo",
		Services:  []string{"postgres"},
		KeepDays:  30,
		KeepCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(deleted) != 1 || deleted[0].Name != "postgres_old.sql" {
		t.Fatalf("expected old backup to be deleted, got %#v", deleted)
	}
	if _, err := os.Stat(filepath.Join(appDir, "postgres_old.sql")); !os.IsNotExist(err) {
		t.Fatalf("expected old backup to be removed")
	}
}
