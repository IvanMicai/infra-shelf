// Package postgres provisions per-app PostgreSQL databases, users and grants
// inside the shared `infra-postgres` container. Backup uses pg_dump, restore
// pipes the dump back through psql.
package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/IvanMicai/infra-shelf/internal/docker"
	"github.com/IvanMicai/infra-shelf/internal/passwordgen"
	"github.com/IvanMicai/infra-shelf/internal/registry"
)

const (
	Container = "infra-postgres"
	superUser = "postgres"
)

// Provision creates a dedicated database, role and grants for appName. Returns
// the credentials to persist in the registry.
func Provision(ctx context.Context, appName string) (registry.PostgresConfig, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return registry.PostgresConfig{}, err
	}
	password := passwordgen.Generate(24)

	steps := []string{
		fmt.Sprintf(`CREATE DATABASE "%s";`, appName),
		fmt.Sprintf(`CREATE USER "%s" WITH PASSWORD '%s';`, appName, password),
		fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s";`, appName, appName),
	}
	for _, sql := range steps {
		if _, err := psql(ctx, "", sql); err != nil {
			return registry.PostgresConfig{}, err
		}
	}

	if err := grantPermissions(ctx, appName); err != nil {
		return registry.PostgresConfig{}, err
	}

	return registry.PostgresConfig{
		Database: appName,
		Username: appName,
		Password: password,
	}, nil
}

// Backup dumps the dedicated database into destPath using pg_dump --clean
// --if-exists. The file is overwritten if it already exists.
func Backup(ctx context.Context, appName, destPath string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	sql, err := docker.Exec(ctx, Container,
		"pg_dump", "-U", superUser, "--clean", "--if-exists", appName,
	)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, []byte(sql), 0o600)
}

// Restore reads srcPath and feeds it into psql, then reapplies grants so the
// app's role keeps ownership of the restored objects.
func Restore(ctx context.Context, appName, srcPath string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := docker.ExecWithStdin(ctx, Container, f,
		"psql", "-U", superUser, "-d", appName,
	); err != nil {
		return fmt.Errorf("psql restore failed: %w", err)
	}

	return grantPermissions(ctx, appName)
}

// Teardown disconnects clients, drops the database, and drops the role.
func Teardown(ctx context.Context, appName string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}

	statements := []string{
		fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s';`, appName),
		fmt.Sprintf(`DROP DATABASE IF EXISTS "%s";`, appName),
		fmt.Sprintf(`DROP USER IF EXISTS "%s";`, appName),
	}
	for _, sql := range statements {
		if _, err := psql(ctx, "", sql); err != nil {
			return err
		}
	}
	return nil
}

func psql(ctx context.Context, database, sql string) (string, error) {
	args := []string{"psql", "-U", superUser}
	if database != "" {
		args = append(args, "-d", database)
	}
	args = append(args, "-c", sql)
	return docker.Exec(ctx, Container, args...)
}

func grantPermissions(ctx context.Context, appName string) error {
	sql := strings.ReplaceAll(grantsTemplate, "__APP__", appName)
	_, err := psql(ctx, appName, sql)
	return err
}

// grantsTemplate mirrors the legacy TS DO $$ block: hands ownership of every
// non-system schema, table and sequence to the app role and updates default
// privileges so future objects inherit the same grants. `__APP__` is replaced
// with the validated app name; validation upstream guarantees it is safe to
// interpolate into SQL identifiers.
const grantsTemplate = `
DO $$ DECLARE r record;
BEGIN
  FOR r IN SELECT nspname FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname != 'information_schema'
  LOOP
    EXECUTE format('ALTER SCHEMA %I OWNER TO %I', r.nspname, '__APP__');
    EXECUTE format('GRANT ALL ON ALL TABLES IN SCHEMA %I TO %I', r.nspname, '__APP__');
    EXECUTE format('GRANT ALL ON ALL SEQUENCES IN SCHEMA %I TO %I', r.nspname, '__APP__');
    EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT ALL ON TABLES TO %I', r.nspname, '__APP__');
    EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT ALL ON SEQUENCES TO %I', r.nspname, '__APP__');
  END LOOP;
  FOR r IN SELECT schemaname, tablename FROM pg_tables WHERE schemaname NOT LIKE 'pg_%' AND schemaname != 'information_schema'
  LOOP
    EXECUTE format('ALTER TABLE %I.%I OWNER TO %I', r.schemaname, r.tablename, '__APP__');
  END LOOP;
  FOR r IN SELECT sequence_schema, sequence_name FROM information_schema.sequences WHERE sequence_schema NOT LIKE 'pg_%' AND sequence_schema != 'information_schema'
  LOOP
    EXECUTE format('ALTER SEQUENCE %I.%I OWNER TO %I', r.sequence_schema, r.sequence_name, '__APP__');
  END LOOP;
END $$;
`
