import adapter from '@sveltejs/adapter-node';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// Orchard runs `node build`. Not adapter-auto: it only picks adapter-node
			// after Vite has built the server, so adapter-node's entry
			// (server/adapter-index.js) never gets built and `node build` crashes.
			adapter: adapter()
		})
	],
	server: {
		// The backend has no CORS and uses a same-site cookie, so the browser has
		// to see the frontend and the backend on one origin. See
		// planning/INTEGRATING.md. /dev only exists when the backend has DEV_MODE=1.
		proxy: {
			'/api': 'http://localhost:8080',
			'/auth': 'http://localhost:8080',
			'/uploads': 'http://localhost:8080',
			'/dev': 'http://localhost:8080'
		}
	}
});
