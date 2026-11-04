# Integrating a frontend with the backend

How to build a frontend (or anything else) on top of the Crossroads backend. Every route is documented in [ROUTES.md](ROUTES.md), with TypeScript types you can copy. To run the backend locally, see [DEVELOPMENT.md](DEVELOPMENT.md).

## 1. Serve the frontend and the backend from the same origin

This is the one thing you can't skip. The backend has **no CORS support**, and sign-in uses a `SameSite=Lax` session cookie. So the browser has to see the frontend, `/api/*` and `/auth/*` all on **one origin**, with a proxy routing `/api` and `/auth` to the backend.

If the frontend calls the backend on another origin, requests are blocked or arrive without the cookie, and everyone looks signed out.

### In development

Point your dev server's proxy at the backend (which runs on `:8080`). With Vite:

```ts
// vite.config.ts
export default defineConfig({
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/auth": "http://localhost:8080",
    },
  },
});
```

With Next.js, use `rewrites()` in `next.config.js` to send `/api/:path*` and `/auth/:path*` to `http://localhost:8080`.

Then make sign-in come back through the proxy. In the backend's `.env`, set

```
HCA_REDIRECT_URL=http://localhost:5173/auth/hcacallback
```

(your dev server's port), and add that exact URL to the Hack Club Auth app's redirect URIs. Open the site at `http://localhost:5173`, not `127.0.0.1`: cookies are tied to the hostname.

### In production

Put the frontend and backend behind one domain, for example `crossroads.hackclub.com`:

- **Vercel frontend:** add rewrites so Vercel proxies the backend:

  ```json
  {
    "rewrites": [
      { "source": "/api/:path*", "destination": "https://BACKEND_HOST/api/:path*" },
      { "source": "/auth/:path*", "destination": "https://BACKEND_HOST/auth/:path*" }
    ]
  }
  ```

- **Any reverse proxy** (Caddy, nginx, …): route `/api/*` and `/auth/*` to the backend and everything else to the frontend.

Set `HCA_REDIRECT_URL=https://crossroads.hackclub.com/auth/hcacallback` on the backend and register it with Hack Club Auth. Because it's `https://`, the backend marks its cookies `Secure` automatically.

If you really need the frontend on a different site from the backend, the backend needs changes first: CORS with credentials, `SameSite=None` cookies, and CSRF protection. See INTERNALS.md, "Known gaps".

## 2. Signing in and out

Sign-in is a full-page redirect through Hack Club Auth, not a `fetch`.

```ts
// Send them to sign in, and bring them back to where they were.
function signIn() {
  const next = location.pathname + location.search;
  location.href = "/auth/login?next=" + encodeURIComponent(next);
}

// Find out who's signed in. A 401 means nobody.
async function currentUser(): Promise<Me | null> {
  const res = await fetch("/api/me");
  if (res.status === 401) return null;
  if (!res.ok) throw new Error("Couldn't load your account");
  return res.json();
}

async function signOut() {
  await fetch("/auth/logout", { method: "POST" });
  location.href = "/";
}
```

`next` has to be a path on the site; anything else lands on `/`. Sessions last 30 days. A participant's name, Slack ID, verification status and so on refresh every time they sign in.

If sign-in fails, the backend shows a plain-text error page (an expired or forged sign-in, a cancelled sign-in, or Hack Club Auth being down). Retrying from your sign-in button is the fix for all of them.

## 3. Calling the API

Same-origin `fetch` sends the session cookie by default, so a small wrapper is enough:

```ts
export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch("/api" + path, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(res.status, data?.error ?? "Something went wrong.");
  return data as T;
}

// e.g.
const projects = await api<Project[]>("GET", "/projects");
const submission = await api<Submission>("POST", `/projects/${id}/submissions`, { special_stamp_id: 12 });
```

### Errors

Every error body is `{"error": "..."}`, written for the person using the site. **Show it as is**; don't replace it with something generic. The status code tells you what kind of problem it is (table in ROUTES.md). A few deserve special handling:

| Situation | Status | What to do |
|---|---|---|
| Not signed in | 401 | Send them through `signIn()` |
| Hackatime problems (`/api/hackatime/projects`, submitting) | 422 | Show the message. It tells them to link Slack on Hack Club Auth, sign in to Hackatime with the same Slack, or turn on public stats in Hackatime settings. |
| Buying before verification | 403 | Show the message with a link to https://auth.hackclub.com |
| Not enough stamps | 422 | Show the message; it includes how many more they need |
| Something changed underneath them | 409 | Show the message and reload the data |

Unknown fields in a request body are rejected with `400`, which catches typos early.

### Units and formatting

- Time is in **seconds** everywhere. Show hours: `(seconds / 3600).toFixed(1)` or "3h 20m".
- Stamps are whole numbers. A balance can be negative after an admin takes stamps back.
- Stipends are in **cents** (`stipend_cents`).
- Timestamps are RFC 3339 strings; parse them with `new Date(...)`.
- IDs are numbers.

## 4. Building each part

### Participant home

- `GET /api/me` gives their name, `stamps` and verification status.
- **Progress to the ticket:** find the item with `kind: "ticket"` in `GET /api/shop/items`; progress is `me.stamps / ticket.price`. Spending stamps on other things pushes the ticket further away, which is intended.
- **Already have a ticket:** some order in `GET /api/orders` has `item.kind === "ticket"` and a status other than `"cancelled"`.
- **Total approved hours:** add up `approved_seconds` over `GET /api/projects`.

### Before their first project: check Hackatime

Call `GET /api/hackatime/projects` early. If it returns `422`, their Hackatime can't be read yet (no Slack linked, no Hackatime account, or private stats), and nothing else will work until they fix it. Show the error prominently with links to https://auth.hackclub.com and https://hackatime.hackclub.com.

### Creating and editing projects

1. Fill a picker from `GET /api/hackatime/projects`. Each entry has `seconds` since the event started, and a `project_id` if it already counts toward one of their projects. Show those as taken (or "move from X"); a Hackatime project can only count toward one Crossroads project.
2. `POST /api/projects` with at least a `name`. Everything else can be added later.
3. `PATCH /api/projects/{id}` to edit. `hackatime_projects` replaces the whole list. A Hackatime project that's already been paid out on a project can't be unlinked from it (`409`).

There are no image uploads: `screenshot_url` is a link (for example to the Hack Club CDN).

### Project page and submitting

- `GET /api/projects/{id}` includes `unsubmitted_seconds`: exactly what a submission right now would claim. Disable the submit button when it's 0, or while the latest submission is `pending`.
- While a submission is pending, the project can't be edited (`409`). Say so in the edit form.
- To use a special stamp, let them pick from `GET /api/special-stamps` where `type.effect !== "none"` and `submission_id === null`, and send `{"special_stamp_id": id}` with the submission. One per submission; it's used up even if the submission is rejected.
- If submitting returns `422` for missing fields, the message lists all of them.
- Submission history is `project.submissions`, newest first, with `feedback` once reviewed. Nothing is pushed to the browser; reload when the page regains focus, or poll gently.

### Boxes and the collection

- Unopened boxes are the ones in `GET /api/boxes` with `opened_at === null`.
- `GET /api/box-types` has each box's art and its **current odds**. Show them before opening; participants are minors, and showing the odds is part of keeping boxes clearly away from gambling.
- `POST /api/boxes/{id}/open` returns the `SpecialStamp` that dropped. Animate the reveal from that.
- The collection is `GET /api/special-stamps`.

### Shop

- `GET /api/shop/items`: each item's `bought` count lets you show "1 of 1 claimed", and `stock` lets you show "3 left".
- `POST /api/shop/items/{id}/orders` to buy. For physical items, either send a `shipping_address` or leave it out to ship to the address on their Hack Club account. Tickets and stipends take no body.
- Expect `403` with a verification message for anyone not yet verified by Hack Club Auth. People can verify and then retry without signing in again; the backend re-checks at checkout.
- `GET /api/orders` for their order history.

### Reviewer pages

1. Queue: `GET /api/review/queue`. Priority-review submissions are already first.
2. Detail: `GET /api/review/submissions/{id}`. Make these stand out:
   - `projects_sharing_links`: other projects using the same repo or demo link. Usually a copied project, or one project split in two.
   - `hackatime.convicted`, or a `hackatime_error`.
   - `user.verification_status` and `ysws_eligible`.
   - `project.submissions` with earlier `internal_notes`.
3. Decision: `POST /api/review/submissions/{id}` with `status`, optional `approved_seconds` (let reviewers enter hours and convert), `feedback` (required to reject) and `internal_notes`. The response says how many stamps were paid and which boxes were granted. Reviewers can't review their own projects (`403`).

### Admin pages

Everything under `/api/admin` (ROUTES.md has the details). Before launch, an admin needs to set up, in this order:

1. **Roles:** `PATCH /api/admin/users/{id}` to make reviewers and other admins. The very first admin has to be set in the database (see DEVELOPMENT.md).
2. **Special stamp types:** at least one unlimited collectible, plus capped ones like a priority-review stamp.
3. **Box types:** create one, then `PUT .../drops`. At least one drop has to be unlimited.
4. **Milestones:** for example `{"name": "First 10 hours", "approved_seconds": 36000, "box_type_id": 1}`. People already past it get the box immediately.
5. **Shop items:** the ticket (`kind: "ticket"`, price 1,800 at 30 stamps an hour for 60 hours), physical items (set `stock` if limited), and stipends (`stipend_cents`).

Orders can be fulfilled from the admin API or by ticking **Fulfilled** in Airtable's Shop Orders table. Cancel through the API so stamps are refunded.

Until there's an admin UI, these routes work with curl and your session cookie (copy `crossroads_session` from the browser's dev tools after signing in):

```sh
curl -b 'crossroads_session=PASTE_IT_HERE' -H 'Content-Type: application/json' \
  -d '{"name": "Express", "rarity": "rare", "effect": "priority_review", "supply_cap": 25}' \
  http://localhost:8080/api/admin/special-stamp-types
```

## 5. What the API doesn't do yet

Things a frontend might expect that aren't there. Each is small to add (see the recipe in INTERNALS.md).

- **Milestones aren't visible to participants.** Only admins can list them, so there's no "next box at 10 hours" yet.
- **The stamps-per-hour rate isn't exposed.** For now it's the ticket price divided by 60.
- **`/api/me` doesn't include their address or birthday,** so the shop can't show which address an order will ship to by default.
- **No live updates.** Nothing is pushed; reload or poll.
- **No pagination.** Lists return everything, which is fine at this event's size. Admin user search returns at most 50.
- **No CORS,** by design (see section 1).
