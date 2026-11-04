import adapter from '@sveltejs/adapter-auto';
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

			// adapter-auto only supports some environments, see https://svelte.dev/docs/kit/adapter-auto for a list.
			// If your environment is not supported, or you settled on a specific environment, switch out the adapter.
			// See https://svelte.dev/docs/kit/adapters for more information about adapters.
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
