import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': {
				// API_TARGET permite apontar para outro backend (ex.: um `go run` local)
				target: process.env.API_TARGET ?? 'http://localhost:8080',
				changeOrigin: true
			}
		}
	}
});
