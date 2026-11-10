# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **Participants:** teenagers aged 13–18, anywhere in the world. They build technical projects (apps, games, tools, hardware), track their time with Hackatime, and ship projects for review to earn a place on the Crossroads trip. They usually open the platform while they're coding, on a laptop.
- **Reviewers:** Hack Club people who work through the review queues, try each project, and approve or reject its hours. A ship goes through review, a fraud check when something flags it, and HQ sign-off before anything is paid (see `planning/REVIEW_FLOW.md`). Same site, extra pages, or Sidekick, Hack Club's review platform.
- **Admins:** run the program: people and roles, stamp adjustments, special stamp types, boxes, milestones, shop items and orders.

## Product Purpose

Crossroads is a Hack Club YSWS ("You Ship, We Ship"). Participants build for 60 approved hours, then travel across Europe by train together in February 2027, leaving from Berlin, with 30 spots. The platform is where they create projects, link Hackatime, ship for review, watch their approved hours grow, and spend stamps. Success is a participant knowing at every moment how far they are from the trip and what to do next.

## Positioning

The reward is a train journey across Europe with other teen builders, not a prize. Qualifying means buying the ticket in the shop with stamps (1,800 stamps = 60 approved hours at 30 per hour), so spending stamps on other items delays the trip.

## Operating Context

- Sign-in is Hack Club Auth only; it also supplies name, Slack ID, birthday, address and identity verification, so the platform never asks for them.
- Time comes from Hackatime, counted from midnight UTC Nov 3, 2026. Only approved hours count, totaled across all projects.
- Reviewers can approve fewer hours than tracked; rejected time can be shipped again, cut time can't. Stamps are paid only once HQ signs a ship off.
- Help happens on Slack in #crossroads-help.
- Backend: Go API in `backend/` (routes in `planning/ROUTES.md`). Postgres is the source of truth; approved projects and orders are copied to Airtable.

## Capabilities and Constraints

- Projects: name, description, repo, demo and screenshot links, linked Hackatime projects. Shipping needs all of them. One pending submission per project; projects can't be edited while in review.
- Stamps: the currency, 30 per approved hour. The shop sells the ticket (one per person), physical items and travel stipends. Buying needs a verified, YSWS-eligible Hack Club identity.
- Boxes: one per approved-hours milestone. Opening one drops a special stamp; some special stamps have an effect (today only priority review). Boxes are earned, never sold, and their odds are always shown because participants are minors.
- Terminology to keep: **stamps**, **boxes**, **special stamps**, **ship / shipping**, **Hackatime**.
- Undecided: trip itinerary beyond Berlin, whether the leaderboard is opt-in, team projects, the exact milestones and shop catalogue.

## Brand Commitments

- Hack Club, written exactly that way.
- The dark film-strip / train-ticket look from the `crossroads-draft` Figma file and the landing page (`platform/src/routes/+page.svelte`), with the Hack Club flag pinned top left.
- Names fixed: Crossroads, stamps, boxes, special stamps.

## Evidence on Hand

- Figma `crossroads-draft` (SLiFzmgzcIoQKyTeTvvt5T): landing page drafts, and platform frames **home**, **project edit**, **project main**.
- Landing page: `platform/src/routes/+page.svelte` (the `/` route; assets in `platform/src/lib/assets/landing/`). RSVP page: `rsvp/`.
- No real participant projects, testimonials or photos yet; don't invent them.

## Product Principles

1. Always show the distance to the trip and the single next step.
2. Never let a participant wonder why they can't ship or buy something; say what's missing.
3. Honesty over hype: show reviewer feedback, box odds, and why hours were cut.
4. The same rules for everyone are enforced by the backend; the UI explains them, it doesn't re-decide them.

## Accessibility & Inclusion

Participants are minors from many countries, often non-native English speakers: plain language, no idioms in instructions, and readable contrast on the dark theme.
