package backup

import (
	"os"
	"path/filepath"
	"reflect"
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

func sampleFiles() []File {
	return []File{
		{App: "web", Service: "postgres", Name: "postgres_1.sql"},
		{App: "web", Service: "redis", Name: "redis_1.json"},
		{App: "api", Service: "postgres", Name: "postgres_2.sql"},
		{App: "api", Service: "mongodb", Name: "mongodb_1.archive"},
	}
}

func TestFilter(t *testing.T) {
	files := sampleFiles()
	cases := []struct {
		name    string
		app     string
		service string
		want    []string // expected file names
	}{
		{"no constraint", "", "", []string{"postgres_1.sql", "redis_1.json", "postgres_2.sql", "mongodb_1.archive"}},
		{"app only", "web", "", []string{"postgres_1.sql", "redis_1.json"}},
		{"service only", "", "postgres", []string{"postgres_1.sql", "postgres_2.sql"}},
		{"app and service", "api", "postgres", []string{"postgres_2.sql"}},
		{"no match", "api", "redis", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Filter(files, tc.app, tc.service)
			names := make([]string, 0, len(got))
			for _, f := range got {
				names = append(names, f.Name)
			}
			if !reflect.DeepEqual(names, tc.want) {
				t.Errorf("Filter(%q, %q) names = %v, want %v", tc.app, tc.service, names, tc.want)
			}
		})
	}
}

func TestApps(t *testing.T) {
	got := Apps(sampleFiles())
	want := []string{"api", "web"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Apps() = %v, want %v", got, want)
	}
	if got := Apps(nil); len(got) != 0 {
		t.Errorf("Apps(nil) = %v, want empty", got)
	}
}

func TestServices(t *testing.T) {
	got := Services(sampleFiles())
	want := []string{"mongodb", "postgres", "redis"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Services() = %v, want %v", got, want)
	}
	if got := Services(nil); len(got) != 0 {
		t.Errorf("Services(nil) = %v, want empty", got)
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
