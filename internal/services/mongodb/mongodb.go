// Package mongodb provisions per-app databases, users and roles inside the
// shared `infra-mongodb` container. Each app gets a dedicated database plus a
// user (created in that same database, with readWrite + dbAdmin scoped to it).
// Backup streams a gzip mongodump archive; restore pipes it back through
// mongorestore --drop.
//
// Like services/aistor, this package talks to the container through its own
// exec helper rather than internal/docker.Exec: mongodump emits a *binary*
// archive on stdout, and docker.Exec trims trailing whitespace (corrupting the
// gzip trailer). mongoShell streams stdout straight to an io.Writer instead,
// and passes the root credentials via `docker exec -e` so the password never
// lands in argv.
package mongodb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/IvanMicai/infra-shelf/internal/passwordgen"
	"github.com/IvanMicai/infra-shelf/internal/registry"
)

const Container = "infra-mongodb"

type rootCreds struct {
	user string
	pass string
}

// getRootCreds resolves MONGO_INITDB_ROOT_USERNAME / MONGO_INITDB_ROOT_PASSWORD
// from the .env files first (the values docker-compose feeds the container),
// then from the process environment. Mirrors services/aistor.getRootCreds.
func getRootCreds() (rootCreds, error) {
	for _, candidate := range envFileCandidates() {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		user := readKey(string(content), "MONGO_INITDB_ROOT_USERNAME")
		pass := readKey(string(content), "MONGO_INITDB_ROOT_PASSWORD")
		if user != "" && pass != "" {
			return rootCreds{user: user, pass: pass}, nil
		}
	}

	user := strings.TrimSpace(os.Getenv("MONGO_INITDB_ROOT_USERNAME"))
	pass := strings.TrimSpace(os.Getenv("MONGO_INITDB_ROOT_PASSWORD"))
	if user != "" && pass != "" {
		return rootCreds{user: user, pass: pass}, nil
	}
	return rootCreds{}, fmt.Errorf("MongoDB: MONGO_INITDB_ROOT_USERNAME and MONGO_INITDB_ROOT_PASSWORD must be set in .env or env")
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

func readKey(content, key string) string {
	prefix := key + "="
	for _, line := range strings.Split(content, "\n") {
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

// mongoShell runs `sh -c <script>` inside the container with the root
// credentials exported as MONGO_ROOT_USER / MONGO_ROOT_PASS. Scripts reference
// those env vars (e.g. `mongosh -u "$MONGO_ROOT_USER" ...`) so the secret stays
// out of the host-side argv. When captureStdout is non-nil the container's
// stdout is streamed to it verbatim (binary-safe, used for mongodump); when
// stdin is non-nil it is piped in (used for mongorestore).
func mongoShell(ctx context.Context, script string, stdin io.Reader, captureStdout io.Writer) error {
	creds, err := getRootCreds()
	if err != nil {
		return err
	}
	args := []string{"exec"}
	if stdin != nil {
		args = append(args, "-i")
	}
	args = append(args,
		"-e", "MONGO_ROOT_USER="+creds.user,
		"-e", "MONGO_ROOT_PASS="+creds.pass,
		Container, "sh", "-c", script,
	)
	cmd := exec.CommandContext(ctx, "docker", args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if captureStdout != nil {
		cmd.Stdout = captureStdout
	} else {
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
	}
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		return fmt.Errorf("mongodb shell failed: %s", msg)
	}
	return nil
}

// mongoEval runs a mongosh --eval as the root user. js must not contain single
// quotes (app names and generated passwords are base64url / validated, so they
// never do).
func mongoEval(ctx context.Context, js string) error {
	script := fmt.Sprintf(
		`mongosh --quiet -u "$MONGO_ROOT_USER" -p "$MONGO_ROOT_PASS" --authenticationDatabase admin --eval '%s'`,
		js,
	)
	return mongoShell(ctx, script, nil, nil)
}

// roles returns the readWrite + dbAdmin role array scoped to db, as a JS
// literal usable inside an --eval expression.
func roles(db string) string {
	return fmt.Sprintf(`[{role:"readWrite",db:"%[1]s"},{role:"dbAdmin",db:"%[1]s"}]`, db)
}

// Provision creates the dedicated database+user for appName. The user is
// created inside the app's own database, so clients authenticate with
// authSource=<app>. Returns the credentials to persist in the registry.
func Provision(ctx context.Context, appName string) (registry.MongoDBConfig, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return registry.MongoDBConfig{}, err
	}
	password := passwordgen.Generate(24)

	js := fmt.Sprintf(
		`db.getSiblingDB("%[1]s").createUser({user:"%[1]s",pwd:"%[2]s",roles:%[3]s})`,
		appName, password, roles(appName),
	)
	if err := mongoEval(ctx, js); err != nil {
		return registry.MongoDBConfig{}, err
	}

	return registry.MongoDBConfig{
		Database: appName,
		Username: appName,
		Password: password,
	}, nil
}

// Backup streams a gzip-compressed mongodump archive of the app database into
// destPath. The archive is binary, so it is written straight from the
// container's stdout to the file with no whitespace trimming.
func Backup(ctx context.Context, appName, destPath string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	script := fmt.Sprintf(
		`mongodump --quiet -u "$MONGO_ROOT_USER" -p "$MONGO_ROOT_PASS" `+
			`--authenticationDatabase admin --db "%s" --archive --gzip`,
		appName,
	)
	return mongoShell(ctx, script, nil, out)
}

// Restore pipes the gzip archive at srcPath into mongorestore. --drop replaces
// each collection in the archive (idempotent); --nsInclude scopes the restore
// to the app database defensively.
func Restore(ctx context.Context, appName, srcPath string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	script := fmt.Sprintf(
		`mongorestore --quiet -u "$MONGO_ROOT_USER" -p "$MONGO_ROOT_PASS" `+
			`--authenticationDatabase admin --archive --gzip --drop --nsInclude "%s.*"`,
		appName,
	)
	return mongoShell(ctx, script, f, nil)
}

// Ensure idempotently re-applies the app's user/password/roles for an existing
// registry entry. Used by the reconcile loop after the mongo volume is lost:
// the registry remembers the credentials, Ensure pushes them back in. Creates
// the user if missing, otherwise resets its password and roles. Safe to call
// repeatedly.
func Ensure(ctx context.Context, cfg registry.MongoDBConfig) error {
	if cfg.Database == "" || cfg.Username == "" || cfg.Password == "" {
		return fmt.Errorf("mongodb ensure: incomplete config (db=%q user=%q)", cfg.Database, cfg.Username)
	}

	js := fmt.Sprintf(
		`const d=db.getSiblingDB("%[1]s");`+
			`const r=%[4]s;`+
			`if(d.getUser("%[2]s")===null){d.createUser({user:"%[2]s",pwd:"%[3]s",roles:r});}`+
			`else{d.updateUser("%[2]s",{pwd:"%[3]s",roles:r});}`,
		cfg.Database, cfg.Username, cfg.Password, roles(cfg.Database),
	)
	return mongoEval(ctx, js)
}

// Teardown drops the app user and database. dropUser is wrapped in try/catch so
// a missing user doesn't abort the drop; dropDatabase is a no-op when absent.
func Teardown(ctx context.Context, appName string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	js := fmt.Sprintf(
		`const d=db.getSiblingDB("%[1]s");`+
			`try{d.dropUser("%[1]s");}catch(e){}`+
			`d.dropDatabase();`,
		appName,
	)
	return mongoEval(ctx, js)
}
