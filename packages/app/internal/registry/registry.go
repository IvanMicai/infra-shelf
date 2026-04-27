package registry

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var appNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type ServiceName string

const (
	Postgres ServiceName = "postgres"
	Redis    ServiceName = "redis"
	RabbitMQ ServiceName = "rabbitmq"
)

var ValidServices = map[string]ServiceName{
	"postgres": Postgres,
	"redis":    Redis,
	"rabbitmq": RabbitMQ,
}

type Registry struct {
	Version int                 `json:"version"`
	Apps    map[string]AppEntry `json:"apps"`
}

type encryptedRegistryFile struct {
	Version    int    `json:"version"`
	Encrypted  bool   `json:"encrypted"`
	Algorithm  string `json:"algorithm"`
	KDF        string `json:"kdf"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type AppEntry struct {
	CreatedAt string   `json:"createdAt"`
	Services  Services `json:"services"`
}

type Services struct {
	Postgres *PostgresConfig `json:"postgres,omitempty"`
	Redis    *RedisConfig    `json:"redis,omitempty"`
	RabbitMQ *RabbitMQConfig `json:"rabbitmq,omitempty"`
}

type PostgresConfig struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type RedisConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Prefix   string `json:"prefix"`
}

type RabbitMQConfig struct {
	Vhost    string `json:"vhost"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Store struct {
	Path string
}

type App struct {
	Name      string
	CreatedAt string
	Entry     AppEntry
}

type EnvBlock struct {
	Service string
	Body    string
}

func NewStore(path string) *Store {
	return &Store{Path: path}
}

func (s *Store) Load() (Registry, error) {
	content, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return Registry{Version: 1, Apps: map[string]AppEntry{}}, nil
	}
	if err != nil {
		return Registry{}, err
	}

	var encrypted encryptedRegistryFile
	if err := json.Unmarshal(content, &encrypted); err == nil && encrypted.Encrypted {
		content, err = decryptRegistryContent(encrypted, s.Path)
		if err != nil {
			return Registry{}, err
		}
	}

	var registry Registry
	if err := json.Unmarshal(content, &registry); err != nil {
		return Registry{}, err
	}
	if registry.Apps == nil {
		registry.Apps = map[string]AppEntry{}
	}
	return registry, nil
}

func decryptRegistryContent(file encryptedRegistryFile, path string) ([]byte, error) {
	if file.Algorithm != "AES-256-GCM" || file.KDF != "SHA-256" {
		return nil, fmt.Errorf("unsupported encrypted registry format: %s/%s", file.Algorithm, file.KDF)
	}

	secret, err := registrySecret(filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	payload, err := base64.StdEncoding.DecodeString(file.Ciphertext)
	if err != nil {
		return nil, err
	}
	if len(payload) <= 16 {
		return nil, errors.New("invalid encrypted registry payload")
	}

	nonce, err := base64.StdEncoding.DecodeString(file.Nonce)
	if err != nil {
		return nil, err
	}

	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid encrypted registry nonce")
	}

	return gcm.Open(nil, nonce, payload, nil)
}

func registrySecret(startDir string) (string, error) {
	for _, key := range []string{"INFRA_SHELF_SECRET", "INFRA_SHELF_REGISTRY_SECRET"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value, nil
		}
	}

	env := loadDotenv(startDir)
	for _, key := range []string{"INFRA_SHELF_SECRET", "INFRA_SHELF_REGISTRY_SECRET"} {
		if value := strings.TrimSpace(env[key]); value != "" {
			return value, nil
		}
	}

	return "", errors.New("INFRA_SHELF_SECRET is required to read the encrypted registry")
}

func loadDotenv(startDir string) map[string]string {
	env := map[string]string{}
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return env
	}

	for {
		content, err := os.ReadFile(filepath.Join(dir, ".env"))
		if err == nil {
			return parseDotenv(string(content))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return env
		}
		dir = parent
	}
}

func parseDotenv(content string) map[string]string {
	env := map[string]string{}
	for _, rawLine := range strings.Split(content, "\n") {
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

func (s *Store) ListApps() ([]App, error) {
	registry, err := s.Load()
	if err != nil {
		return nil, err
	}

	apps := make([]App, 0, len(registry.Apps))
	for name, entry := range registry.Apps {
		apps = append(apps, App{Name: name, CreatedAt: entry.CreatedAt, Entry: entry})
	}

	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Name < apps[j].Name
	})

	return apps, nil
}

func (s *Store) GetApp(name string) (App, bool, error) {
	registry, err := s.Load()
	if err != nil {
		return App{}, false, err
	}
	entry, ok := registry.Apps[name]
	if !ok {
		return App{}, false, nil
	}
	return App{Name: name, CreatedAt: entry.CreatedAt, Entry: entry}, true, nil
}

func ValidateAppName(name string) error {
	if !appNamePattern.MatchString(name) {
		return fmt.Errorf("invalid app name %q", name)
	}
	return nil
}

func ParseServices(values []string) ([]string, error) {
	seen := map[string]bool{}
	services := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := ValidServices[value]; !ok {
			return nil, fmt.Errorf("invalid service %q", value)
		}
		if !seen[value] {
			seen[value] = true
			services = append(services, value)
		}
	}

	return services, nil
}

func (a App) ServiceNames() []string {
	names := make([]string, 0, 3)
	if a.Entry.Services.Postgres != nil {
		names = append(names, string(Postgres))
	}
	if a.Entry.Services.Redis != nil {
		names = append(names, string(Redis))
	}
	if a.Entry.Services.RabbitMQ != nil {
		names = append(names, string(RabbitMQ))
	}
	return names
}

func (a App) CreatedAtTime() time.Time {
	created, err := time.Parse(time.RFC3339, a.CreatedAt)
	if err != nil {
		return time.Time{}
	}
	return created
}

func (a App) EnvBlocks() []EnvBlock {
	blocks := make([]EnvBlock, 0, 3)
	if cfg := a.Entry.Services.Postgres; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "PostgreSQL", Body: postgresEnv(*cfg)})
	}
	if cfg := a.Entry.Services.Redis; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "Redis", Body: redisEnv(*cfg)})
	}
	if cfg := a.Entry.Services.RabbitMQ; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "RabbitMQ", Body: rabbitMQEnv(*cfg)})
	}
	return blocks
}

func (a App) EnvFile() string {
	blocks := a.EnvBlocks()
	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		lines = append(lines, block.Body)
	}
	return strings.Join(lines, "\n\n")
}

func postgresEnv(cfg PostgresConfig) string {
	return strings.Join([]string{
		"# === PostgreSQL ===",
		fmt.Sprintf("DATABASE_URL=postgres://%s:%s@postgres:5432/%s", cfg.Username, cfg.Password, cfg.Database),
		"DB_HOST=postgres",
		"DB_PORT=5432",
		fmt.Sprintf("DB_USERNAME=%s", cfg.Username),
		fmt.Sprintf("DB_PASSWORD=%s", cfg.Password),
		fmt.Sprintf("DB_NAME=%s", cfg.Database),
	}, "\n")
}

func redisEnv(cfg RedisConfig) string {
	return strings.Join([]string{
		"# === Redis ===",
		fmt.Sprintf("REDIS_URL=redis://%s:%s@redis:6379/0", cfg.Username, cfg.Password),
		"REDIS_HOST=redis",
		"REDIS_PORT=6379",
		fmt.Sprintf("REDIS_USERNAME=%s", cfg.Username),
		fmt.Sprintf("REDIS_PASSWORD=%s", cfg.Password),
		fmt.Sprintf("REDIS_PREFIX=%s", cfg.Prefix),
	}, "\n")
}

func rabbitMQEnv(cfg RabbitMQConfig) string {
	return strings.Join([]string{
		"# === RabbitMQ ===",
		fmt.Sprintf("AMQP_URL=amqp://%s:%s@rabbitmq:5672/%s", cfg.Username, cfg.Password, url.PathEscape(cfg.Vhost)),
		"RABBITMQ_HOST=rabbitmq",
		"RABBITMQ_PORT=5672",
		fmt.Sprintf("RABBITMQ_USERNAME=%s", cfg.Username),
		fmt.Sprintf("RABBITMQ_PASSWORD=%s", cfg.Password),
		fmt.Sprintf("RABBITMQ_VHOST=%s", cfg.Vhost),
	}, "\n")
}
