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
	AIStor   ServiceName = "aistor"
	MongoDB  ServiceName = "mongodb"
	Signoz   ServiceName = "signoz"
)

var ValidServices = map[string]ServiceName{
	"postgres": Postgres,
	"redis":    Redis,
	"rabbitmq": RabbitMQ,
	"aistor":   AIStor,
	"mongodb":  MongoDB,
	"signoz":   Signoz,
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
	CreatedAt string `json:"createdAt"`
	// Set at setup time when --envs / --env is used. Survives addon
	// detach/reattach so re-attaching SignOz via the UI preserves the env.
	Environment       string   `json:"environment,omitempty"`
	SignozServiceName string   `json:"signozServiceName,omitempty"`
	Services          Services `json:"services"`
}

type Services struct {
	Postgres *PostgresConfig `json:"postgres,omitempty"`
	Redis    *RedisConfig    `json:"redis,omitempty"`
	RabbitMQ *RabbitMQConfig `json:"rabbitmq,omitempty"`
	AIStor   *AIStorConfig   `json:"aistor,omitempty"`
	MongoDB  *MongoDBConfig  `json:"mongodb,omitempty"`
	Signoz   *SignozConfig   `json:"signoz,omitempty"`
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

type AIStorConfig struct {
	Bucket    string `json:"bucket"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	Endpoint  string `json:"endpoint"`
}

type MongoDBConfig struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type SignozConfig struct {
	ServiceName string `json:"serviceName"`
	Environment string `json:"environment"`
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

type ServiceInfo struct {
	Name        string
	Label       string
	EnvBody     string
	BackupGlob  string
	BackupHow   string
	RestoreNote string
	Backupable  bool
	IsAddon     bool
}

var serviceCatalog = []ServiceName{Postgres, Redis, RabbitMQ, AIStor, MongoDB, Signoz}

var serviceLabels = map[ServiceName]string{
	Postgres: "PostgreSQL",
	Redis:    "Redis",
	RabbitMQ: "RabbitMQ",
	AIStor:   "AIStor",
	MongoDB:  "MongoDB",
	Signoz:   "SignOz",
}

var serviceBackupHow = map[ServiceName]string{
	Postgres: "pg_dump --clean --if-exists of the dedicated database — captures schema + data; restore is a drop-and-reimport via psql.",
	Redis:    "Logical snapshot via Lua: iterates KEYS '<app>:*' and serializes strings/hashes/lists/sets/zsets to JSON; restore rewrites the keys.",
	RabbitMQ: "rabbitmqctl export_definitions filtered by vhost — definitions only (queues/exchanges/bindings/policies/users). In-flight messages are NOT saved.",
	AIStor:   "mc mirror local/<bucket> to a temp directory + tar streaming — preserves every object. Restore extracts the tar and runs mc mirror --overwrite --remove to match the snapshot state.",
	MongoDB:  "mongodump --archive --gzip of the dedicated database — captures collections, documents and indexes. Restore is a drop-and-reimport via mongorestore --drop.",
	Signoz:   "No per-app backup — telemetry lives in the shared ClickHouse and expires via the SignOz retention policy.",
}

var serviceRestoreNote = map[ServiceName]string{
	Postgres: "psql -d <app> reimports the dump; ownership and privileges are reapplied to the app's role.",
	Redis:    "Reads the JSON and applies SET/HSET/RPUSH/SADD/ZADD per key; does not delete keys absent from the snapshot.",
	RabbitMQ: "rabbitmqctl import_definitions recreates the vhost's objects (idempotent).",
	AIStor:   "Overwrites the entire bucket with the tar contents (--remove deletes objects that disappeared between the snapshots).",
	MongoDB:  "mongorestore --archive --gzip --drop reimports the database; each snapshot collection is dropped and recreated (idempotent). The app's user is reapplied by reconcile, not by restore.",
	Signoz:   "—",
}

var serviceBackupGlob = map[ServiceName]string{
	Postgres: "postgres_<ts>.sql",
	Redis:    "redis_<ts>.json",
	RabbitMQ: "rabbitmq_<ts>.json",
	AIStor:   "aistor_<ts>.tar",
	MongoDB:  "mongodb_<ts>.archive",
	Signoz:   "",
}

var nonBackupable = map[ServiceName]bool{
	Signoz: true,
}

// Addons aren't per-app provisioned resources — they're shelf-wide
// capabilities an app opts into. Today only SignOz qualifies (no
// credentials, no isolation, just config). The UI splits services vs.
// addons for clarity.
var addons = map[ServiceName]bool{
	Signoz: true,
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

func (a App) hasService(name ServiceName) bool {
	switch name {
	case Postgres:
		return a.Entry.Services.Postgres != nil
	case Redis:
		return a.Entry.Services.Redis != nil
	case RabbitMQ:
		return a.Entry.Services.RabbitMQ != nil
	case AIStor:
		return a.Entry.Services.AIStor != nil
	case MongoDB:
		return a.Entry.Services.MongoDB != nil
	case Signoz:
		return a.Entry.Services.Signoz != nil
	}
	return false
}

// MissingServices lists missing non-addon services — the ones with real
// per-app provisioning (DB, ACL user, vhost, bucket). Used by the
// "Attach services" form on the app detail page.
func (a App) MissingServices() []string {
	missing := []string{}
	for _, s := range serviceCatalog {
		if addons[s] {
			continue
		}
		if !a.hasService(s) {
			missing = append(missing, string(s))
		}
	}
	return missing
}

// MissingAddons lists addons the app could still attach.
func (a App) MissingAddons() []string {
	missing := []string{}
	for _, s := range serviceCatalog {
		if !addons[s] {
			continue
		}
		if !a.hasService(s) {
			missing = append(missing, string(s))
		}
	}
	return missing
}

func (a App) buildServiceInfos(predicate func(ServiceName) bool) []ServiceInfo {
	blocks := map[string]string{}
	for _, b := range a.EnvBlocks() {
		blocks[b.Service] = b.Body
	}
	infos := []ServiceInfo{}
	for _, s := range serviceCatalog {
		if !a.hasService(s) {
			continue
		}
		if !predicate(s) {
			continue
		}
		label := serviceLabels[s]
		infos = append(infos, ServiceInfo{
			Name:        string(s),
			Label:       label,
			EnvBody:     blocks[label],
			BackupGlob:  serviceBackupGlob[s],
			BackupHow:   serviceBackupHow[s],
			RestoreNote: serviceRestoreNote[s],
			Backupable:  !nonBackupable[s],
			IsAddon:     addons[s],
		})
	}
	return infos
}

// ServiceInfos returns only non-addon services (for the Backup strategy
// table on the app detail page).
func (a App) ServiceInfos() []ServiceInfo {
	return a.buildServiceInfos(func(s ServiceName) bool { return !addons[s] })
}

// AddonInfos returns only the addons currently attached (for the Addons
// section on the app detail page).
func (a App) AddonInfos() []ServiceInfo {
	return a.buildServiceInfos(func(s ServiceName) bool { return addons[s] })
}

func (a App) ServiceNames() []string {
	names := make([]string, 0, 6)
	if a.Entry.Services.Postgres != nil {
		names = append(names, string(Postgres))
	}
	if a.Entry.Services.Redis != nil {
		names = append(names, string(Redis))
	}
	if a.Entry.Services.RabbitMQ != nil {
		names = append(names, string(RabbitMQ))
	}
	if a.Entry.Services.AIStor != nil {
		names = append(names, string(AIStor))
	}
	if a.Entry.Services.MongoDB != nil {
		names = append(names, string(MongoDB))
	}
	if a.Entry.Services.Signoz != nil {
		names = append(names, string(Signoz))
	}
	return names
}

func (a App) BackupableServiceNames() []string {
	names := make([]string, 0, 5)
	for _, n := range a.ServiceNames() {
		if !nonBackupable[ServiceName(n)] {
			names = append(names, n)
		}
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
	blocks := make([]EnvBlock, 0, 5)
	if cfg := a.Entry.Services.Postgres; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "PostgreSQL", Body: postgresEnv(*cfg)})
	}
	if cfg := a.Entry.Services.Redis; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "Redis", Body: redisEnv(*cfg)})
	}
	if cfg := a.Entry.Services.RabbitMQ; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "RabbitMQ", Body: rabbitMQEnv(*cfg)})
	}
	if cfg := a.Entry.Services.AIStor; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "AIStor", Body: aistorEnv(*cfg)})
	}
	if cfg := a.Entry.Services.MongoDB; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "MongoDB", Body: mongodbEnv(*cfg)})
	}
	if cfg := a.Entry.Services.Signoz; cfg != nil {
		blocks = append(blocks, EnvBlock{Service: "SignOz", Body: signozEnv(*cfg)})
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

func signozEnv(cfg SignozConfig) string {
	attrs := fmt.Sprintf(
		"service.name=%s,service.namespace=infra-shelf,deployment.environment=%s",
		cfg.ServiceName, cfg.Environment,
	)
	return strings.Join([]string{
		"# === SignOz (OpenTelemetry) ===",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317",
		"OTEL_EXPORTER_OTLP_PROTOCOL=grpc",
		fmt.Sprintf("OTEL_SERVICE_NAME=%s", cfg.ServiceName),
		fmt.Sprintf("OTEL_RESOURCE_ATTRIBUTES=%s", attrs),
		"OTEL_TRACES_EXPORTER=otlp",
		"OTEL_METRICS_EXPORTER=otlp",
		"OTEL_LOGS_EXPORTER=otlp",
	}, "\n")
}

func aistorEnv(cfg AIStorConfig) string {
	return strings.Join([]string{
		"# === AIStor (S3) ===",
		fmt.Sprintf("S3_ENDPOINT=%s", cfg.Endpoint),
		fmt.Sprintf("S3_BUCKET=%s", cfg.Bucket),
		"S3_REGION=us-east-1",
		fmt.Sprintf("S3_ACCESS_KEY_ID=%s", cfg.AccessKey),
		fmt.Sprintf("S3_SECRET_ACCESS_KEY=%s", cfg.SecretKey),
		"S3_FORCE_PATH_STYLE=true",
		fmt.Sprintf("AWS_ENDPOINT_URL=%s", cfg.Endpoint),
		fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", cfg.AccessKey),
		fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", cfg.SecretKey),
		"AWS_REGION=us-east-1",
	}, "\n")
}

func mongodbEnv(cfg MongoDBConfig) string {
	return strings.Join([]string{
		"# === MongoDB ===",
		fmt.Sprintf("MONGODB_URL=mongodb://%s:%s@mongodb:27017/%s?authSource=%s",
			cfg.Username, cfg.Password, cfg.Database, cfg.Database),
		"MONGODB_HOST=mongodb",
		"MONGODB_PORT=27017",
		fmt.Sprintf("MONGODB_USERNAME=%s", cfg.Username),
		fmt.Sprintf("MONGODB_PASSWORD=%s", cfg.Password),
		fmt.Sprintf("MONGODB_DATABASE=%s", cfg.Database),
	}, "\n")
}
