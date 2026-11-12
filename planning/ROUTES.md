# Crossroads API routes

Every route the backend serves, what it takes and what it returns. For how to call these from a frontend (same-origin setup, sign-in, error handling, typical flows), see [INTEGRATING.md](INTEGRATING.md). For how they're implemented, see [INTERNALS.md](INTERNALS.md).

The code is the source of truth. Routes are registered in `backend/main.go`, `backend/auth/auth.go` (`Routes`) and `backend/api/api.go` (`Routes`). If this file and the code disagree, the code wins. Please fix this file.

## Conventions

- **Sign-in.** Everything under `/api` needs a signed-in session (the `crossroads_session` cookie). Without one: `401 {"error": "Sign in first."}`.
- **Roles.** People are `participant`, `reviewer` or `admin`. `/api/review/*` needs reviewer or admin, `/api/admin/*` needs admin. Otherwise: `403 {"error": "You don't have access to this."}`. Admins can do everything reviewers can.
- **Bodies** are JSON. Unknown fields are rejected with `400`, so typos don't fail silently. An empty body counts as `{}`.
- **Errors** are always `{"error": "..."}`, written to be shown to people as is. Server errors are `500 {"error": "Something went wrong on our end."}`; the details only go to the server log.
- **IDs in paths** that aren't numbers give `404 {"error": "That doesn't exist."}`. So does anything that belongs to someone else: participants can't tell someone else's project from a missing one.
- **Units.** Time is always whole **seconds** (`approved_seconds`, `new_seconds`, …). Stamps are whole numbers. Timestamps are RFC 3339 strings with an offset. Money for stipends is in **cents**.
- **PATCH** changes only the fields you send. For fields marked *nullable*, sending `null` clears them.

Status codes used across the API:

| Code | Meaning |
|---|---|
| 200 | OK, body is the result |
| 201 | Created, body is the new thing |
| 204 | Done, no body |
| 400 | The request couldn't be read (bad JSON, unknown field) |
| 401 | Not signed in |
| 403 | Signed in but not allowed (role, verification, flagged by Hackatime, own submission) |
| 404 | Doesn't exist, or isn't yours |
| 409 | Conflicts with the current state (already pending, sold out, already reviewed, …) |
| 422 | Understood, but invalid or not possible (missing fields, not enough stamps, Hackatime problems) |
| 502 | An outside service (Hack Club Auth) couldn't be reached |

## Shapes

TypeScript definitions of everything the API returns. Copy them into the frontend.

```ts
type Role = "participant" | "reviewer" | "admin";
// What participants see. Reviewer routes show the real stage instead
// (ReviewStatus, below): "final_check" is "fraud_check" or "awaiting_hq".
type SubmissionStatus = "pending" | "final_check" | "approved" | "rejected";
type ReviewStatus = "pending" | "fraud_check" | "awaiting_hq" | "approved" | "rejected";
type Rarity = "common" | "uncommon" | "rare" | "epic" | "legendary";
type Effect = "none" | "priority_review" | "stamp_multiplier";
type ShopItemKind = "ticket" | "price_lock" | "physical" | "stipend";
type OrderStatus = "pending" | "fulfilled" | "cancelled";

interface Me {
  id: number;
  name: string;
  email: string;
  slack_id: string | null;
  role: Role;
  stamps: number; // current balance; can be negative after an admin clawback
  verification_status: string; // "needs_submission" | "pending" | "verified" | "ineligible"
  ysws_eligible: boolean;
  address: Address | null; // from Hack Club Auth; where physical items ship by default
  stamps_per_hour: number; // stamps paid per approved hour, for showing prices in hours
  // Which review stages they can decide: all for admins, none for participants.
  can_review: boolean;
  can_fraud_review: boolean;
  can_hq_review: boolean;
}

interface Project {
  id: number;
  name: string;
  description: string;
  repo_url: string | null;
  demo_url: string | null;
  screenshot_url: string | null;
  hackatime_projects: string[]; // linked Hackatime project names, sorted
  approved_seconds: number;     // total approved across its submissions
  created_at: string;
  updated_at: string;
  // Only on GET /api/projects/{id}, and only if it has Hackatime projects linked:
  hackatime_seconds?: number;   // its Hackatime time since the event started, right now
  unsubmitted_seconds?: number; // how much a submission right now would claim
  hackatime_error?: string;     // set instead of the two above if Hackatime couldn't be asked
  submissions: Submission[];    // newest first
}

interface Submission {
  id: number;
  status: SubmissionStatus;
  hackatime_projects: string[]; // what it counted, as of submitting
  hackatime_seconds: number;    // the project's Hackatime total when submitted
  new_seconds: number;          // what this submission claims
  approved_seconds: number | null; // what it pays (or will, in final_check) once signed off
  feedback: string | null;      // the latest decision's message to them
  submitted_at: string;
  reviewed_at: string | null;   // when the first reviewer decided
  // Only on GET /api/projects/{id}: what reviewers said to them about this ship.
  messages?: { kind: string; message: string; created_at: string }[];
  // Reviewer routes only (and status is a ReviewStatus there):
  internal_notes?: string | null;
  reviewer_id?: number | null;
  checks?: Check[];             // the automatic checks when the reviewer decided
  fraud_reasons?: string[];     // why it went to the fraud check: check IDs, "manual", "everything"
  override_reason?: string;     // why it's approved for more than Hackatime tracked
  fraud_reviewer_id?: number;
  fraud_reviewed_at?: string;
  hq_reviewer_id?: number;
  hq_reviewed_at?: string;
}

interface HackatimeProject {
  name: string;
  seconds: number;          // since the event started
  project_id: number | null; // the Crossroads project it counts toward, if any
}

interface SpecialStampType {
  id: number;
  name: string;
  description: string;
  image_url: string | null;
  rarity: Rarity;
  effect: Effect; // "none" means it's only a collectible
}

interface SpecialStamp {
  id: number;
  type: SpecialStampType;
  box_id: number | null;        // null if an admin gave it or it was claimed
  submission_id: number | null; // set once it's been used
  acquired_at: string;
}

interface Box {
  id: number;
  box_type_id: number;
  milestone_id: number | null;      // null if an admin gave it
  granted_at: string;
  opened_at: string | null;         // null until opened
  special_stamp_id: number | null;  // what it dropped, once opened
}

interface BoxType {
  id: number;
  name: string;
  description: string;
  image_url: string | null;
  // What can drop right now. Types that have run out are left out, so the
  // chances (0 to 1) always add up to 1.
  odds: { special_stamp_type: SpecialStampType; chance: number }[];
}

interface ShopItem {
  id: number;
  kind: ShopItemKind;
  name: string;
  description: string;
  image_url: string | null;
  price: number;                // stamps; in GET /api/shop/items, what they'd pay now: a ticket's batch (see below), or the sale price
  stipend_cents: number | null; // stipends only
  stock: number | null;         // left in stock; null is unlimited
  max_per_user: number | null;  // null is unlimited; tickets and price locks are always 1
  active: boolean;
  position: number;             // order in the shop: lower first, ties cheapest first
  batch_size: number | null;    // tickets sold in batches: how many in each
  batch_step: number | null;    // and how many stamps more each batch costs than the one before
  batches: Batches | null;      // tickets sold in batches only
  bought: number;               // how many the signed-in person has (0 on admin routes)
  locked_price: number | null;  // tickets: the price they locked in with a price lock (null on admin routes)
  sale: Sale | null;            // while it's on sale (null on admin routes, which show the full price)
}

interface Sale {
  full_price: number;    // what it costs once the sale's over
  percent: number;       // how much is off: 10
  ends_at: string;       // when the sale ends
}

interface Batches {
  size: number;          // tickets in each batch
  prices: number[];      // each batch's price, first to last
  current: number;       // the batch on sale, as an index into prices; the last once sold out
  left_in_batch: number; // how many are left at the current price
}

interface Address {
  name: string;
  line_1: string;
  line_2: string;
  city: string;
  state: string;
  postal_code: string;
  country: string;
}

interface Order {
  id: number;
  item: ShopItem;   // the item as it is now; price below is what was paid
  price: number;
  status: OrderStatus;
  shipping_address: Address | null; // physical items only
  locked_price: number | null;       // price locks only: the ticket price it locked in
  created_at: string;
  updated_at: string;
}
```

Reviewer and admin shapes are listed with their routes below.

## Route index

| Method | Path | Who | What |
|---|---|---|---|
| GET | `/healthz` | anyone | Is the server up and the database reachable |
| GET | `/auth/login?next=/path` | anyone | Start signing in (full-page navigation) |
| GET | `/auth/hcacallback` | Hack Club Auth | Where sign-in comes back to |
| POST | `/auth/logout` | anyone | Sign out |
| GET | `/api/me` | signed in | The signed-in person |
| GET | `/api/hackatime/projects` | signed in | Their Hackatime projects, for linking |
| GET | `/api/community` | signed in | The community goal: hours shipped by everyone so far |
| GET | `/api/projects` | signed in | Their projects |
| POST | `/api/projects` | signed in | Create a project |
| GET | `/api/projects/{id}` | owner | One project, with live Hackatime numbers |
| PATCH | `/api/projects/{id}` | owner | Edit a project |
| DELETE | `/api/projects/{id}` | owner | Delete a never-submitted project |
| POST | `/api/projects/{id}/submissions` | owner | Submit for review |
| POST | `/api/uploads` | signed in | Upload a screenshot image |
| GET | `/uploads/{file}` | anyone | An uploaded image |
| GET | `/api/boxes` | signed in | Their boxes |
| POST | `/api/boxes/{id}/open` | owner | Open a box |
| GET | `/api/box-types` | signed in | Box types and their current odds |
| GET | `/api/special-stamps` | signed in | Their special stamps |
| POST | `/api/special-stamps/{id}/placement` | owner | Stick a special stamp on a project, or move it there |
| POST | `/api/special-stamps/claims` | signed in | Claim a stamp with a claim link's code |
| GET | `/api/shop/items` | signed in | What's in the shop |
| POST | `/api/shop/items/{id}/orders` | signed in, verified | Buy something |
| GET | `/api/orders` | signed in | Their orders |
| GET | `/api/review/queue?stage=` | reviewer | One review queue, and how many are in each |
| POST | `/api/review/next` | reviewer | Claim the next ship in a queue |
| GET | `/api/review/submissions/{id}` | reviewer | Everything about one ship |
| POST | `/api/review/submissions/{id}` | reviewer | Decide it at its stage: approve, reject or send back |
| POST | `/api/review/submissions/{id}/claim` | reviewer | Claim it (or keep it longer) |
| DELETE | `/api/review/submissions/{id}/claim` | reviewer | Let go of it |
| POST | `/api/review/submissions/{id}/skip` | reviewer | Let go, and don't get it from Next for 3 hours |
| POST | `/api/review/submissions/{id}/comments` | reviewer | Comment, for the participant or reviewers only |
| PUT | `/api/review/users/{id}/note` | reviewer | The reviewers-only note on a person |
| GET | `/api/review/templates` | reviewer | Feedback templates |
| POST | `/api/review/templates` | reviewer | Add one |
| PATCH | `/api/review/templates/{id}` | reviewer | Change one |
| DELETE | `/api/review/templates/{id}` | reviewer | Delete one |
| POST | `/api/sidekick` | Sidekick (shared secret) | Sidekick's protocol; see "Sidekick" below |
| GET | `/api/admin/activity?days=&min_minutes=` | admin | Participants on the platform and working on projects, per day |
| GET | `/api/admin/users?q=` | admin | Find people |
| PATCH | `/api/admin/users/{id}` | admin | Change someone's role, review stages and Emporium access |
| GET | `/api/admin/review-settings` | admin | Fraud check settings |
| PATCH | `/api/admin/review-settings` | admin | Change them |
| POST | `/api/admin/users/{id}/stamp-adjustments` | admin | Add or take away stamps |
| POST | `/api/admin/users/{id}/boxes` | admin | Give someone a box |
| POST | `/api/admin/users/{id}/special-stamps` | admin | Give someone a special stamp |
| GET | `/api/admin/role-invites` | admin | Roles waiting for people who haven't signed in yet |
| POST | `/api/admin/role-invites` | admin | Give a role to whoever first signs in with an email |
| DELETE | `/api/admin/role-invites/{id}` | admin | Take back a role invite |
| GET | `/api/admin/special-stamp-types` | admin | Stamp types with supply numbers |
| POST | `/api/admin/special-stamp-types` | admin | Create a stamp type |
| PATCH | `/api/admin/special-stamp-types/{id}` | admin | Edit a stamp type, including its supply cap and claim code |
| GET | `/api/admin/box-types` | admin | Box types with their full drop tables |
| POST | `/api/admin/box-types` | admin | Create a box type |
| PATCH | `/api/admin/box-types/{id}` | admin | Edit a box type |
| PUT | `/api/admin/box-types/{id}/drops` | admin | Replace a box type's drop table |
| GET | `/api/admin/milestones` | admin | Milestones |
| POST | `/api/admin/milestones` | admin | Create a milestone |
| GET | `/api/admin/shop-items` | admin | Every shop item, including inactive |
| POST | `/api/admin/shop-items` | admin | Create a shop item |
| PATCH | `/api/admin/shop-items/{id}` | admin | Edit a shop item |
| GET | `/api/admin/orders?status=` | admin | Orders to fulfill |
| PATCH | `/api/admin/orders/{id}` | admin | Fulfill or cancel an order |
| POST | `/slack/clicks` | the Slack bot | Record a click on a Slack announcement's button |

---

## Health

### `GET /healthz`

`204` if the server is up and can reach Postgres, `503` (plain text) if it can't reach the database. Not under `/api`, so no sign-in needed.

## Signing in (`/auth`)

These are browser routes, not JSON APIs: navigate to them, don't `fetch` them. Errors here are plain text pages.

### `GET /auth/login?next=/path`

Redirects (`302`) to Hack Club Auth. `next` is where to land afterwards, and has to be a path on this site (`/shop?tab=stickers` works; `https://…` or `//…` fall back to `/`). Sets a short-lived `crossroads_oauth_state` cookie (10 minutes) to guard against forged sign-ins.

### `GET /auth/hcacallback`

Hack Club Auth sends people here with `?code=…&state=…`. The backend checks the state, trades the code for the person's identity, creates or updates their user, starts a 30-day session (the `crossroads_session` cookie, `HttpOnly`, `SameSite=Lax`) and redirects (`302`) to `next`.

Name, email, Slack ID, birthday, phone, primary mailing address, verification status and YSWS eligibility are refreshed from Hack Club Auth on every sign-in.

| Status | When |
|---|---|
| 302 | Signed in, redirecting to `next` |
| 400 | State missing, expired or forged ("Sign-in timed out. Please try again.") |
| 403 | They cancelled on Hack Club Auth's side |
| 502 | Hack Club Auth couldn't be reached or rejected the code |

### `POST /auth/logout`

Ends the session and clears the cookie. `204`. Safe to call when already signed out.

## The signed-in person

### `GET /api/me`

`200` → `Me`. Use it to find out whether someone is signed in: `401` means they aren't.

## Hackatime

### `GET /api/hackatime/projects`

`200` → `HackatimeProject[]`: every Hackatime project they've worked on since Hackatime time started counting (Oct 30 2026, midnight UTC), with which Crossroads project each counts toward. Use it to build the project-linking picker.

| Status | When |
|---|---|
| 422 | No Slack linked to their Hack Club account; no Hackatime account for their Slack; their Hackatime stats are private; or Hackatime couldn't be reached. The `error` says which and what to do. |

## Projects

### `GET /api/projects`

`200` → `Project[]`, newest first, each with all its submissions. No live Hackatime numbers (use `GET /api/projects/{id}` for those).

### `POST /api/projects`

```json
{
  "name": "Crossroads Racer",
  "description": "A racing game",
  "repo_url": "https://github.com/heidi/racer",
  "demo_url": "https://heidi.github.io/racer",
  "screenshot_url": "https://cdn.example.com/racer.png",
  "hackatime_projects": ["racer-client", "racer-server"]
}
```

Only `name` is required. Everything else can be filled in later, but submitting needs it all. URLs have to be `http(s)://`. Hackatime project names are trimmed, deduplicated and sorted; at most 20, no commas.

`201` → `Project`.

| Status | When |
|---|---|
| 422 | Missing or too-long name (100 characters max), a bad URL, a description over 5,000 characters, or bad Hackatime project names |
| 409 | One of the Hackatime projects already counts toward another of their projects. Nothing is created. |

### `GET /api/projects/{id}`

`200` → `Project`, including `hackatime_seconds` and `unsubmitted_seconds` (or `hackatime_error`) if it has Hackatime projects linked. `unsubmitted_seconds` is exactly what submitting now would claim, after both double-counting limits (see INTERNALS.md). Asking Hackatime makes this slower than the list.

`404` if it's not theirs.

### `PATCH /api/projects/{id}`

Any of the fields from `POST`. Left-out fields don't change. A URL set to `""` is cleared. `hackatime_projects` replaces the whole list.

`200` → `Project`.

| Status | When |
|---|---|
| 409 | It's waiting for review ("can't be changed until that's done") |
| 409 | Trying to unlink a Hackatime project that's already been paid out on this project |
| 409 | A new Hackatime project already counts toward another of their projects |
| 422 | Same validation as `POST` |

### `DELETE /api/projects/{id}`

`204`. Only for projects that have never been submitted; otherwise `409`.

### `POST /api/projects/{id}/submissions`

Sends the project for review. The body is optional:

```json
{ "special_stamp_id": 12 }
```

to use a special stamp on it (one with an effect, like priority review, that hasn't been used yet).

The submission claims the project's Hackatime time that nobody has claimed yet. See INTERNALS.md, "Hours accounting".

`201` → `Submission` (status `pending`).

| Status | When |
|---|---|
| 422 | Missing a description, repo link, demo link, screenshot or Hackatime project. The `error` lists them all. |
| 422 | A Hackatime problem (no Slack, no account, private stats, unreachable) |
| 403 | Hackatime has flagged them as a cheater |
| 409 | It's already waiting for review |
| 409 | Their Hackatime links changed mid-request; try again |
| 422 | No new time since its last approval, or its new time was already claimed by another of their projects (the two cases have different messages) |
| 404 | The special stamp isn't theirs |
| 409 | The special stamp has already been used |
| 422 | The special stamp has no effect (it's only a collectible) |

## Uploads

### `POST /api/uploads`

A `multipart/form-data` body with one image in a field called `file`: PNG, JPEG, GIF or WebP, up to 5 MB. The type is judged from the file's bytes, not its name.

`201` → `{"url": "https://…/uploads/<sha256>.png"}`. Put the URL in a project's `screenshot_url`. Uploading the same image twice gives the same URL.

| Status | When |
|---|---|
| 400 | No `file` field |
| 413 | Over 5 MB |
| 422 | Not a PNG, JPEG, GIF or WebP |

### `GET /uploads/{file}`

Serves an uploaded image, cached for a year (names are content hashes). Mounted outside `/api`, so the frontend's proxy has to route `/uploads` to the backend too. Files live in `UPLOADS_DIR` (default `data/uploads`), and URLs are built from `PUBLIC_URL` (default: the origin of `HCA_REDIRECT_URL`).

## Boxes and special stamps

### `GET /api/boxes`

`200` → `Box[]`, newest first. Unopened boxes have `opened_at: null`.

People get boxes when their approved hours pass a milestone (granted during review), or from an admin.

### `POST /api/boxes/{id}/open`

Rolls a special stamp from the box's type, among the stamp types that still have supply left, weighted by the drop table. `200` → `SpecialStamp` (what dropped).

| Status | When |
|---|---|
| 404 | Not their box |
| 409 | Already opened |
| 409 | Nothing left to drop (shouldn't happen: admins can't leave a box without an unlimited stamp) |

### `GET /api/box-types`

`200` → `BoxType[]` with live odds. Show these to people before they open anything.

### `GET /api/special-stamps`

`200` → `SpecialStamp[]`, newest first. Ones they can use on a submission have `type.effect !== "none"` and `submission_id === null`.

### `POST /api/special-stamps/claims`

```json
{ "code": "heard-it-first" }
```

Claims a stamp of the type with that claim code (the platform's `/claim/{code}` page does this as it opens). Each person gets one per type. → `201 { special_stamp: SpecialStamp, already_claimed: false }`, or `200` with `already_claimed: true` and the stamp they claimed before. `404` for a code no type has, `409` if the type's supply has run out.

## Shop

### `GET /api/shop/items`

`200` → `ShopItem[]`: active items only, by `position` then cheapest first, each with how many they've `bought`. The event ticket is the item with `kind: "ticket"`.

**Ticket batches.** A ticket with `batches` sells in batches: the first `batch_size` cost the first price, and each batch after costs `batch_step` more. The next batch goes on sale when one sells out, and `stock` counts what's left across all of them. The ticket's `price` here is what the signed-in person would pay now: the batch on sale, or their `locked_price` if that's lower.

**The launch sale.** Until 11:59 PM EST on November 18, 2026, when the community goal closes (`saleEnd` in `main.go`), everything except tickets and price locks is 10% off, rounded down but never below 1 stamp. Those items have a `sale`, and their `price` is the sale price, which is what buying charges. Prices go back to full when it ends, so a page showing sale prices should reload them at `ends_at`.

**Price locks.** The item with `kind: "price_lock"` fixes their ticket at the price of the batch on sale when they buy it, for whenever they buy the ticket. It's on top of the ticket's price, it doesn't hold a ticket, and it's one per person. If the tickets sell out before they buy one, the lock is cancelled and its stamps refunded automatically.

### `PATCH /api/me`

`{"name": "Max"}` changes the name the site shows (people click their name in the top bar). It's trimmed, inner runs of spaces become one, and it has to be 1 to 50 characters with nothing invisible in it. Signing in then leaves it alone; `{"name": null}` goes back to the name on their Hack Club account. `/api/me`'s `name_customized` says which it is. Their legal `first_name` and `last_name` don't change: the Unified YSWS DB and Airtable orders use those. Answers like `GET /api/me`.

### The Smugglers' Emporium

A second shop, hidden from everyone but reviewers, admins, and people an admin let in for a project worth rewarding (`emporium` on `/api/me` says who). `GET /api/emporium/items` lists it the way `GET /api/shop/items` lists the shop, and answers 404 to anyone else; its items aren't in the shop's list, and buying one without access answers 404 as if it didn't exist. The launch sale doesn't apply in the Emporium.

**Grants** (kind `grant`, Emporium only) are money for a project, sold in pieces: `grant_cents` a piece (default $5) at `grant_cents_per_hour` of approved time (default $6.50). A piece costs the stamps for the hours it's worth, rounded up, so with `STAMPS_PER_HOUR=30` a $5 piece is 24 stamps and $50 is 231. Buy them with `{"quantity": 10}` (1 to 200 pieces). `stock` counts pieces, so it caps the whole budget. The order's `grant_cents` is what it's worth; Airtable gets it in Stipend.

Admins let someone in with `PATCH /api/admin/users/{id}` `{"emporium_access": true, "emporium_note": "for Tiny Compiler"}`, and out with `false`, from the People tab or a ship's review page.

### `POST /api/shop/items/{id}/orders`

Buys one. For physical items, the body can give a shipping address:

```json
{
  "shipping_address": {
    "name": "Heidi Trashworth", "line_1": "15 Falls Rd", "line_2": "",
    "city": "Shelburne", "state": "VT", "postal_code": "05482", "country": "US"
  }
}
```

If it's left out, the address they gave Hack Club Auth is used. Name, `line_1`, city and country are required. Tickets and stipends take no body.

Buying needs a verified identity that Hack Club Auth says is eligible for YSWS programs. If what was saved at sign-in doesn't say that, the backend asks Hack Club Auth again first.

`201` → `Order` (status `pending`; a price lock is `fulfilled` straight away, with its `locked_price`). Their stamps go down by `price` straight away.

| Status | When |
|---|---|
| 403 | Not verified yet, verification still pending, or not eligible. The `error` says which. |
| 502 | Hack Club Auth couldn't be reached to check |
| 404 | No such item, or it's inactive |
| 422 | A physical item with no address (and none on file), an incomplete address, or an address on a non-physical item |
| 409 | Already have a ticket, already have `max_per_user` of it, or sold out |
| 409 | Price locks: already have one, already have a ticket, the ticket's price doesn't change, the tickets are sold out, or the last batch is on sale (the price can't go up) |
| 422 | Not enough stamps ("That costs 1800 stamps and you have 1500, so you need 300 more.") |

### `GET /api/orders`

`200` → `Order[]`, newest first. Their ticket is an order whose `item.kind` is `"ticket"` and whose status isn't `"cancelled"`.

## Reviewing (`/api/review`, reviewers and admins)

A ship goes through up to three stages before it counts. See [REVIEW_FLOW.md](REVIEW_FLOW.md) for why, and `backend/api/stages.go` for the rules.

| Stage (queue) | Status | Who decides | Approving it… |
|---|---|---|---|
| `review` | `pending` | `can_review` | decides the hours. Goes to `fraud_check` if anything flagged it, else to `awaiting_hq`, or straight to `approved` if the reviewer can also sign off. |
| `fraud` | `fraud_check` | `can_fraud_review` | passes it, maybe cutting hours (never adding). Goes to `awaiting_hq`. |
| `hq` | `awaiting_hq` | `can_hq_review` | signs it off at any hours up to `new_seconds`: `approved`, stamps and boxes paid, sent to Unified YSWS. |

Admins can decide every stage. Reviewers can see every queue, but only decide the stages they have a flag for. Nobody can decide their own project. At every stage a ship can be **rejected** (feedback required), and at the fraud and HQ stages it can be **sent back** to `pending` (a reason for the next reviewer required), which clears every decision on it; the timeline keeps them.

A ship goes to the fraud check when a fraud check fails or can't run (`fraud: true` below), when the reviewer ticks `send_to_fraud`, or when an admin has switched on `fraud_check_everything`.

```ts
interface Check {
  id: "hackatime_flagged" | "hackatime_dropped" | "shared_links" | "large_ship" | "long_day" | "identity";
  name: string;
  status: "pass" | "fail" | "inconclusive";
  severity: "info" | "warn" | "fail" | "critical"; // how bad it is when it doesn't pass
  summary?: string; // why, when it doesn't pass
  fraud: boolean;   // a fraud signal: failing or inconclusive sends it to the fraud check
}
interface Claim { user_id: number; name: string; expires_at: string }
```

### `GET /api/review/queue?stage=review|fraud|hq`

`stage` defaults to `review`. `200` →

```ts
interface Queue {
  counts: { review: number; fraud: number; hq: number };
  items: {
    id: number;            // submission ID
    project_id: number; project_name: string;
    user_id: number; user_name: string; slack_id: string | null;
    status: ReviewStatus;
    hackatime_projects: string[];
    new_seconds: number;
    approved_seconds: number | null; // the reviewer's number, at the later stages
    submitted_at: string; reviewed_at: string | null;
    reviewer_name: string | null;
    fraud_reasons: string[];
    override_reason: string | null; // approved for more than Hackatime tracked
    priority: boolean;     // a priority review (Express) stamp is on it
    claim: Claim | null;   // who's on it right now
    skipped_by_me: boolean;
    mine: boolean;         // their own project
  }[];
}
```

Priority review first, then oldest first. `422` for an unknown stage.

### `GET /api/review/stats`

The dashboard at the top of the queue: this week's leaderboard and the all-time one, and how the queue is doing. A **review** is one decision on a ship, its timeline event: approving or rejecting at the first review, passing or rejecting at the fraud check, signing off or rejecting at HQ, or sending back. Comments, notes and edits aren't reviews, and Sidekick decisions count the same. The admin's pay view below counts the same way. Weeks run Monday 12:00 AM to Monday 12:00 AM Eastern.

```ts
{
  week_start: string; week_end: string;
  week: Leader[]; all_time: Leader[]; // most reviews first; names only
  pace: {
    reviewed: number;              // reviews this week, by everyone
    shipped: number;               // ships submitted this week
    seconds_approved: number;      // hours signed off this week
    waiting: number;               // ships waiting for a first review now
    oldest_wait_seconds: number | null;
    median_wait_seconds: number | null; // for a first review, over the last 7 days
  };
}
Leader = { id; name; reviews; first_reviews; fraud_checks; hq_decisions; seconds_first_reviewed; last_review_at }
```

### `POST /api/review/next`

`{"stage": "review"}` claims the next ship in that queue for 30 minutes and answers `200 {submission_id, claim}`: their own live claim if they have one, otherwise the first ship (priority first, then oldest) that nobody's holding, isn't theirs, and they haven't skipped in the last 3 hours. `204` when there's nothing left. `403` for a stage they can't decide. Two reviewers asking at once never get the same ship.

### Claims

- **`POST /api/review/submissions/{id}/claim`** → `200 Claim`. Claims it, or renews their claim, for 30 minutes. `409` if someone else holds it ("Ana is reviewing this right now (until 14:05 UTC).") or it's been decided. `403` for their own project or a stage they can't decide.
- **`DELETE /api/review/submissions/{id}/claim`** → `204`. Lets go of their claim; admins can clear anyone's.
- **`POST /api/review/submissions/{id}/skip`** → `204`. Lets go, and Next won't give it to them for 3 hours.

Claims are advisory but enforced at decision time: nobody can decide a ship someone else holds. Deciding drops the claim. Opening the review page doesn't claim.

### `GET /api/review/submissions/{id}`

`200` →

```ts
interface ReviewDetail {
  submission: Submission;  // with the reviewer-only fields; status is a ReviewStatus
  stage: "review" | "fraud" | "hq" | ""; // "" once decided
  project: Project;        // its submissions have the reviewer-only fields too
  user: {
    id: number; name: string; email: string; slack_id: string | null;
    verification_status: string; ysws_eligible: boolean;
    reviewer_note: string | null; // see PUT /api/review/users/{id}/note
  };
  events: ShipEvent[];     // the project's whole timeline, oldest first
  checks: Check[];         // run just now
  projects_sharing_links: { id: number; name: string; repo_url: string | null; demo_url: string | null; user_id: number; user_name: string }[];
  hackatime: { seconds: number; projects: { name: string; seconds: number; project_id: null }[]; convicted: boolean } | null;
  hackatime_error?: string;
  claim: Claim | null;
  can_decide: boolean;     // whether the viewer can decide it right now
  cant_decide_reason?: string;
}

interface ShipEvent {
  id: number;
  submission_id: number;
  kind: "comment" | "internal_comment" | "flagged" | "approval" | "fraud_pass" | "sent_back" | "final_approval" | "rejection";
  stage: ReviewStatus;     // the ship's status when it happened
  actor: { id: number; name: string } | null; // null for "flagged", which the checks did
  seconds: number | null;  // approvals: the hours from then on
  message: string | null;  // what the participant sees
  internal: string | null; // reviewers only: justification, notes, reasons
  created_at: string;
  edited_at: string | null;
}
```

`404` if there's no such submission.

### `POST /api/review/submissions/{id}`

```json
{
  "action": "approve",
  "stage": "pending",
  "approved_seconds": 7200,
  "feedback": "Great work!",
  "internal_notes": "Hackatime matches the commits.",
  "send_to_fraud": false,
  "override_reason": "3 hours soldering the board, which Hackatime can't track."
}
```

- `action`: `approve`, `reject` or `send_back`. What approving means depends on the stage (table above).
- `stage`: optional, the status the reviewer was looking at. If the ship has moved on since, `409` instead of applying the decision to a stage they didn't mean. The review pages always send it.
- `approved_seconds`: approvals only. Defaults to all of `new_seconds` at the first stage and to the current `approved_seconds` after that. The fraud check can only lower it; HQ can set anything up to `new_seconds`.
- `feedback`: shown to the participant. Required to reject.
- `internal_notes`: reviewers only. Required to send back.
- `send_to_fraud`: first-stage approvals only.
- `override_reason`: approving more than Hackatime tracked (`new_seconds`) needs a reason, at the first stage or at HQ. At HQ the reviewer's reason carries over, so it's only needed for a new one. The fraud check can't go over. An approval over Hackatime always waits for HQ sign-off, even from someone who could sign off themselves. The reason isn't shown to the participant; it goes to HQ, the timeline and the Unified YSWS justification.

Signing off pays stamps for the approved time and grants a box for every milestone the person's approved total has now reached, all at once. Nothing is paid before that.

`200` →

```ts
interface ReviewResult {
  submission: Submission; // with the reviewer-only fields
  stamps_paid: number;    // 0 unless this signed it off
  boxes_granted: Box[];
}
```

| Status | When |
|---|---|
| 422 | Bad action, missing feedback or reason, hours out of range, or more than Hackatime tracked without an `override_reason` |
| 404 | No such submission |
| 403 | Their own project, or a stage they can't decide |
| 409 | Already decided, moved on from `stage`, someone else holds it, or sending back a ship that's already pending |

### Comments, notes and templates

- **`POST /api/review/submissions/{id}/comments`** `{"message": "Can you add a README?", "internal": false}` → `201 ShipEvent`. Not internal: the participant sees it on their project. `422` for an empty or 5,000+ character message, `403` on their own project.
- **`PUT /api/review/users/{id}/note`** `{"note": "Hackatime was broken until Nov 5"}` → `200 {note}`. A note about the person that reviewers see on all their ships. `""` clears it.
- **`GET /api/review/templates`** → `200 FeedbackTemplate[]` (`{id, title, body, created_by, updated_at}`), by title. **`POST`** `{title, body}` → `201`, **`PATCH /{id}`** `{title, body}` → `200`, **`DELETE /{id}`** → `204`. Shared by everyone who reviews. `422` for an empty title or body.

## Sidekick (`POST /api/sidekick`)

The master endpoint for [Sidekick](https://github.com/hackclub/sidekick), Hack Club's review platform, following its `docs/PROTOCOL.md`. It's only there when `SIDEKICK_SECRET` is set, and every request needs `Authorization: Bearer <SIDEKICK_SECRET>`. Errors are `{"error": "NOT_FOUND" | "CONFLICT" | "FORBIDDEN" | "VALIDATION_ERROR" | "INVALID_ACTION" | "UNAUTHORIZED", "message": "..."}`.

Supported actions: `HEALTH_CHECK`, `GET_PROGRAM_STATS`, `FETCH_PROJECTS`, `FETCH_PROJECT_DETAIL`, `FETCH_PROJECT_TIMELINE`, `FETCH_AUTHOR_PROJECTS`, `FETCH_USER_NOTE`, `UPDATE_USER_NOTE`, `SUBMIT_REVIEW_ACTION`, `UPDATE_REVIEW_ACTION`. The shop actions answer `INVALID_ACTION`: orders are fulfilled through Airtable.

How Crossroads maps onto it is in [REVIEW_FLOW.md](REVIEW_FLOW.md#sidekick). The short version: `fraud_check` and `awaiting_hq` both look like `pending_hq`, Sidekick decisions go through the same rules as the review pages, and the reviewer has to have signed in to Crossroads once.

## Admin (`/api/admin`, admins only)

Admin user shape, used by several routes below:

```ts
interface AdminUser {
  id: number; name: string; email: string; slack_id: string | null;
  role: Role; stamps: number; verification_status: string; ysws_eligible: boolean;
  can_review: boolean; can_fraud_review: boolean; can_hq_review: boolean; // review stages; admins can do all
}
```

### People

**`GET /api/admin/users?q=heidi`** → `200 AdminUser[]`. Matches part of a name or email, or a whole Slack ID. Without `q`, the newest 50 people.

**`PATCH /api/admin/users/{id}`** with any of `{"role": "reviewer", "can_review": true, "can_fraud_review": false, "can_hq_review": false}` → `200 AdminUser`. Fields left out don't change. `422` for an unknown role, an admin removing their own admin role, or review stages on a participant. `404` for no such user. Changes apply on the person's next request.

### Reviewer stats (what reviewers are paid by)

`GET /api/admin/reviewer-stats?from=2026-11-09&to=2026-11-15` counts each person's reviews (as defined under `GET /api/review/stats`) for those days, both included, midnight to midnight Eastern. Without `from` and `to` it's this week. Each row has `name`, `email`, `slack_id`, `reviews` (the total), `first_reviews` with `first_approvals` and `first_rejections`, `fraud_checks`, `hq_decisions`, `later_sent_back` (first reviews whose approval was later sent back; they still count), `seconds_first_reviewed` and `last_review_at`. Ships can't be deleted and events never are, so a finished period's counts never change.

`GET /api/admin/reviews?from=&to=[&reviewer=ID]` lists the reviews behind the counts, oldest first. Add `&format=csv` to either for a spreadsheet; cells a spreadsheet would read as a formula get a leading apostrophe.

### Review settings

**`GET /api/admin/review-settings`** → `200`, and **`PATCH`** with any of the fields → `200`:

```ts
interface ReviewSettings {
  fraud_check_everything: boolean; // every approved ship goes through the fraud check
  large_ship_seconds: number;      // ships claiming more than this get a fraud check (default 20h)
  long_day_seconds: number;        // so do ships with a UTC day over this on the project (default 14h)
  updated_at: string;
}
```

`422` unless `large_ship_seconds` is over 0 and `long_day_seconds` is 1 to 86,400. Changes apply to approvals from then on.

**`POST /api/admin/users/{id}/stamp-adjustments`** with `{"amount": -300, "note": "clawback: copied project"}` → `201 AdminUser` (with their new balance). The amount can be negative, and can take them below zero. A note is required (1,000 characters max). `422` for a zero amount or no note, `404` for no such user.

**`POST /api/admin/users/{id}/boxes`** with `{"box_type_id": 1}` → `201 Box`. `404` for no such user, `422` for no such box type.

**`POST /api/admin/users/{id}/special-stamps`** with `{"special_stamp_type_id": 1}` → `201 SpecialStamp`. Comes out of the type's supply: `409` if it's run out. `404` for no such user, `422` for no such type.

### Role invites

For giving someone a role before they've ever signed in.

```ts
interface RoleInvite {
  id: number; email: string; role: "reviewer" | "admin";
  invited_by_name: string | null; // null if that admin's account is gone
  created_at: string;
}
```

**`GET /api/admin/role-invites`** → `200 RoleInvite[]`, newest first.

**`POST /api/admin/role-invites`** with `{"email": "orpheus@hackclub.com", "role": "admin"}` → `201 RoleInvite`. The first time someone signs in whose Hack Club Auth email matches (ignoring case), they get the role and the invite is used up, so a later role change sticks. Inviting the same email again replaces its role. `422` for something that isn't a bare email address, or a role other than `reviewer` or `admin`. `409` if someone with that email has already signed in: change their role with `PATCH /api/admin/users/{id}` instead.

**`DELETE /api/admin/role-invites/{id}`** → `204`. `404` if it doesn't exist, including once it's been used.

### Special stamp types

```ts
interface AdminSpecialStampType extends SpecialStampType {
  supply_cap: number | null; // null is unlimited
  issued_count: number;      // how many exist
  applied_count: number;     // how many have been used on submissions (list only; 0 elsewhere)
  claim_code: string | null; // anyone signed in can claim one at /claim/{code}; null if not
}
```

**`GET /api/admin/special-stamp-types`** → `200 AdminSpecialStampType[]`.

**`POST /api/admin/special-stamp-types`**:

```json
{ "name": "Express", "description": "Skip the line", "image_url": null,
  "rarity": "rare", "effect": "priority_review", "supply_cap": 25 }
```

`name` and `rarity` are required. `effect` defaults to `"none"`. `supply_cap` is *nullable* (null or left out = unlimited). `claim_code` is lowercased, can only have letters, numbers and dashes, and has to be unique (`409` if another type has it); null or `""` means it can't be claimed. → `201 AdminSpecialStampType`. `422` for invalid fields.

**`PATCH /api/admin/special-stamp-types/{id}`**: any of the same fields. → `200`. Lowering `supply_cap` below `issued_count` just stops further drops; nobody loses theirs. `409` if capping it would leave a box type with no unlimited stamp type, since that box could then come up empty. `404` for no such type.

### Box types

```ts
interface AdminBoxType {
  id: number; name: string; description: string; image_url: string | null;
  drops: { special_stamp_type_id: number; weight: number }[]; // including types that have run out
}
```

**`GET /api/admin/box-types`** → `200 AdminBoxType[]`.

**`POST /api/admin/box-types`** with `{"name": "Starter box", "description": "", "image_url": null}` → `201 AdminBoxType` with no drops. Set its drops before handing any out.

**`PATCH /api/admin/box-types/{id}`**: name, description, image_url (*nullable*). → `200`.

**`PUT /api/admin/box-types/{id}/drops`** replaces the whole drop table:

```json
[ { "special_stamp_type_id": 1, "weight": 90 },
  { "special_stamp_type_id": 2, "weight": 10 } ]
```

A type's chance is its weight over the total weight of types that still have supply. → `200 AdminBoxType`. `422` if a weight isn't above 0, a type is listed twice or doesn't exist, or none of the types is unlimited.

### Milestones

```ts
interface Milestone { id: number; name: string; approved_seconds: number; box_type_id: number }
```

**`GET /api/admin/milestones`** → `200 Milestone[]`, lowest first.

**`POST /api/admin/milestones`** with `{"name": "First 10 hours", "approved_seconds": 36000, "box_type_id": 1}` → `201 {"milestone": Milestone, "boxes_granted": number}`. Everyone already past it gets the box straight away; `boxes_granted` says how many did. `409` if there's already a milestone at that many seconds, `422` for a bad name, non-positive seconds or no such box type.

### Shop items

**`GET /api/admin/shop-items`** → `200 ShopItem[]`, including inactive ones, in shop order.

**`POST /api/admin/shop-items`**:

```json
{ "kind": "ticket", "name": "Crossroads ticket", "description": "", "image_url": null,
  "price": 1800, "stipend_cents": null, "stock": null, "max_per_user": 1, "active": true }
```

`kind`, `name` and `price` are required. Stipends need `stipend_cents`, and nothing else can have it. Tickets and price locks are always one per person (`max_per_user` defaults to 1 for them, and anything else is rejected). A ticket can sell in batches with `batch_size` and `batch_step` (both or neither, both above 0); its `price` is then the first batch's, and it needs a `stock`, the total across every batch. For Crossroads: price 1800, stock 30, `batch_size` 6, `batch_step` 150, which is 60 hours up to 80. The price lock is `{"kind": "price_lock", "name": "Price lock", "price": 150}`. `stock`, `max_per_user` and `image_url` are *nullable* (null = unlimited / none). `active` defaults to true and `position` to 0. → `201 ShopItem`.

**`PATCH /api/admin/shop-items/{id}`**: any of the same fields. → `200`. Price changes don't affect existing orders. Set `active: false` to take something out of the shop.

### Orders

```ts
interface AdminOrder extends Order {
  user: AdminUser;
  notes: string | null; // admin notes
}
```

**`GET /api/admin/orders?status=pending`** → `200 AdminOrder[]`, oldest first. `status` is optional.

**`PATCH /api/admin/orders/{id}`** with `{"status": "fulfilled" | "cancelled", "notes": "shipped 11/10"}` → `200 Order`. Only pending orders can change (`409` otherwise). Cancelling refunds the stamps and puts the item back in stock. Orders can also be fulfilled by ticking **Fulfilled** in the Shop Orders table in Airtable (see INTERNALS.md, "Airtable sync").

### Activity

**`GET /api/admin/activity?days=30&min_minutes=0`** → `200`:

```ts
interface Activity {
  days: {
    day: string;                 // "2026-11-06", a UTC day; the last one is today
    active_participants: number; // participants whose projects got at least min_minutes between them
    active_projects: number;     // projects with any time, participants' only
    seconds: number;             // participants' total project time
    visitors: number;            // participants who made a signed-in request
    sign_ups: number;            // participants who signed up
    participants: number;        // participants signed up by the end of the day
  }[];
  range: {                       // each participant counted once over all the days returned
    visitors: number;            // used the platform on at least one of them
    active_participants: number; // active (min_minutes) on at least one of them
  };
  min_minutes: number;
  updated_at: string | null; // when the poller last recorded anything
}
```

Project numbers count Hackatime time on the Hackatime projects linked to a project, recorded per UTC day by the activity poller. Visitors are recorded on each person's first signed-in request of the UTC day (see INTERNALS.md, "Activity"). `min_minutes` only affects the project numbers. Reviewers and admins aren't counted, and nor is time on Hackatime projects nobody has linked. `days` is 1 to 366 (default 30) and doesn't go back before Hackatime time starts counting; `min_minutes` is 0 to 1440 (default 0, any time). `422` for anything else.

## Placed stamps

Special stamps leave the owner's **stamp box** by being stuck on one of their projects. Once on a project they stay there for good: the owner can move them around that project, but can't take them off or move them to another project. (Deleting a never-shipped project puts its stamps back in the box.)

```ts
interface PlacedStamp {
  id: number;
  type: SpecialStampType;
  x: number;        // 0-1 across the project's stamp board (its screenshot card)
  y: number;        // 0-1 down it
  rotation: number; // degrees, set when it's stuck on
  submission_id: number | null; // the ship an Express stamp gave priority to
  placed_at: string;
}
```

`SpecialStamp` (from `GET /api/special-stamps`) also has `project_id: number | null`: null means it's in the stamp box. `GET /api/projects/{id}` returns the project's `stamps: PlacedStamp[]`. Nothing in `PlacedStamp` is private, so it's meant to be shown to anyone viewing the project once project pages are public.

**`POST /api/special-stamps/{id}/placement`** with `{"project_id": 3, "x": 0.4, "y": 0.2, "rotation": -6}` → `200 PlacedStamp`. The first call sticks it on (x and y are clamped to 0-1, rotation to ±45 and only taken then). Later calls with the same `project_id` move it. `404` if it isn't their stamp or project, `409` if it's on another project or was used on a ship before placement existed.

Effects, from when the stamp goes on:
- **priority_review** (Express): the project's ship in review gets priority review; if nothing is in review, the next ship does. One Express per ship.
- **stamp_multiplier** (Golden Spike): every approved ship of the project pays 10% more stamps (rounded), as a separate `bonus` ledger entry and included in `stamps_paid`. More than one spike is still 10%.
- **none**: a collectible; it's only shown on the project.

The old `special_stamp_id` on `POST /api/projects/{id}/submissions` still works, but the site now uses placement instead.

## Community goal

The home page's "Repair Bridge": the in-person event only runs if participants together ship enough hours between launch and the deadline.

**`GET /api/community`** → `200`:

```ts
interface Community {
  seconds: number;      // shipped so far
  goal_seconds: number; // what's needed (500 hours)
  starts_at: string;    // launch: when Hackatime time starts counting
  ends_at: string;      // the deadline, 11:59 PM EST on November 18
}
```

`seconds` is the `new_seconds` of every participant's submission made between `starts_at` and `ends_at`. A ship counts as soon as it's submitted, without waiting for review, and drops out if it's rejected; a reviewer cutting its hours doesn't change it. The goal and deadline are `communityGoal` in `backend/main.go`.


## Slack clicks

### `POST /slack/clicks`

For the Slack bot (`slackbot/`), not the frontend. Not under `/api`, and the public ingress doesn't route to it; the bot calls the backend inside the cluster. Only mounted when `SLACK_CLICKS_SECRET` is set, and needs `Authorization: Bearer <SLACK_CLICKS_SECRET>` (`401` otherwise).

```json
{ "slack_id": "U123", "button": "open_platform", "channel_id": "C123", "message_ts": "1731340000.000100" }
```

`slack_id` and `button` (the button's `action_id`) are required. → `204`, and a row in `slack_clicks`.
