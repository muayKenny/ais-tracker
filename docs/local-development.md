# Local development

Run the Go backend and TimescaleDB with Docker Compose. The React/Vite frontend
runs on your computer for fast UI development. No AWS account, CDK deployment,
ECR, or cloud resources are needed for this stage.

This sets up the environment, not replay itself. The backend currently keeps
vessel state in memory and does not read `DATABASE_URL` or write observations.
The observations schema, migrations, persistence, and replay API are the next
milestone in [the roadmap](./roadmap.md#phase-3a-persistent-history-and-replay-api).

## 1. Prerequisites

- Install and start Docker Desktop, which includes Docker Compose.
- For the frontend, install Node.js 22.12+ (or a newer supported LTS) and
  pnpm 9.13.2, the version declared in the root package manifest.
- An aisstream.io API key is optional for setup checks, required for live ships.
- Local Go and PostgreSQL installations are not required for the container path.

Run commands below from the repository root:

```sh
docker version
docker compose version
```

The Compose file pins TimescaleDB 2.25.1 with PostgreSQL 17 rather than tracking
`latest`. It selects `linux/amd64`, which Docker Desktop can emulate on Apple
Silicon; startup may be slower there. This is a local setup choice, not a
production CPU architecture decision. See the
[official image instructions](https://github.com/timescale/timescaledb-docker).

## 2. Configure live ingestion (optional)

Create or edit `.env` at the repository root and add the following, preserving
any existing settings. The file is ignored by Git.

```dotenv
AISSTREAM_API_KEY=your_key_here
```

Compose passes this value into the backend container. An existing
`apps/backend/.env` is not copied into the image; use the root `.env` for this
workflow. Without a key, the backend still starts but skips AIS ingestion.

The database uses the local-only credentials in `compose.yaml`:

| Setting | Value |
| --- | --- |
| Host from your computer | `localhost` |
| Host from the backend container | `db` |
| Port | `5432` |
| Database | `ais_tracker` |
| User | `ais` |
| Password | `local_dev_only` |

The database and backend ports bind only to the local machine. These credentials
are for disposable development environments and are not the AWS secret setup.

## 3. Start TimescaleDB

```sh
docker compose up -d --wait db
docker compose exec db psql -U ais -d ais_tracker -v ON_ERROR_STOP=1 -c 'CREATE EXTENSION IF NOT EXISTS timescaledb;'
docker compose exec db psql -U ais -d ais_tracker -c "SELECT extname, extversion FROM pg_extension WHERE extname = 'timescaledb';"
```

Expect a healthy database and one extension row with version `2.25.1`.
The extension command is safe to repeat. A named Docker volume stores database
files outside the container, so replacing the container retains the data.

## 4. Start the backend

```sh
docker compose up -d --build backend
docker compose logs --tail=30 backend
curl --fail http://localhost:8080/health
```

The current health response is `ok now`. This checks the HTTP server only;
it does not prove an upstream AIS connection or database integration.
Compose waits for database readiness before starting the backend.

`DATABASE_URL` already points to `db:5432` inside Compose for the future database
client. If running Go directly on your computer instead, the database address
would be `localhost:5432`. Inside a container, `localhost` refers to that
container itself.

## 5. Start the frontend (optional)

In a separate terminal at the repository root:

```sh
pnpm install
pnpm dev:web
```

Open the local URL printed by Vite. The current frontend connects to
`ws://localhost:8080/ws`. Keep the backend running and configure an API key to
see live ships. The local backend image currently serves the API/WebSocket,
not the built frontend.

## 6. Everyday commands

```sh
# Inspect service status and follow logs.
docker compose ps
docker compose logs -f backend db

# Open a SQL session; use \q to exit.
docker compose exec db psql -U ais -d ais_tracker

# Rebuild after changing Go code (no automatic Go reload is configured).
docker compose up -d --build backend

# Apply a changed AIS key from .env.
docker compose up -d --force-recreate backend

# Stop and remove containers while retaining database data.
docker compose down

# Start again using the retained database volume.
docker compose up -d --wait
```

Do not add `--volumes` to `down` unless you intend to erase the local database.
Changing `POSTGRES_PASSWORD` in Compose does not change the password of an
already-initialized database. Keep the pinned PostgreSQL major version until
you deliberately perform a database upgrade.

## 7. Verify persistence

Create a small setup-only record, recreate the database container, and read it:

```sh
docker compose exec db psql -U ais -d ais_tracker -v ON_ERROR_STOP=1 -c 'CREATE TABLE IF NOT EXISTS local_setup_check (id integer PRIMARY KEY); INSERT INTO local_setup_check VALUES (1) ON CONFLICT DO NOTHING;'
docker compose up -d --force-recreate --wait db
docker compose exec db psql -U ais -d ais_tracker -c 'SELECT * FROM local_setup_check;'
```

Expect the row containing `1` to survive. This verifies the volume lifecycle;
it does not yet verify vessel recording or backups. The small check table is
separate from the future application schema.

## Troubleshooting

- **Cannot connect to Docker:** start Docker Desktop and retry `docker version`.
- **Port already allocated:** stop the existing local backend or PostgreSQL
  process, or change the host-side port in Compose. If changing port 8080,
  update the frontend WebSocket URL too.
- **Database unhealthy:** inspect `docker compose logs --tail=100 db` before
  changing or deleting any volume.
- **Map has no ships:** verify the API key and backend logs. Ingestion currently
  lacks automatic reconnect; restart the backend after an upstream failure.

Once this environment works, implement versioned schema migrations, an
observations hypertable, and the Go database client. AWS infrastructure follows
the local history API, as described in [system-design.md](./system-design.md).
