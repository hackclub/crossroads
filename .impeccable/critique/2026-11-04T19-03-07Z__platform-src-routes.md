---
target: the Crossroads platform
total_score: 24
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 4
target_identity: "file:/home/max/Documents/git repos/crossroads/platform/src/routes"
timestamp: 2026-11-04T19-03-07Z
slug: platform-src-routes
---
# Critique: Crossroads platform (platform/src/routes)

Method: dual-agent (A: design review · B: detector + browser evidence)

## Design Health Score: 24/40 (Acceptable)
| # | Heuristic | Score | Key issue |
|---|---|---|---|
| 1 | Visibility of System Status | 2 | Hours bar segments 1.37/1.46:1, tile order reversed vs bar, no notice of approvals/boxes |
| 2 | Match System / Real World | 3 | Plain copy; one state has 3 names; enum values leak |
| 3 | User Control and Freedom | 2 | Admin role select commits on change; cut-hours approval no confirm |
| 4 | Consistency and Standards | 2 | Save weaker than Cancel; mixed title alignment; 60H vs 14h; invisible chips on review cards |
| 5 | Error Prevention | 2 | Shop's only lit CTA is stickers; confirm doesn't mention ticket |
| 6 | Recognition Rather Than Recall | 3 | Form doesn't mark ship-required fields |
| 7 | Flexibility and Efficiency | 2 | No ship from home; no next-in-queue for reviewers |
| 8 | Aesthetic and Minimalist Design | 3 | Single-weight font flattens shop/boxes hierarchy; screenshot pushes Ship below fold |
| 9 | Error Recovery | 3 | Dev ":8080" copy; unlinked verify URL |
| 10 | Help and Documentation | 2 | "Docs" is an FAQ; Hackatime never explained/linked |

## Design Specificity
Shell is specific (film-strip nav, flag, Cute Font, Figma home/project/form); contents are category-interchangeable (tables, text cards, callouts). Ticket, stamps, boxes and the trip have no visual identity; PRODUCT.md's train-ticket look is absent.
Detector: source scan 0 (regex-only for .svelte). URL scans: low-contrast warning `.field em` on --head 2.74:1 (ProjectForm.svelte:62, also review/admin); low-contrast warning placeholders --faint 2.58:1 (admin search unlabelled, People.svelte:53); low-contrast advisory --muted on --body 4.38:1 (project, edit, shop); nested-cards at 390 on Ship panel callout (partly false positive). No slop rules. Overlay unavailable (no browser automation tool).

## Priority Issues
- [P1] Home doesn't show distance to trip or next step (bar unreadable, hours vs stamps disagree, no "Next:" line). Fix: reorder tiles, pattern-distinguished segments, stamps-to-ticket line in hero, derived Next action. /impeccable clarify, /impeccable layout
- [P1] Contrast/focus: small/translucent text on --head 2.4–2.7, placeholders 2.58, row hover 2.92, focus ring 1.34 on grey, disabled shop info 2.85. Fix: solid ink on --head, raise --faint, darker hover, white+black focus ring, info as text beside disabled control, label admin search. /impeccable audit, /impeccable polish
- [P1] Shop steers away from the trip: ticket last and same size as stickers; sticker is the only lit CTA; generic confirm. Fix: ticket first as ticket-shaped hero card; others secondary until owned; confirm states ticket cost. /impeccable shape, /impeccable layout
- [P1 admins] Irreversible one-input actions: role select commits on change (People.svelte:71); cut-hours approve no confirm. Fix: draft + Save role; confirm cut. /impeccable harden
- [P2] Peak moments are text callouts (box drop, ticket bought "pending", approvals invisible). Fix: box reveal with rarity colour; ticket card replaces home hero; "+N stamps" notice. /impeccable delight

## Persona Red Flags
- First-timer: zeros-only home; only action is 3.36:1 grey New Project; empty state not clickable; no Hackatime install link; ship-required fields unmarked.
- Keyboard/screen reader: invisible focus on grey; no live regions for results; panel heads not headings; role select on arrow keys; every tab titled "Crossroads"; 15px-tall top-bar links at 390.
- 14-yo non-native speaker: Cute Font D≈O ("Oocs"); 21px renders ~13px; Latin-only subset (ł ş č Cyrillic, arrows/✓ fall back); three names for one state; "Used up either way".

## Minor Observations
Invisible chips + broken screenshot alt on review detail; ship-card badges lose colour; Delete chip same as Edit; admin subtitle exposes repo path; ":8080" copy; Ship panel ~y=1100 at 1000px; at 390 "Not Submitted" wraps, ticks collide, admin table scrolls; dev overlays cover content (dev-only); shop grid leaves quarter-width hole.

## Questions to Consider
- Should the ticket be the home hero rather than a shop card?
- Hours or stamps: which single number?
- What would boxes and special stamps look like as real stamps on a ticket stub?
