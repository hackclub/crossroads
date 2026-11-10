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

**More than Hackatime tracked.** At the first stage and at HQ, a reviewer can tick "Approve more than Hackatime tracked" and approve more hours than the ship claimed, for work Hackatime can't see (soldering, design, testing hardware). They have to say why. An approval over Hackatime **always waits for HQ sign-off**, even from someone who could sign off themselves, and HQ sees the reason first. The fraud check can only cut. The reason goes on the timeline and into the Unified YSWS justification, but not to the participant. In Sidekick, the same thing is the "Why more hours than Hackatime tracked?" field when approving. Going over doesn't change the time-accounting: the next ship still only claims Hackatime time nobody has claimed yet.

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

| `unified_double_dip` | The repo or demo is already in the Unified YSWS DB from another program (needs `UNIFIED_AIRTABLE_TOKEN`; Sidekick's AT1) |

The other checks are for the reviewer to judge and don't send a ship anywhere. Most are Sidekick's:

| Check | Fails when | Sidekick |
|---|---|---|
| `repo_works` | The repo link isn't a link, doesn't open, or GitHub says the repo doesn't exist or is private | D2, D3, G1 |
| `demo_works` | The demo link isn't a link, doesn't open, or is on a notebook or puzzle site (Colab, Kaggle, Binder, LeetCode) | D2, D4 |
| `readme` | The repo has no README, only a title, or a project generator's template | G2 |
| `new_commits` | No commits since the last approved ship, or since the event started for a first ship | G3 |
| `full_name` | No first or last name on file, or one that's a placeholder or has digits or symbols | |
| `unified_ready` | Something the Unified YSWS DB needs is missing (below) | D1 |
| `identity` | They aren't verified and YSWS-eligible | D5 |

A link that answers 401, 403 or 429, or takes too long, is "couldn't check" rather than broken: plenty of sites block scripts. Check those by hand. The `readme` and `new_commits` checks need the repo on GitHub.

**What the Unified YSWS DB needs** (`unified.Missing`): a real full name, email, birthday, an address with street, city, postal code and country, a description, a screenshot, a repo on GitHub (it asks for a GitHub username) and a demo link, plus YSWS eligibility. A ship missing any of these can still be approved and signed off; the Airtable sync copies its row but **doesn't tick "Automation - Submit to Unified YSWS" until it's complete**, then sends the whole row again with the box ticked.

## What reviewers see

Besides the checks, the review page shows:

- **Code**: the repo on GitHub (language, created, last push, all-time commits), and for this ship (since the last approved ship, or the event's start, until it shipped) the commits, lines added and removed, who made them, and each commit with a link to it. Line counts need `GITHUB_TOKEN` and cover the newest 60 commits.
- **Hackatime hours and commits per day** over the same window, as two small charts with a table view. Daily totals only: **reviewers never see heartbeats**.
- On the fraud check, a link to the person's **Telescreen** profile, when `TELESCREEN_PROFILE_URL` is set.

GitHub answers are cached for 10 minutes and link checks for 5, so opening a ship and then approving it doesn't ask twice.

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

- **A repo has to be on GitHub to go to the Unified YSWS DB automatically**, because the DB asks for a GitHub username. Projects on GitLab or Codeberg wait for a person. Drop that line from `unified.Missing` if the DB takes them without one.
- **Broken links and missing READMEs don't send ships to the fraud check**; they're for the reviewer. To make one a fraud signal, set `Fraud: true` on it in `api/checks.go`.
- **The Telescreen link format** is a guess until someone sets `TELESCREEN_PROFILE_URL` to the real one.

- **HQ reviewers skip HQ sign-off on unflagged ships.** That matches Sidekick's `isHq`. If every ship should get a second person, change the `perms.HQReview` case in `decide` to always go to `awaiting_hq`.
- **Hackatime being down sends ships to the fraud check.** That's the safe choice, but an outage during a busy day fills the fraud queue.
- **The limits** (20h per ship, 14h per day) are guesses. Change them on the admin page.
- **Overrides have no upper limit**, only a required reason and HQ sign-off. If you want a cap (say, Hackatime's hours plus 50%), it goes in `checkOverride` in `stages.go`.
- **The fraud check is hidden from participants.** They see "final check" for both later stages.
