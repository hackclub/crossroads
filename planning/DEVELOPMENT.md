# Developing the backend

How to get the backend running locally, test it, and change it. The code lives in `backend/`. For how it's put together, see [INTERNALS.md](INTERNALS.md); for its routes, [ROUTES.md](ROUTES.md).

## What you need

- **Go 1.27** or newer (`backend/go.mod` sets the version).
- **Docker**, for Postgres. On Linux, make sure the daemon is running: `sudo systemctl start docker` (or `enable --now` to start it at boot).
- **A C compiler** (gcc or clang). sqlc, the query code generator, is built from source the first time you run it.
- **A Hack Club Auth app**, for signing in. See below.
- Optionally `psql`. You can also use the one inside the Postgres container.

## First-time setup

```sh
cd backend
docker compose up -d     # Postgres 18 on localhost:5433
cp .env.example .env     # then fill it in; see below
set -a; . ./.env; set +a
go run .                 # runs migrations, then serves on :8080
```

Check it's up with `curl -i localhost:8080/healthz`, which should return `204`.

Postgres is on **5433**, not the usual 5432, so it doesn't collide with other projects' databases. Its data lives in the `crossroads_postgres` Docker volume and survives restarts.

### The Hack Club Auth app

1. Go to https://auth.hackclub.com, turn on developer mode, open Developer Apps and create an app ("app me up!").
2. Add the redirect URI the backend will use: `http://localhost:8080/auth/hcacallback` if you open the backend directly, or your frontend dev server's origin plus `/auth/hcacallback` if it proxies the backend (see INTEGRATING.md).
3. Put the client ID and secret in `.env`.
4. The app needs the `hq_official` trust level, because sign-in asks for the `basic_info` (birthday, phone) and `address` scopes, which the Unified YSWS DB needs. Only HQ staff can set that; ask whoever runs Hack Club Auth if it's missing. The Crossroads app already has it.

### Environment variables

| Variable | Required | What |
|---|---|---|
| `DATABASE_URL` | yes | Postgres connection string. The compose database is `postgres://crossroads:crossroads@localhost:5433/crossroads?sslmode=disable`. The server refuses to start without it, rather than falling back to Postgres' defaults (localhost:5432, which may be another project's database). |
| `PORT` | no | Defaults to `8080`. |
| `HCA_CLIENT_ID`, `HCA_CLIENT_SECRET` | yes | From the Hack Club Auth app. |
| `HCA_REDIRECT_URL` | yes | Has to match a redirect URI on the app exactly. If it's `https://`, cookies are marked `Secure`. |
| `STAMPS_PER_HOUR` | yes | Stamps paid per approved hour. **30** for Crossroads, so the 60-hour ticket costs 1,800. |
| `AIRTABLE_TOKEN` | no | Turns on the Airtable sync. **Leave it empty in development** unless `AIRTABLE_BASE_ID` points at a test copy of the base; otherwise test data goes into the real one. Needs `data.records:read` and `data.records:write` on the base. |
| `AIRTABLE_BASE_ID` | with the token | The real base is `appuxTrbdZ2tjCo0X` ("YSWS - Crossroads"). |
| `TEST_DATABASE_URL` | for tests | Any Postgres the tests can create schemas in. The compose one works. |

`.env` is gitignored. Keep secrets out of `.env.example`.

The date Hackatime time starts counting from isn't an environment variable. It's `hoursStart` in `backend/main.go` (midnight UTC, Oct 30 2026, the day the RSVP page first went up).

### Making yourself an admin

Sign in once (open http://localhost:8080/auth/login?next=/api/me), then:

```sh
docker exec crossroads-postgres-1 psql -U crossroads \
  -c "update users set role = 'admin' where email = 'you@hackclub.com'"
```

After that, admins can change roles through `PATCH /api/admin/users/{id}`.

## Everyday commands

All from `backend/`:

```sh
set -a; . ./.env; set +a; go run .   # run the server
go vet ./...                          # catch mistakes
gofmt -l .                            # list badly formatted files (gofmt -w . fixes them)
go generate ./db                      # regenerate query code after changing SQL

# tests (see below)
TEST_DATABASE_URL='postgres://crossroads:crossroads@localhost:5433/crossroads?sslmode=disable' go test ./...
```

Open a database shell with `docker exec -it crossroads-postgres-1 psql -U crossroads`.

## Tests

```sh
export TEST_DATABASE_URL='postgres://crossroads:crossroads@localhost:5433/crossroads?sslmode=disable'
go test ./...              # everything
go test -race ./...        # with the race detector; worth it, there's real concurrency
go test ./api -run Shop -v # one area, verbose
```

**Without `TEST_DATABASE_URL`, the database tests are skipped, not failed**, and `go test` still prints `ok`. Run with `-v` to see `SKIP` lines if you're unsure.

How they're set up:

- Each test gets its own throwaway schema in the test database (`db/dbtest`), migrated from scratch and dropped afterwards. They never touch the dev data.
- Outside services are faked with `httptest` servers: Hackatime (with overlapping time, private stats and flagged accounts), Hack Club Auth's sign-in and verification check, and Airtable.
- The first test in each package can take 5 to 10 seconds. Packages run in parallel, and migrations take a Postgres advisory lock, so they wait for each other.
- Concurrency tests (simultaneous submissions, box openings and purchases) hold requests at a barrier so they really do race. They've been checked to fail when the protection they test is removed. Keep it that way if you touch them.

What's where:

| File | Covers |
|---|---|
| `db/schema_test.go` | Rules the database enforces by itself, and migrating all the way down and back up |
| `auth/auth_test.go` | Signing in and out, sessions, roles, the saved profile |
| `api/projects_test.go` | Projects, linking Hackatime, submitting (plus the shared fakes and helpers) |
| `api/dupes_test.go` | Every way of getting the same time paid twice |
| `api/review_test.go` | The review queue, approvals, payouts, milestones |
| `api/boxes_test.go` | Boxes, supply caps, special stamps, admin setup |
| `api/shop_test.go` | Verification, purchases, stock, refunds, adjustments |
| `airtablesync/airtablesync_test.go` | What gets copied to Airtable, and fulfilling from there |

## Changing the database

The schema is in `db/migrations` (goose), queries in `db/queries` (sqlc), and `go generate ./db` turns them into Go code in `db/` (`models.go`, `*.sql.go`; don't edit those by hand). The server applies new migrations when it starts.

### Adding a migration

1. Create the next numbered file, like `db/migrations/00010_something.sql`, with `-- +goose Up` and `-- +goose Down` sections. Wrap functions and anything else with semicolons inside in `-- +goose StatementBegin` / `-- +goose StatementEnd`.
2. Make the Down section undo the Up section completely. `TestMigrationsRoundTrip` migrates everything down and checks nothing is left behind.
3. `go generate ./db`, then `go build ./...`.
4. Restart the server to apply it.

**Don't edit a migration that's already been applied somewhere you care about**, like production. Goose won't run it again, so add a new one. (Migrations 1 to 5 were edited during early development, before anything was deployed.)

Name constraints when the API needs to recognize them (`db.IsViolation(err, "constraint_name")`). Postgres' automatic names for multi-column checks are just `table_check`, which says nothing.

### Adding or changing a query

Write it in the right `db/queries/*.sql` file with a `-- name: DoSomething :one|:many|:exec|:execrows` comment above it, then `go generate ./db`. Useful sqlc features already in use:

- `sqlc.embed(table)` returns whole rows as nested structs (see `GetSubmissionForReview`).
- `sqlc.arg(name)` and `sqlc.narg(name)` name parameters, with `narg` for nullable ones.
- Nullable columns come out as pointers (`*string`, `*time.Time`), and `timestamptz` and `date` as `time.Time`. See `sqlc.yaml`.
- Enums get typed constants, `Valid()` and `All…Values()`.

### Rolling back or resetting the dev database

Goose's CLI, built without the database drivers we don't use:

```sh
GOOSE='go run -tags=no_clickhouse,no_libsql,no_mssql,no_mysql,no_sqlite3,no_vertica,no_ydb github.com/pressly/goose/v3/cmd/goose@v3.29.0'
$GOOSE -dir db/migrations postgres "$DATABASE_URL" status
$GOOSE -dir db/migrations postgres "$DATABASE_URL" down-to 5   # undo 6 onward
```

Stop the server first, because it migrates up again when it starts.

To wipe everything: `docker compose down -v && docker compose up -d`, then restart the server.

## Fake data

`db/seed.sql` fills an empty database with an admin setup (special stamp types, box types and their drops, milestones, shop items) and thirteen test users in every state the platform has to show: waiting for review, rejected, hours cut, close to the ticket, ticket bought, a negative balance after a clawback, and more. The full list is at the top of the file.

```sh
docker exec -i crossroads-postgres-1 psql -U crossroads -X -v ON_ERROR_STOP=1 < db/seed.sql
```

With `DEV_MODE=1`, sign in as any of them with `/dev/login?name=maya`. Pass `role=reviewer` for rita, `role=admin` for adam and `verified=0` for nina, because signing in sets the role and verification every time. Their Hackatime is the dev fake, which only has the three starter projects. Alice's and Bob's projects line up with those. Everyone else's show no time until you add some to their Hackatime project in the dev panel.

It's one transaction, and it refuses to run on production or on a database that already has an admin setup or seeded projects. To start over, empty the database and run it again:

- **Locally:** `docker compose down -v && docker compose up -d`, then restart the server so it migrates, then the seed.
- **On Orchard** (`dev-db` in the development environment, already seeded): in the database's primary pod, `psql -d ysws_crossroads_crossroads_db -c 'truncate users, shop_items, special_stamp_types, box_types restart identity cascade'`, then the seed with `psql -d ysws_crossroads_crossroads_db -f -` or `-c`. That cascades to every table, and also signs out and removes everyone who signed in there. Nobody can sign in as the test users on Orchard, since there's no `DEV_MODE` there, but they fill the review queue and the admin pages.

## Talking to outside services in development

- **Hackatime** is read directly from the live API (https://hackatime.hackclub.com/api-docs). People are looked up by Slack ID, and need public stats turned on. There's no fake for local running, so use your own account.
- **Hack Club Auth** is the real one, including the verification check at checkout. To test the shop locally, you either need a verified account, or to set `verification_status = 'verified', ysws_eligible = true` on your user in the dev database.
- **Airtable** is off unless `AIRTABLE_TOKEN` is set. To try it, duplicate the base in Airtable and point `AIRTABLE_BASE_ID` at the copy. The copy needs the **YSWS Project Submission** and **Shop Orders** tables with the same field names.

## Deploying

Nothing is deployed yet. What the backend needs:

- Postgres. The server runs migrations on start, guarded by an advisory lock so several instances can start at once.
- The environment variables above, with an `https://` `HCA_REDIRECT_URL` registered on the Hack Club Auth app.
- To sit behind the same origin as the frontend (INTEGRATING.md, section 1).
- **Only one instance with `AIRTABLE_TOKEN` set.** The sync runs inside the server; two copies running at once could add the same rows twice.
- Build it with `go build -o crossroads .`. There's no Dockerfile yet.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `DATABASE_URL has to be set` (or the HCA or stamps variables) | `.env` isn't loaded into the shell. Run `set -a; . ./.env; set +a` first. |
| `connection refused` on 5433 | Postgres isn't running: `docker compose up -d`, and check Docker itself is running. |
| Signed in, but every request says "Sign in first." | The cookie is for a different host. Use `localhost` everywhere (not `127.0.0.1`), and make sure the frontend proxies the backend rather than calling `:8080` directly. |
| Hack Club Auth says the redirect URI is invalid | `HCA_REDIRECT_URL` doesn't exactly match one registered on the app. |
| Hackatime errors on `/api/hackatime/projects` | No Slack linked on Hack Club Auth, no Hackatime account for that Slack, or private Hackatime stats. The error message says which. |
| `go test` passes suspiciously fast | `TEST_DATABASE_URL` isn't set, so the database tests were skipped. |
| Generated code doesn't match the SQL | Run `go generate ./db`. |
