# Examples

## Connecting a project to infra-shelf

[`docker-compose.yml`](docker-compose.yml) shows how to attach your own project
to the shared `infra-shelf` Docker network.

1. Start infra-shelf in its repo: `make up`.
2. Provision credentials: `./shelf setup myapp -s postgres,redis,rabbitmq,mongodb`.
3. Paste the printed env block into your project's `.env`.
4. Start your app: `docker compose up`.

Inside the network your app reaches services by hostname — `postgres`, `redis`,
`rabbitmq`, `mongodb` (and `aistor` once you run `make s3-up`).
