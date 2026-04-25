package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var safeNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type File struct {
	App      string
	Service  string
	Name     string
	Path     string
	Size     int64
	Modified time.Time
}

type Snapshot map[string]File

type PruneOptions struct {
	AppName   string
	All       bool
	Services  []string
	KeepDays  int
	KeepCount int
}

func List(dir string) ([]File, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	files := []File{}
	for _, entry := range entries {
		if !entry.IsDir() || !safeNamePattern.MatchString(entry.Name()) {
			continue
		}
		app := entry.Name()
		appDir := filepath.Join(dir, app)
		appFiles, err := os.ReadDir(appDir)
		if err != nil {
			continue
		}
		for _, appFile := range appFiles {
			if appFile.IsDir() {
				continue
			}
			service := DetectService(appFile.Name())
			if service == "" {
				continue
			}
			info, err := appFile.Info()
			if err != nil {
				continue
			}
			files = append(files, File{
				App:      app,
				Service:  service,
				Name:     appFile.Name(),
				Path:     filepath.Join(appDir, appFile.Name()),
				Size:     info.Size(),
				Modified: info.ModTime(),
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Modified.After(files[j].Modified)
	})

	return files, nil
}

func TakeSnapshot(dir string) (Snapshot, error) {
	files, err := List(dir)
	if err != nil {
		return nil, err
	}
	snapshot := Snapshot{}
	for _, file := range files {
		snapshot[file.Path] = file
	}
	return snapshot, nil
}

func Diff(before Snapshot, after []File) []File {
	files := []File{}
	for _, file := range after {
		if _, ok := before[file.Path]; !ok {
			files = append(files, file)
		}
	}
	return files
}

func ListForApp(dir, app string) ([]File, error) {
	files, err := List(dir)
	if err != nil {
		return nil, err
	}
	filtered := make([]File, 0)
	for _, file := range files {
		if file.App == app {
			filtered = append(filtered, file)
		}
	}
	return filtered, nil
}

func Prune(dir string, options PruneOptions) ([]File, error) {
	if options.KeepDays <= 0 && options.KeepCount <= 0 {
		return nil, nil
	}

	files, err := List(dir)
	if err != nil {
		return nil, err
	}

	serviceSet := map[string]bool{}
	for _, service := range options.Services {
		serviceSet[service] = true
	}

	groups := map[string][]File{}
	for _, file := range files {
		if !options.All && options.AppName != "" && file.App != options.AppName {
			continue
		}
		if len(serviceSet) > 0 && !serviceSet[file.Service] {
			continue
		}
		key := file.App + "/" + file.Service
		groups[key] = append(groups[key], file)
	}

	cutoff := time.Time{}
	if options.KeepDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -options.KeepDays)
	}

	deleted := []File{}
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			return group[i].Modified.After(group[j].Modified)
		})

		for index, file := range group {
			deleteByAge := !cutoff.IsZero() && file.Modified.Before(cutoff)
			deleteByCount := options.KeepCount > 0 && index >= options.KeepCount
			if !deleteByAge && !deleteByCount {
				continue
			}
			if err := os.Remove(file.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return deleted, err
			}
			deleted = append(deleted, file)
		}
	}

	return deleted, nil
}

func Resolve(dir, app, name string) (string, error) {
	if !safeNamePattern.MatchString(app) {
		return "", fmt.Errorf("invalid app %q", app)
	}
	if filepath.Base(name) != name || DetectService(name) == "" {
		return "", fmt.Errorf("invalid backup file %q", name)
	}

	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(filepath.Join(root, app, name))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || rel == "." {
		return "", fmt.Errorf("backup path escapes root")
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func DetectService(name string) string {
	switch {
	case strings.HasPrefix(name, "postgres_") && strings.HasSuffix(name, ".sql"):
		return "postgres"
	case strings.HasPrefix(name, "redis_") && strings.HasSuffix(name, ".json"):
		return "redis"
	case strings.HasPrefix(name, "rabbitmq_") && strings.HasSuffix(name, ".json"):
		return "rabbitmq"
	default:
		return ""
	}
}
