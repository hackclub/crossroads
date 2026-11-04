# Dev mode

Dev mode lets you try the whole platform locally without Hack Club Auth credentials or real Hackatime time: sign in as test users, add fake coding hours, ship, review, open boxes and buy the ticket. **It lets anyone sign in as anyone, so it must never run anywhere real people can reach.**

This file covers how to use it, everything it adds, and what has to happen before the platform is ready for real users. The last section is a prompt you can give Claude Code to do that work.

## Using it

You need Docker, Go 1.27 and Node (see [DEVELOPMENT.md](DEVELOPMENT.md) for the backend's setup).

```sh
# 1. Database
cd backend && docker compose up -d

# 2. Backend, with DEV_MODE=1 in backend/.env
set -a; . ./.env; set +a; go run .
#    It logs "DEV_MODE is on" when it starts.

# 3. Frontend, in another terminal
cd platform && npm install && npm run dev
```

Open **http://localhost:5173** (use `localhost`, not `127.0.0.1`; the session cookie is tied to the hostname).

The backend still refuses to start without `HCA_CLIENT_ID` and `HCA_CLIENT_SECRET`. In dev mode they can be placeholders like `placeholder`; only the real "Sign in with Hack Club" button needs real ones.

### Signing in

The sign-in page shows a **Dev sign-in** box when the backend is in dev mode. Type any name (lowercase letters, digits and dashes) and pick a role. Each name is its own account and keeps its data. Every dev user is created verified and YSWS-eligible, with a test address, so the shop works.

You can also sign in from a link: `http://localhost:5173/dev/login?name=alice&role=participant&next=/home`. Add `verified=0` to try the shop's "verify first" path; it will fail with a "couldn't reach Hack Club Auth" error unless real credentials are set, because checkout asks Hack Club Auth again.

### The dev panel

Once signed in, a purple **Dev mode** box sits bottom right. It has:

- **Sign in as:** one-click switches between alice and bob (participants), rita (reviewer) and adam (admin), staying on the same page.
- **Fake Hackatime:** add hours to any Hackatime project name for the signed-in user. New names become new Hackatime projects you can link.

Every new dev user starts with three fake Hackatime projects: `crossroads-game` (14h), `personal-site` (6h) and `discord-bot` (2.5h).

Fake Hackatime time lives in the backend's memory and **resets when the backend restarts**, while the database keeps what was submitted and approved. After a restart, a project shows no new time until you add more. That's harmless: the backend never pays out more than Hackatime reports.

### A full run-through

1. As **adam (admin)**, open Admin and set up, in order: special stamp types (at least one unlimited), a box type and its drops, a milestone (say 10 hours), and shop items, including the ticket (`kind` ticket, price 1800).
2. As **alice**, create a project, fill in every field, tick `crossroads-game`, and ship it.
3. As **rita (reviewer)**, open the Review queue, open the submission and approve it, cutting some hours if you like.
4. As **alice**, check the stamps (30 per approved hour) and the box from the milestone. Open the box on the Boxes page.
5. In the dev panel, add more hours to `crossroads-game`, ship again, approve again as rita, and buy the ticket in the Shop.
6. As **adam**, mark the order fulfilled or cancel it (cancelling refunds the stamps).

### Starting over

`docker compose down -v && docker compose up -d` in `backend/` wipes the database. Then restart the backend.

## Everything dev mode adds

Removing dev mode means deleting or reverting each of these. The ✂ items exist only for dev mode.

**Backend**
- ✂ `backend/auth/dev.go`: `DevLogin`, the `/dev/login` handler.
- ✂ `backend/devhackatime/`: the fake Hackatime server, mounted at `/dev/hackatime`.
- `backend/main.go`: the `DEV_MODE` block, which points the Hackatime client at the fake and mounts both `/dev` routes. It refuses to start if `HCA_REDIRECT_URL` is `https://`.
- `backend/.env.example`: the `DEV_MODE` entry.

**Frontend** (`platform/`)
- ✂ `src/lib/DevPanel.svelte`, and its use in `src/routes/(app)/+layout.svelte`.
- `src/lib/api.js`: `isDevMode`, `devLoginUrl` and `devHackatime`.
- `src/routes/+page.svelte`: the Dev sign-in form.
- `src/app.css`: the `.devpanel` styles.
- `vite.config.js`: the `/dev` proxy entry. Keep `/api`, `/auth` and `/uploads`.

**Not dev mode, but temporary**
- `platform/blockout/`: the old mock-data blockout, kept for reference. Nothing imports it. Delete it when it's no longer useful.
- The plain wireframe look (`src/app.css`). The design lives in the `crossroads-draft` Figma file.

## What has to happen before real users

1. **Turn dev mode off.** Remove it as listed above, or keep the backend parts as a local-only tool. If you keep them, make sure `DEV_MODE` is never set in any deployed environment, and add a test that the `/dev` routes 404 without it.
2. **Real sign-in.** Put the Crossroads app's real `HCA_CLIENT_ID` and `HCA_CLIENT_SECRET` in `.env`. Register `http://localhost:5173/auth/hcacallback` (development) and the production `https://…/auth/hcacallback` as redirect URIs, and set `HCA_REDIRECT_URL` to match. Then do the first real sign-in and confirm the birthday and address come through (INTERNALS.md, "Known gaps").
3. **Deploy on one origin.** The frontend and backend must share a domain, with `/api` and `/auth` routed to the backend (INTEGRATING.md, section 1). The frontend uses `@sveltejs/adapter-auto` and renders only in the browser (`ssr = false`). A static adapter with an SPA fallback, or a Node adapter, behind the same proxy is the simplest setup. The backend has no Dockerfile yet.
4. **Airtable.** Set `AIRTABLE_TOKEN` and `AIRTABLE_BASE_ID` on exactly one production instance.
5. **Launch data.** As an admin: special stamp types, box types and drops, milestones, shop items with the 1,800-stamp ticket. Set the first admin in the database (DEVELOPMENT.md).
6. **The real design.** Restyle the pages from the Figma frames (home, project edit, project main) and the landing page in `main page/`. Wire the landing page's "Get Started" buttons to `/auth/login`.
7. **Gaps the frontend works around today:**
   - screenshot uploads are stored on the backend's own disk (`UPLOADS_DIR`). In production that folder has to survive redeploys, and only works with one backend instance; otherwise move storage to S3 or the Hack Club CDN (`backend/uploads/`). Set `PUBLIC_URL` to the site's https address so upload URLs are right.
   - participants can't see milestones ("next box at 30h")
   - the stamps-per-hour rate is worked out from the ticket price
   - `/api/me` has no address, so the shop can't show where an order will ship
   - nothing refreshes on its own
8. **Check the details:**
   - phone widths
   - keyboard use and screen readers
   - what pages show while loading and when they're empty
   - every error message reaching the person

## Prompt: get it ready for real users

Copy this into Claude Code from the repository root.

```text
Get the Crossroads platform ready for real participants. Read planning/DEV_MODE.md,
planning/INTEGRATING.md, planning/INTERNALS.md and PRODUCT.md first.

1. Remove dev mode completely, using the "Everything dev mode adds" list in
   planning/DEV_MODE.md: backend/auth/dev.go, backend/devhackatime/, the DEV_MODE block
   in backend/main.go and .env.example, platform/src/lib/DevPanel.svelte and its use,
   the dev helpers in platform/src/lib/api.js, the Dev sign-in form, the .devpanel CSS
   and the /dev proxy (keep /api, /auth and /uploads). Then grep both apps for "dev" and DEV_MODE to make sure nothing
   is left. Keep /api and /auth proxying. Delete platform/blockout/.
2. Run go vet, gofmt and the full backend test suite (with TEST_DATABASE_URL set so
   nothing is skipped), and build the frontend with no warnings.
3. Ask me for the Hack Club Auth client ID and secret instead of guessing, set
   HCA_REDIRECT_URL for local development, and walk me through the first real
   sign-in. Confirm the birthday and address are saved.
4. Restyle the platform from the crossroads-draft Figma file (fileKey
   SLiFzmgzcIoQKyTeTvvt5T: frames "home", "project edit", "project main") and the
   landing page in "main page/". Use the Figma MCP's design context rather than
   screenshots. Keep every page's behavior and copy. Point the landing page's
   "Get Started" buttons at /auth/login.
5. Propose a deployment that serves the frontend and backend on one origin (a
   Dockerfile for the backend, a frontend adapter, and the reverse-proxy rules) and
   wait for my go-ahead before creating anything outside the repository.
6. List anything from "What has to happen before real users" that still needs a
   decision from me.

Don't commit or push until I ask. Hack Club is always written "Hack Club".
```
