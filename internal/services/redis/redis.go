// Package redis provisions per-app Redis ACL users with key-prefix isolation.
// Backup serializes the app's namespace (string/hash/list/set/zset) into a
// single JSON document via a Lua EVAL; restore replays the document key by
// key.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IvanMicai/infra-shelf/internal/docker"
	"github.com/IvanMicai/infra-shelf/internal/passwordgen"
	"github.com/IvanMicai/infra-shelf/internal/registry"
)

const Container = "infra-redis"

type ProvisionOptions struct {
	FullAccess bool // grant access to all keys (no `<app>:` prefix isolation)
}

// Provision creates the ACL user, sets its password and registers the key
// pattern it can touch. ACL SAVE persists the change to /data/users.acl so it
// survives container restarts.
func Provision(ctx context.Context, appName string, opts ProvisionOptions) (registry.RedisConfig, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return registry.RedisConfig{}, err
	}
	password := passwordgen.Generate(24)

	prefix := appName + ":"
	keyPattern := "~" + prefix + "*"
	if opts.FullAccess {
		prefix = ""
		keyPattern = "~*"
	}

	if _, err := adminCli(ctx,
		"ACL", "SETUSER", appName,
		"on", ">"+password, keyPattern, "+@all",
	); err != nil {
		return registry.RedisConfig{}, err
	}
	if _, err := adminCli(ctx, "ACL", "SAVE"); err != nil {
		return registry.RedisConfig{}, err
	}

	return registry.RedisConfig{
		Username: appName,
		Password: password,
		Prefix:   prefix,
	}, nil
}

// backupScript serializes all keys under <app>:* into an array of
// [key, type, value] triples, JSON-encoded via cjson.
const backupScript = `local keys = redis.call('KEYS', ARGV[1])
local result = {}
for i, key in ipairs(keys) do
  local t = redis.call('TYPE', key)['ok']
  local val
  if t == 'string' then val = redis.call('GET', key)
  elseif t == 'hash' then val = redis.call('HGETALL', key)
  elseif t == 'list' then val = redis.call('LRANGE', key, 0, -1)
  elseif t == 'set' then val = redis.call('SMEMBERS', key)
  elseif t == 'zset' then val = redis.call('ZRANGE', key, 0, -1, 'WITHSCORES')
  end
  result[#result+1] = {key, t, val}
end
return cjson.encode(result)`

func Backup(ctx context.Context, appName, destPath string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	jsonOut, err := adminCli(ctx, "EVAL", backupScript, "0", appName+":*")
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, []byte(jsonOut), 0o600)
}

// backupEntry mirrors the [key, type, value] triple emitted by backupScript.
type backupEntry struct {
	Key   string
	Type  string
	Value any
}

func (b *backupEntry) UnmarshalJSON(data []byte) error {
	var raw [3]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	key, ok := raw[0].(string)
	if !ok {
		return fmt.Errorf("redis backup: expected string key, got %T", raw[0])
	}
	typ, ok := raw[1].(string)
	if !ok {
		return fmt.Errorf("redis backup: expected string type, got %T", raw[1])
	}
	b.Key = key
	b.Type = typ
	b.Value = raw[2]
	return nil
}

func Restore(ctx context.Context, _ string, srcPath string) error {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	// cjson encodes an empty Lua table as `{}` (object), not `[]`. Treat
	// either as "no keys to restore" so an empty-namespace round-trip works.
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "[]" {
		return nil
	}
	var entries []backupEntry
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return fmt.Errorf("redis restore: parse %s: %w", filepath.Base(srcPath), err)
	}

	for _, entry := range entries {
		if err := restoreEntry(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

func restoreEntry(ctx context.Context, e backupEntry) error {
	switch e.Type {
	case "string":
		s, _ := e.Value.(string)
		_, err := adminCli(ctx, "SET", e.Key, s)
		return err
	case "hash":
		args := []string{"HSET", e.Key}
		args = append(args, stringSlice(e.Value)...)
		_, err := adminCli(ctx, args...)
		return err
	case "list":
		items := stringSlice(e.Value)
		if len(items) == 0 {
			return nil
		}
		args := append([]string{"RPUSH", e.Key}, items...)
		_, err := adminCli(ctx, args...)
		return err
	case "set":
		items := stringSlice(e.Value)
		if len(items) == 0 {
			return nil
		}
		args := append([]string{"SADD", e.Key}, items...)
		_, err := adminCli(ctx, args...)
		return err
	case "zset":
		pairs := stringSlice(e.Value)
		// ZRANGE WITHSCORES emits member,score,member,score; ZADD wants score,member.
		args := []string{"ZADD", e.Key}
		for i := 0; i+1 < len(pairs); i += 2 {
			args = append(args, pairs[i+1], pairs[i])
		}
		if len(args) == 2 {
			return nil
		}
		_, err := adminCli(ctx, args...)
		return err
	}
	return nil
}

func stringSlice(value any) []string {
	if value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		} else {
			out = append(out, fmt.Sprintf("%v", it))
		}
	}
	return out
}

// Teardown wipes every key under <app>:* and deletes the ACL user. Persists
// the ACL file so the user doesn't come back on restart.
func Teardown(ctx context.Context, appName string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	const deleteScript = `local keys = redis.call('keys', ARGV[1]) for i=1,#keys do redis.call('del', keys[i]) end return #keys`
	if _, err := adminCli(ctx, "EVAL", deleteScript, "0", appName+":*"); err != nil {
		return err
	}
	if _, err := adminCli(ctx, "ACL", "DELUSER", appName); err != nil {
		return err
	}
	_, err := adminCli(ctx, "ACL", "SAVE")
	return err
}

func adminCli(ctx context.Context, args ...string) (string, error) {
	pass := strings.TrimSpace(adminPassword())
	cli := []string{"redis-cli"}
	if pass != "" {
		cli = append(cli, "-a", pass)
	}
	cli = append(cli, args...)
	return docker.Exec(ctx, Container, cli...)
}

// adminPassword reads REDIS_PASSWORD from env first, then falls back to the
// .env file resolved from the workspace root. Matches what docker-compose.yml
// passes to the container so the same secret is used on both sides.
func adminPassword() string {
	if v := strings.TrimSpace(os.Getenv("REDIS_PASSWORD")); v != "" {
		return v
	}
	for _, candidate := range envFileCandidates() {
		v := readDotenvKey(candidate, "REDIS_PASSWORD")
		if v != "" {
			return v
		}
	}
	return ""
}

func envFileCandidates() []string {
	out := []string{}
	if root := strings.TrimSpace(os.Getenv("INFRA_SHELF_ROOT")); root != "" {
		out = append(out, filepath.Join(root, ".env"))
	}
	if cwd, err := os.Getwd(); err == nil {
		out = append(out, filepath.Join(cwd, ".env"))
	}
	return out
}

func readDotenvKey(path, key string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if len(value) >= 2 {
			if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
				(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
				value = value[1 : len(value)-1]
			}
		}
		return value
	}
	return ""
}
