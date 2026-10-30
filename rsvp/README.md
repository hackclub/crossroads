# crossroads rsvp

The RSVP page for Crossroads: a ticket machine you type your email and name into. It's a static page (`index.html`, `style.css`, `script.js`) plus one Vercel function (`api/rsvp.js`) that writes to the **Event RSVP** table in the **YSWS - Crossroads** Airtable base.

## Setup

1. Create an Airtable personal access token with `data.records:read` and `data.records:write` on the YSWS - Crossroads base.
2. Set it as `AIRTABLE_TOKEN` in the Vercel project's environment variables (see `.env.example`).
3. Deploy to Vercel. There's no build step.

To run it locally, put the token in `.env.local` and run `npx vercel dev`.

## Notes

- RSVPs are keyed by email: submitting the same email again updates the name instead of adding a row.
- Layout positions in `index.html` are design pixels from the Figma frame (1728×2036). `style.css` scales them, and on screens narrower than 1000px it zooms in on the form.
- `assets/screen.png` and `assets/ticket.png` are baked from Figma's dither shader. Re-export them from Figma if the design changes.
