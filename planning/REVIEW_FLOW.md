# The review flow

How a ship goes from "submitted" to "paid", who decides each step, and how Sidekick fits in. For the routes, see [ROUTES.md](ROUTES.md#reviewing-apireview-reviewers-and-admins); for the code, `backend/api/stages.go` (the rules), `review.go` (the review pages' routes) and `sidekick.go` (Sidekick's protocol).

It borrows from two Hack Club projects: [Sidekick](https://github.com/hackclub/sidekick), a separate review platform programs plug into, and [Stardance](https://github.com/hackclub/stardance), a program with review built in. Crossroads does both. It has its own review pages, and it also serves Sidekick's protocol, so reviewers can use either and the same rules apply.

## The stages

```
            approve                 pass (may cut hours)        sign off
 pending ───────────▶ fraud_check ─────────────────────▶ awaiting_hq ─────────▶ approved
    │   \  (flagged)        │                                 │                 stamps, boxes,
    │    \                  │                                 │                 Unified YSWS
    │     └── approve, nothing flagged ───────────────────────┘
    │         (reviewer can also sign off ──────────────────────────────────────▶ approved)
    │
    └── reject ──▶ rejected ◀── reject ── (fraud_check, awaiting_hq)
                  fraud_check and awaiting_hq can also "send back" to pending
```

1. **Review** (`pending`, needs `can_review`). A reviewer tries the project and decides the hours, or rejects it with feedback. The automatic checks run at this moment.
2. **Fraud check** (`fraud_check`, needs `can_fraud_review`). Only for ships that need it (below). The fraud reviewer passes it, cuts hours (never adds), rejects it, or sends it back to review.
3. **HQ sign-off** (`awaiting_hq`, needs `can_hq_review`). HQ signs it off at any number of hours up to what was claimed, rejects it, or sends it back. **Signing off is the only thing that pays**: stamps, milestone boxes, and the row in the Unified YSWS DB.

A reviewer who can also sign off (an admin, or someone with `can_hq_review`) skips the HQ stage when nothing flags the ship: their approval is the sign-off. A flagged ship always goes through the fraud check first, whoever approved it.

**Sent back** clears every decision on the ship and puts it back in the review queue. The timeline keeps the old approval, and the person sending it back has to say why.

Participants see three states: **In review** (`pending`), **Approved, final check** (`fraud_check` or `awaiting_hq`: a reviewer approved it and it's waiting to be signed off), and **Approved** or **Rejected**. They aren't told when a ship is at the fraud check. They see the feedback on each decision and any comment a reviewer marks for them.

## What sends a ship to the fraud check

When the first reviewer approves, any of these sends it to `fraud_check`:

| Check | Fails when |
|---|---|
| `hackatime_flagged` | Hackatime's trust level is red, **or Hackatime couldn't be asked** |
| `hackatime_dropped` | Hackatime has less time on the ship's projects now than when it was shipped (deleted heartbeats) |
| `shared_links` | Another project, anyone's, uses the same repo or demo link |
| `large_ship` | It claims more than the large-ship limit (default 20h) |
| `long_day` | The project had a UTC day over the long-day limit (default 14h) since its last approved ship |
| `manual` | The reviewer ticked "send to the fraud check" |
| `everything` | An admin turned on "send every ship through the fraud check" |

`identity` (not YSWS-eligible) is shown but isn't a fraud signal. The ship can still be approved; the Unified YSWS submission waits until they're verified.

Admins change the toggle and both limits on the admin page (Review tab), or with `PATCH /api/admin/review-settings`. Changes apply to approvals from then on; ships already past the first stage stay where they are.

## Who can do what

People keep their role (`participant`, `reviewer`, `admin`). On top of that, each reviewer has three flags, set on the admin page: **Review**, **Fraud check** and **HQ sign-off**. Admins can do everything. A reviewer with no flags can see the queues but not decide anything. Nobody can decide their own project.

Existing reviewers were given **Review** by the migration, and nothing else. An admin has to tick the fraud and HQ flags for whoever should have them.

## Working through a queue

- **Next** claims the next ship in a queue for 30 minutes: your own claim first, then priority review (Express stamps), then oldest first, skipping ships someone else holds and ones you've skipped in the last 3 hours. Two reviewers pressing Next at once never get the same ship.
- **Claims** can be renewed, released, or skipped. Nobody can decide a ship someone else holds; claims run out on their own.
- **Comments** go on a ship's timeline, either for the participant or for reviewers only. There's also a reviewers-only **note on the person**, shown on all their ships (Sidekick shows and edits the same note).
- **Feedback templates** (`/review/templates`) are snippets anyone who reviews can drop into feedback. They're shared and anyone can edit them.

## Stamps and the Unified YSWS DB

Nothing is paid until a ship is signed off. Then, in the same transaction, the backend pays stamps on the person's running total, adds the Golden Spike bonus, and grants milestone boxes, exactly as before.

The Airtable sync copies signed-off ships into **YSWS Project Submission** with a justification that names everyone who checked the hours: Hackatime's numbers, the reviewer's hours and notes, the fraud check (and what flagged it), and HQ's sign-off. It **ticks "Automation - Submit to Unified YSWS"** on rows HQ signed off, as soon as the person is YSWS-eligible; rows for people who aren't eligible yet are ticked on a later pass, once they are. Ships approved before the stages existed have no HQ sign-off, so the sync leaves them for a person to tick, as before.

**Dev mode never syncs.** With `DEV_MODE=1` the backend won't start the Airtable sync or the Sidekick endpoint, even if `AIRTABLE_TOKEN` or `SIDEKICK_SECRET` is set. Test ships can't reach Airtable or the Unified YSWS DB.

## Sidekick

`POST /api/sidekick` serves Sidekick's protocol (`docs/PROTOCOL.md` in the Sidekick repo). To turn it on:

1. Set `SIDEKICK_SECRET` on the backend to a long random string. The endpoint doesn't exist without it.
2. Ask a Sidekick super admin to make you a program author. Then create the Crossroads program in Sidekick with the master endpoint `https://<crossroads>/api/sidekick` and the same secret. "Test" should succeed (`HEALTH_CHECK`).
3. In the program's API config, **turn on "Report held approvals upstream."** Approvals then reach Crossroads as soon as they're made, and Crossroads' stages stay the single source of truth. Without it, Sidekick holds non-HQ approvals on its side and sends them later.
4. Invite reviewers in Sidekick. **Each one has to sign in to Crossroads once**, so their decisions can be credited to an account; Sidekick's reviewer IDs are matched to Crossroads accounts by Slack ID or Hack Club Auth ID.

How the stages look in Sidekick:

| Crossroads | Sidekick | Notes |
|---|---|---|
| `pending` | `pending` | Approving offers a "Send to Crossroads' fraud check" checkbox |
| `fraud_check` | `pending_hq` | Tagged **Fraud check**, with a failing "Waiting for Crossroads' fraud check" check. Authorize answers 409 until a Crossroads fraud reviewer passes it. |
| `awaiting_hq` | `pending_hq` | Tagged **Awaiting HQ**. Authorize signs it off; deauthorize sends it back. |
| `approved`, `rejected` | the same | |

Sidekick vouches for its own reviewers' permissions: an `isHq` approval counts as a sign-off (if nothing flags it), and `authorize` and `deauthorize` are trusted to come from HQ. The fraud check is the one stage only Crossroads' review pages can decide.

Not supported: rewarded hours overrides (refused with 400), and the shop and fulfilment actions (`INVALID_ACTION`; orders are fulfilled through Airtable).

## Decisions worth revisiting

- **HQ reviewers skip HQ sign-off on unflagged ships.** That matches Sidekick's `isHq`. If every ship should get a second person, change the `perms.HQReview` case in `decide` to always go to `awaiting_hq`.
- **Hackatime being down sends ships to the fraud check.** That's the safe choice, but an outage during a busy day fills the fraud queue.
- **The limits** (20h per ship, 14h per day) are guesses. Change them on the admin page.
- **The fraud check is hidden from participants.** They see "final check" for both later stages.
