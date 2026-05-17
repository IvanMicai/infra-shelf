// Package aistor provisions per-app buckets, IAM users and policies inside
// the shared AIStor (MinIO) container. Backup mirrors the bucket into a tmp
// directory and streams a tar archive; restore extracts the tar and mirrors
// it back with --remove.
package aistor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ivan/infra-shelf/internal/passwordgen"
	"github.com/ivan/infra-shelf/internal/registry"
)

const (
	Container    = "infra-aistor"
	Alias        = "local"
	endpointHost = "aistor"
	endpointPort = 9000
)

type rootCreds struct {
	user string
	pass string
}

func getRootCreds() (rootCreds, error) {
	for _, candidate := range envFileCandidates() {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		user := readKey(string(content), "AISTOR_ROOT_USER")
		pass := readKey(string(content), "AISTOR_ROOT_PASSWORD")
		if user != "" && pass != "" {
			return rootCreds{user: user, pass: pass}, nil
		}
	}

	user := strings.TrimSpace(os.Getenv("AISTOR_ROOT_USER"))
	pass := strings.TrimSpace(os.Getenv("AISTOR_ROOT_PASSWORD"))
	if user != "" && pass != "" {
		return rootCreds{user: user, pass: pass}, nil
	}
	return rootCreds{}, fmt.Errorf("AIStor: AISTOR_ROOT_USER and AISTOR_ROOT_PASSWORD must be set in .env or env")
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

func mcHostEnv() (string, error) {
	creds, err := getRootCreds()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("MC_HOST_%s=http://%s:%s@localhost:%d",
		Alias,
		url.QueryEscape(creds.user),
		url.QueryEscape(creds.pass),
		endpointPort,
	), nil
}

// mc invokes the `mc` CLI inside the AIStor container, passing the
// MC_HOST_<alias> credential via -e so we never persist root creds in the
// container's environment.
func mc(ctx context.Context, args ...string) (string, error) {
	hostEnv, err := mcHostEnv()
	if err != nil {
		return "", err
	}
	full := append([]string{"exec", "-e", hostEnv, Container, "mc"}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("mc %s failed: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// mcShell runs an arbitrary sh -c script inside the container with the
// MC_HOST_<alias> env set. Used for compound operations the mc CLI can't
// express in a single invocation (writing a policy file then loading it,
// mirror + tar pipeline, etc).
func mcShell(ctx context.Context, script string, stdin io.Reader, captureStdout io.Writer) error {
	hostEnv, err := mcHostEnv()
	if err != nil {
		return err
	}
	args := []string{"exec"}
	if stdin != nil {
		args = append(args, "-i")
	}
	args = append(args, "-e", hostEnv, Container, "sh", "-c", script)
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
		return fmt.Errorf("mc shell failed: %s", msg)
	}
	return nil
}

func policyName(appName string) string { return appName + "-rw" }

func bucketPolicy(appName string) string {
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect": "Allow",
				"Action": []string{"s3:*"},
				"Resource": []string{
					fmt.Sprintf("arn:aws:s3:::%s", appName),
					fmt.Sprintf("arn:aws:s3:::%s/*", appName),
				},
			},
		},
	}
	b, _ := json.Marshal(policy)
	return string(b)
}

func Provision(ctx context.Context, appName string) (registry.AIStorConfig, error) {
	if err := registry.ValidateAppName(appName); err != nil {
		return registry.AIStorConfig{}, err
	}
	secretKey := passwordgen.Generate(24)
	policy := policyName(appName)

	if _, err := mc(ctx, "mb", "--ignore-existing", fmt.Sprintf("%s/%s", Alias, appName)); err != nil {
		return registry.AIStorConfig{}, err
	}
	if _, err := mc(ctx, "admin", "user", "add", Alias, appName, secretKey); err != nil {
		return registry.AIStorConfig{}, err
	}

	// Write the bucket policy into a tmpfile inside the container, register it,
	// then clean up — exactly mirrors the TS flow.
	policyJSON := strings.ReplaceAll(bucketPolicy(appName), `'`, `'\''`)
	script := fmt.Sprintf(
		`printf '%%s' '%s' > /tmp/aistor-policy-%s.json && `+
			`mc admin policy create %s %s /tmp/aistor-policy-%s.json && `+
			`rm -f /tmp/aistor-policy-%s.json`,
		policyJSON, appName,
		Alias, policy, appName,
		appName,
	)
	if err := mcShell(ctx, script, nil, nil); err != nil {
		return registry.AIStorConfig{}, err
	}

	if _, err := mc(ctx, "admin", "policy", "attach", Alias, policy, "--user", appName); err != nil {
		return registry.AIStorConfig{}, err
	}

	return registry.AIStorConfig{
		Bucket:    appName,
		AccessKey: appName,
		SecretKey: secretKey,
		Endpoint:  fmt.Sprintf("http://%s:%d", endpointHost, endpointPort),
	}, nil
}

// Backup mirrors the bucket into /tmp inside the container, then tars the
// directory and streams the bytes back to destPath. The /tmp directory is
// always cleaned up, even on failure.
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
		`set -e; rm -rf /tmp/aistor-bk && mkdir -p /tmp/aistor-bk && `+
			`mc mirror --quiet --overwrite %[1]s/%[2]s /tmp/aistor-bk/%[2]s/ >/dev/null && `+
			`tar -cf - -C /tmp/aistor-bk %[2]s; `+
			`rc=$?; rm -rf /tmp/aistor-bk; exit $rc`,
		Alias, appName,
	)
	return mcShell(ctx, script, nil, out)
}

// Restore extracts the tar into /tmp inside the container, ensures the bucket
// exists, and mirrors the contents back with --remove so the bucket reflects
// the snapshot exactly.
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
		`set -e; rm -rf /tmp/aistor-rs && mkdir -p /tmp/aistor-rs && `+
			`tar -xf - -C /tmp/aistor-rs && `+
			`mc mb --ignore-existing %[1]s/%[2]s >/dev/null && `+
			`mc mirror --quiet --overwrite --remove /tmp/aistor-rs/%[2]s/ %[1]s/%[2]s >/dev/null; `+
			`rc=$?; rm -rf /tmp/aistor-rs; exit $rc`,
		Alias, appName,
	)
	return mcShell(ctx, script, f, nil)
}

// Teardown removes the bucket, detaches and deletes the policy, and removes
// the user. Each step is best-effort so a partial provisioning state can
// still be cleaned up.
func Teardown(ctx context.Context, appName string) error {
	if err := registry.ValidateAppName(appName); err != nil {
		return err
	}
	policy := policyName(appName)

	_, _ = mc(ctx, "rb", "--force", "--dangerous", fmt.Sprintf("%s/%s", Alias, appName))
	_, _ = mc(ctx, "admin", "policy", "detach", Alias, policy, "--user", appName)
	_, _ = mc(ctx, "admin", "policy", "rm", Alias, policy)
	_, _ = mc(ctx, "admin", "user", "remove", Alias, appName)
	return nil
}
