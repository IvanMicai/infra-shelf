package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr                 string
	Username             string
	Password             string
	UsingDefaultPassword bool
	RootDir              string
	RegistryPath         string
	BackupsDir           string
	DatabasePath         string
	Timezone             string
	S3                   S3Config
}

type S3Config struct {
	Bucket          string
	Region          string
	Prefix          string
	Endpoint        string
	ForcePathStyle  bool
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func Load() (Config, error) {
	root, err := resolveRoot()
	if err != nil {
		return Config{}, err
	}
	env := loadDotenv(filepath.Join(root, ".env"))

	username := getenv(env, "APP_USERNAME", "admin")
	password := getenv(env, "APP_PASSWORD", "admin")

	cfg := Config{
		Addr:                 getenv(env, "APP_ADDR", "127.0.0.1:8080"),
		Username:             username,
		Password:             password,
		UsingDefaultPassword: username == "admin" && password == "admin",
		RootDir:              root,
		RegistryPath:         getenv(env, "INFRA_SHELF_REGISTRY_PATH", defaultRegistryPath(root)),
		BackupsDir:           getenv(env, "INFRA_SHELF_BACKUPS_DIR", filepath.Join(root, "backups")),
		DatabasePath:         getenv(env, "APP_DATABASE_PATH", filepath.Join(root, "data", "app", "infra-shelf-app.db")),
		Timezone:             getenv(env, "APP_TIMEZONE", "America/Sao_Paulo"),
		S3: S3Config{
			Bucket:          getenv(env, "BACKUP_S3_BUCKET", ""),
			Region:          getenv(env, "BACKUP_S3_REGION", getenv(env, "AWS_REGION", "us-east-1")),
			Prefix:          strings.Trim(getenv(env, "BACKUP_S3_PREFIX", "infra-shelf/backups"), "/"),
			Endpoint:        getenv(env, "BACKUP_S3_ENDPOINT", ""),
			ForcePathStyle:  getenvBool(env, "BACKUP_S3_FORCE_PATH_STYLE", false),
			AccessKeyID:     getenv(env, "AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getenv(env, "AWS_SECRET_ACCESS_KEY", ""),
			SessionToken:    getenv(env, "AWS_SESSION_TOKEN", ""),
		},
	}

	return cfg, nil
}

func resolveRoot() (string, error) {
	if root := os.Getenv("INFRA_SHELF_ROOT"); root != "" {
		return filepath.Abs(root)
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	wd, err = filepath.Abs(wd)
	if err != nil {
		return "", err
	}

	for dir := wd; ; dir = filepath.Dir(dir) {
		if exists(filepath.Join(dir, "docker-compose.yml")) && exists(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	return "", errors.New("could not find infra-shelf root; set INFRA_SHELF_ROOT")
}

// defaultRegistryPath prefers the legacy <root>/packages/cli/src/apps.json
// location when it still exists (so existing installs keep working without
// an env var). Otherwise falls back to <root>/data/apps.json.
func defaultRegistryPath(root string) string {
	legacy := filepath.Join(root, "packages", "cli", "src", "apps.json")
	if exists(legacy) {
		return legacy
	}
	return filepath.Join(root, "data", "apps.json")
}

func getenv(env map[string]string, key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if value := env[key]; value != "" {
		return value
	}
	return fallback
}

func getenvBool(env map[string]string, key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(getenv(env, key, "")))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func loadDotenv(path string) map[string]string {
	env := map[string]string{}
	content, err := os.ReadFile(path)
	if err != nil {
		return env
	}

	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
				(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
				value = value[1 : len(value)-1]
			}
		}
		env[strings.TrimSpace(key)] = value
	}

	return env
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
