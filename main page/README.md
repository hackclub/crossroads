# crossroads main page

The landing page for Hack Club Crossroads, built with SvelteKit from the "MacBook Pro 16" - 6" frame in the crossroads-draft Figma file.

```sh
npm install
npm run dev
```

## Notes

- Positions in `src/routes/+page.svelte` are design pixels from the 1728×3060 Figma frame. `--u` scales them to the viewport width. Below about 780px wide the page stops shrinking and crops the sides instead.
- The `film-*.png` strips in `src/lib/assets` are baked from Figma's film shader at 2×. Re-export them from Figma if the design changes. The left strips include the shader's bleed past the strip's edge.
- Both "Get Started" buttons link to `#` for now.
