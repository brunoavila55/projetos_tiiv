<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { auth } from '$lib/auth.svelte';
	import PdvLogin from '$lib/components/PdvLogin.svelte';
	import Navbar from '$lib/components/Navbar.svelte';
	import ToastContainer from '$lib/components/ToastContainer.svelte';

	let { children } = $props();

	onMount(() => {
		auth.checkAuth();
	});
</script>

<svelte:head>
	<title>TIIV — App Interno</title>
</svelte:head>

{#if auth.loading}
	<div class="min-h-screen bg-slate-50 flex items-center justify-center">
		<div class="flex flex-col items-center gap-3">
			<div class="w-10 h-10 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
			<p class="text-sm font-medium text-slate-500">Inicializando sistema...</p>
		</div>
	</div>
{:else if !auth.user}
	<PdvLogin />
{:else}
	<div class="min-h-screen flex flex-col bg-slate-50 text-slate-900">
		<Navbar />
		<main class="flex-1 w-full max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
			{@render children()}
		</main>
	</div>
{/if}

<ToastContainer />
