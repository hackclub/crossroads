#!/bin/sh
# Runs the backend for trying the review flow, cut off from everything real:
#
#   - its own database, crossroads_review_test (your usual dev database is left alone)
#   - DEV_MODE=1: sign in as anyone, fake Hackatime
#   - no AIRTABLE_TOKEN and no SIDEKICK_SECRET, whatever backend/.env says
#     (and dev mode refuses to start either one anyway), so nothing can reach
#     Airtable, the Unified YSWS DB or Sidekick
#   - example data from db/seed.sql and devreview/seed-review.sql, and fake
#     Hackatime time to match it
#
# From backend/:   sh devreview/run.sh           start (sets up the database the first time)
#                  sh devreview/run.sh --reset   throw the test database away and start over
# Then, in another terminal:   cd platform && npm run dev   →  http://localhost:5173
#
# Builds with Go if it's installed, and in the golang:1.27 Docker image if it
# isn't. Needs Docker either way, for Postgres.

set -eu
cd "$(dirname "$0")/.."

DB=crossroads_review_test
DATA=data/review-test
HACKATIME="$DATA/devhackatime.json"
PSQL="docker exec -i crossroads-postgres-1 psql -U crossroads -X -q -v ON_ERROR_STOP=1"

docker compose up -d >/dev/null
until docker exec crossroads-postgres-1 pg_isready -U crossroads >/dev/null 2>&1; do sleep 1; done

if [ "${1:-}" = "--reset" ]; then
	$PSQL -d postgres -c "drop database if exists $DB with (force)"
	rm -rf "$DATA"
fi
fresh=no
if [ -z "$($PSQL -d postgres -At -c "select 1 from pg_database where datname = '$DB'")" ]; then
	$PSQL -d postgres -c "create database $DB"
	fresh=yes
fi
mkdir -p "$DATA/uploads"

# Build the backend once. With Go installed it's built here; without, in the
# golang:1.27 Docker image (a static Linux binary, which runs here too).
BIN="$DATA/crossroads-backend"
echo "Building the backend…"
if command -v go >/dev/null 2>&1; then
	CGO_ENABLED=0 go build -o "$BIN" .
else
	docker volume create crossroads-gomod >/dev/null
	docker volume create crossroads-gobuild >/dev/null
	# The cache volumes have to belong to you, not root.
	docker run --rm -v crossroads-gomod:/m -v crossroads-gobuild:/b golang:1.27 chown -R "$(id -u):$(id -g)" /m /b
	docker run --rm -u "$(id -u):$(id -g)" -e HOME=/tmp -e CGO_ENABLED=0 -e GOOS=linux \
		-e GOMODCACHE=/cache/mod -e GOCACHE=/cache/build -e GOFLAGS=-buildvcs=false \
		-v crossroads-gomod:/cache/mod -v crossroads-gobuild:/cache/build \
		-v "$PWD:/src" -w /src golang:1.27 go build -o "$BIN" .
fi

# Only these variables reach the backend: nothing from backend/.env or the
# shell, so no Airtable or Sidekick credentials.
backend() {
	exec env -i PATH="$PATH" HOME="$HOME" \
		DATABASE_URL="postgres://crossroads:crossroads@localhost:5433/$DB?sslmode=disable" \
		PORT=8080 DEV_MODE=1 STAMPS_PER_HOUR=30 \
		HCA_CLIENT_ID=placeholder HCA_CLIENT_SECRET=placeholder \
		HCA_REDIRECT_URL=http://localhost:5173/auth/hcacallback \
		UPLOADS_DIR="$DATA/uploads" DEV_HACKATIME_FILE="$HACKATIME" \
		AIRTABLE_TOKEN= AIRTABLE_BASE_ID= SIDEKICK_SECRET= \
		"$BIN"
}

if curl -fs localhost:8080/healthz >/dev/null 2>&1; then
	echo "Something is already running on :8080 (your usual dev backend?). Stop it first." >&2
	exit 1
fi

if [ "$fresh" = yes ]; then
	echo "Setting up $DB: migrating, then loading the example data…"
	backend >"$DATA/first-run.log" 2>&1 &
	pid=$!
	until curl -fs localhost:8080/healthz >/dev/null 2>&1; do
		if ! kill -0 "$pid" 2>/dev/null; then cat "$DATA/first-run.log"; exit 1; fi
		sleep 1
	done
	kill "$pid"; wait "$pid" 2>/dev/null || true

	$PSQL -d "$DB" <db/seed.sql
	$PSQL -d "$DB" <devreview/seed-review.sql

	# Fake Hackatime for everyone: the starter projects, plus every linked
	# project at the time its latest ship says it had. Without this, ships
	# look like their Hackatime time vanished and get flagged.
	$PSQL -d "$DB" -At >"$HACKATIME" <<'SQL'
select json_object_agg(slack_id, projects) from (
    select users.slack_id, jsonb_build_object('crossroads-game', 50400, 'personal-site', 21600, 'discord-bot', 9000)
        || coalesce(jsonb_object_agg(links.hackatime_project, links.seconds) filter (where links.hackatime_project is not null), '{}') as projects
    from users
    left join lateral (
        select l.hackatime_project, greatest(7200, coalesce(max(s.hackatime_seconds / cardinality(s.hackatime_projects)), 0)) as seconds
        from project_hackatime_projects l
        left join submissions s on s.project_id = l.project_id and l.hackatime_project = any(s.hackatime_projects)
        where l.user_id = users.id
        group by l.hackatime_project
    ) links on true
    where users.hca_id like 'dev-%'
    group by users.slack_id
) people;
SQL
	echo "Example data loaded."
fi

cat <<'EOF'

Review test backend on http://localhost:8080, disconnected (no Airtable, no Unified YSWS, no Sidekick).
Start the frontend with:  cd platform && npm run dev   and open http://localhost:5173

Sign in as (open these after the frontend is up):
  rita, every review stage:    http://localhost:5173/dev/login?name=rita&role=reviewer&next=/review
  fern, fraud check only:      http://localhost:5173/dev/login?name=fern&role=reviewer&stages=fraud&next=/review?stage=fraud
  hugo, HQ sign-off only:      http://localhost:5173/dev/login?name=hugo&role=reviewer&stages=hq&next=/review?stage=hq
  adam, admin (settings):      http://localhost:5173/dev/login?name=adam&role=admin&next=/admin
  mei, a participant:          http://localhost:5173/dev/login?name=mei&next=/home

EOF
exec env -i PATH="$PATH" HOME="$HOME" \
	DATABASE_URL="postgres://crossroads:crossroads@localhost:5433/$DB?sslmode=disable" \
	PORT=8080 DEV_MODE=1 STAMPS_PER_HOUR=30 \
	HCA_CLIENT_ID=placeholder HCA_CLIENT_SECRET=placeholder \
	HCA_REDIRECT_URL=http://localhost:5173/auth/hcacallback \
	UPLOADS_DIR="$DATA/uploads" DEV_HACKATIME_FILE="$HACKATIME" \
	AIRTABLE_TOKEN= AIRTABLE_BASE_ID= SIDEKICK_SECRET= \
	"$BIN"
