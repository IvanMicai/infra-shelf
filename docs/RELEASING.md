# Releasing

Releases are created automatically from commits merged into `main`. The release
workflow uses semantic-release and Conventional Commits to decide the next
SemVer version, updates [`CHANGELOG.md`](../CHANGELOG.md), creates a GitHub
release, then pushes the image to Docker Hub with matching tags.

> **Branch protection:** the workflow commits the updated `CHANGELOG.md` back to
> `main` (via `@semantic-release/git`, with `[skip ci]`). If `main` is
> protected, allow the GitHub Actions bot to bypass the push restriction, or the
> release step will fail when it tries to push the changelog commit.

## Docker Hub Setup

Create a Docker Hub repository in your user or organization namespace:

- `infra-shelf`

Create a Docker Hub personal access token:

1. Sign in to Docker Hub.
2. Open Account settings.
3. Open Personal access tokens.
4. Generate a token with `Read & Write` access.
5. Copy the token immediately. Docker Hub does not show it again later.

In the GitHub repository, open Settings > Environments > `production` and add:

| Name | Value |
| --- | --- |
| `DOCKERHUB_NAMESPACE` | Docker Hub username or organization, for example `ivanmicai` |
| `DOCKERHUB_USERNAME` | Docker Hub username used to push images |
| `DOCKERHUB_TOKEN` | Docker Hub personal access token |

`DOCKERHUB_NAMESPACE` may also be a repository variable; the workflow prefers
the variable and falls back to the secret.

The workflow uses `GITHUB_TOKEN` for the GitHub release. Keep GitHub Actions
enabled with write access to repository contents so it can create tags and
releases.

## Version Rules

Use Conventional Commit messages:

| Commit message | Release |
| --- | --- |
| `fix: reconcile skips existing mongo users` | Patch, for example `1.2.3` to `1.2.4` |
| `perf: cache the registry between requests` | Patch |
| `feat: add per-app ClickHouse provisioning` | Minor, for example `1.2.3` to `1.3.0` |
| `feat!: change the apps.json schema` | Major, for example `1.2.3` to `2.0.0` |
| Message with `BREAKING CHANGE:` footer | Major |
| `docs: update backup guide` | No release by default |
| `chore: update tooling` | No release by default |

When using squash merge, make the pull request title follow the same convention
because GitHub uses it as the final commit title. PR titles are already linted
by the `pr-title` workflow.

Add `[skip release]` or `[release skip]` to a commit message when a change should
be ignored by release analysis.

Version numbering continues from the `v0.1.0` tag that marks the initial
open-source release.

## Published Docker Tags

Each release publishes `${DOCKERHUB_NAMESPACE}/infra-shelf` for `linux/amd64`
and `linux/arm64`. For version `1.2.3`, the workflow publishes:

- `1.2.3`
- `1.2`
- `1`
- `latest`

Use exact tags such as `1.2.3` for stable deployments and rollback. Use
`latest` only when the server should always pull the newest release.

The image carries both binaries: `shelf-web` is the entrypoint, and the `shelf`
CLI is on the `PATH` at `/usr/local/bin/shelf`.

## Deploy From Docker Hub

The `app` service in `docker-compose.yml` already points at the published image,
so there is no separate compose file. Set the namespace and desired release tag
in `.env`:

```bash
DOCKERHUB_NAMESPACE=ivanmicai
IMAGE_TAG=1.2.3
```

Then pull and start:

```bash
docker compose pull app
docker compose up -d app
```

To upgrade, change `IMAGE_TAG` and run `pull` and `up -d` again. Because the
service also declares a `build:` section, `make app` still rebuilds from the
working tree when you want your local changes instead of a published release.

## Manual Image Push

The automated workflow is preferred, but a manual push is useful for testing a
Docker Hub repository:

```bash
docker login --username ivanmicai

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ivanmicai/infra-shelf:0.0.0-test \
  --push .
```

A multi-platform push needs a `docker-container` builder — create one once with
`docker buildx create --name multiarch --driver docker-container --bootstrap`
and add `--builder multiarch` to the build command.
