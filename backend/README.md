# crossroads backend

The API behind Crossroads: projects, reviews, stamps, stamp boxes and the shop. It's a Go server (chi) on Postgres. Postgres is the source of truth, and approved projects and orders get copied into the **YSWS - Crossroads** Airtable base.

Fuller docs are in [`../planning`](../planning): [ROUTES.md](../planning/ROUTES.md) (every route and its JSON), [INTEGRATING.md](../planning/INTEGRATING.md) (building a frontend on it), [DEVELOPMENT.md](../planning/DEVELOPMENT.md) (running and changing it) and [INTERNALS.md](../planning/INTERNALS.md) (how it works and why).

## Running it

1. `docker compose up -d` starts Postgres on port 5433.
2. Copy `.env.example` to `.env` and fill in the Hack Club Auth client ID and secret.
3. `set -a; . ./.env; set +a; go run .`

The server runs any new migrations when it starts.

## Signing in

Everyone signs in with Hack Club Auth. `/auth/login?next=/somewhere` starts it, `/auth/hcacallback` is where Hack Club Auth sends people back, and `POST /auth/logout` signs out. Name, email, Slack ID, birthday, phone number, primary mailing address and verification status are refreshed from Hack Club Auth on every sign-in. Birthday, phone and address need the `basic_info` and `address` scopes, which the app has as an `hq_official` program.

Sessions last 30 days. The cookie holds a random token, and the database only keeps its SHA-256 hash.

Everyone starts as a `participant`. To make someone a reviewer or admin, sign in once and then run `update users set role = 'admin' where email = '...'`.

## API

Everything under `/api` needs someone signed in. Errors come back as `{"error": "..."}`, written to be shown to people as is.

- `GET /api/me`: the signed-in user and their stamp balance.
- `GET /api/hackatime/projects`: their Hackatime projects with time since the event started, and which project each one counts toward.
- `GET /api/projects`, `POST /api/projects`: list and create projects. `hackatime_projects` links Hackatime projects to it.
- `GET`, `PATCH`, `DELETE /api/projects/{id}`. `GET` also asks Hackatime for `unsubmitted_seconds`, the time since the last approval. Projects can't be edited while waiting for review, and can't be deleted once they've been submitted.
- `POST /api/projects/{id}/submissions`: send it for review. It needs a description, repo, demo, screenshot and at least one Hackatime project.

### Reviewing

Reviewers and admins only.

- `GET /api/review/queue`: pending submissions, those with a priority review stamp first, then oldest first.
- `GET /api/review/submissions/{id}`: the submission, its project with every earlier submission and its internal notes, who made it, and what Hackatime says now.
- `POST /api/review/submissions/{id}` with `{"status": "approved" | "rejected", "approved_seconds", "feedback", "internal_notes"}`. `approved_seconds` defaults to all the new time. Rejections need feedback, which the participant sees; internal notes they don't. Nobody can review their own project.

Approving pays `STAMPS_PER_HOUR` for each approved hour and grants a box for every milestone the person's approved total has now reached. Stamps are worked out on the running total, so splitting time across submissions never rounds anyone out of a stamp.

### Boxes and special stamps

- `GET /api/boxes`: their boxes, and what each dropped once opened.
- `GET /api/box-types`: every box type and the current chance of each stamp, leaving out ones that have run out.
- `POST /api/boxes/{id}/open`: rolls a special stamp.
- `GET /api/special-stamps`: their special stamps. One with an effect can be used once, by submitting with `{"special_stamp_id": 1}`. Only one goes on a submission.

Each special stamp type has an optional `supply_cap`. Opening a box rolls only among types with supply left, and claims one with a single conditional update, so simultaneous opens can't give out more than the cap. Admins can't leave a box type without an unlimited stamp, so a box can never come up empty.

### Shop

- `GET /api/shop/items`, `POST /api/shop/items/{id}/orders`, `GET /api/orders`. Physical items need `{"shipping_address": {"name", "line_1", "line_2", "city", "state", "postal_code", "country"}}`.

Buying needs a verified identity that's eligible for YSWS programs. If what was stored at sign-in doesn't say so, checkout asks Hack Club Auth again, so getting verified mid-session works. The ticket is one per person, stock can't oversell, and the ledger can't go below zero, even with simultaneous purchases.

### Admin

Admins only, under `/api/admin`:

- `users` (`?q=` searches name, email or Slack ID), `PATCH users/{id}` to set `role`, `POST users/{id}/stamp-adjustments` (`{"amount", "note"}`, can go negative for clawbacks), and `POST users/{id}/boxes` or `users/{id}/special-stamps` to give them directly. Given special stamps still come out of the supply.
- `special-stamp-types`, `box-types`, `PUT box-types/{id}/drops` (`[{"special_stamp_type_id", "weight"}]`), `milestones`. A new milestone's box goes straight to everyone already past it.
- `shop-items`, `orders` (`?status=pending`), and `PATCH orders/{id}` with `{"status": "fulfilled" | "cancelled", "notes"}`. Cancelling refunds the stamps and restocks the item.

### Hackatime

Time counts from `hoursStart` in `main.go` (midnight UTC, Nov 3 2026). People are looked up on Hackatime by the Slack ID from Hack Club Auth, so they need Slack linked there and public stats turned on in Hackatime. Anyone Hackatime has flagged as a cheater (trust level red) can't submit.

A submission claims Hackatime time nobody has claimed yet. That's the smaller of two numbers: the project's time minus what its approved submissions covered, and the person's time across every Hackatime project they've linked anywhere (overlaps counted once) minus everything their pending and approved submissions claimed. The second limit stops time that counted toward two projects at once, or moved to a renamed Hackatime project, from being paid twice. Submissions for the same person go one at a time. Once a Hackatime project has been paid out on a project, it can't be unlinked. Reviewers also see any other projects sharing the same repo or demo link.

## Airtable

Postgres is the source of truth. When `AIRTABLE_TOKEN` and `AIRTABLE_BASE_ID` are set, a background sync copies into the base every 30 seconds:

- Each approved submission becomes a row in **YSWS Project Submission**, once, with approved hours as the override hours, Hackatime project names, the date range and Slack ID in the justification fields, plus the address and birthday from Hack Club Auth. Updates to an already-approved project get a duplicate justification. The sync never ticks "Automation - Submit to Unified YSWS"; a person does after checking the row.
- Each shop order becomes a row in **Shop Orders**, updated whenever it changes. The backend owns every column except **Fulfilled** and **Fulfillment Notes**. Ticking Fulfilled marks the order fulfilled in the backend. Cancel orders through the backend so the stamps are refunded.

Leave `AIRTABLE_TOKEN` empty in development unless the base ID points at a test copy of the base, or test data lands in the real one.

## Database

- Migrations are in `db/migrations` (goose), queries in `db/queries` (sqlc). After changing either, run `go generate ./db` to regenerate the Go code next to them.
- `go test ./db` checks the rules the schema enforces on its own. It needs `TEST_DATABASE_URL`; the compose database works, and each run uses a throwaway schema.

## Rules the schema enforces

- **Stamps** are the currency. `stamp_ledger` is append-only, and every insert updates `users.stamps`. A purchase that would take someone below zero fails. Admin adjustments can go negative, so stamps already spent can be clawed back from a fraudulent project.
- **Submissions** pay out once, and only for time added since the project's last approved submission. A project has at most one pending submission, and each Hackatime project counts toward only one of its owner's projects.
- **Special stamps** come from boxes, one box per milestone per person. Each type has an optional `supply_cap`. A submission takes at most one special stamp, so effects don't stack.
