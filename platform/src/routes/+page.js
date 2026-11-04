// The landing page needs no signed-in data, so render it on the server and at
// build time (the app pages are browser-only; see +layout.js).
export const ssr = true;
export const prerender = true;
