<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { auth } from '$lib/auth.svelte';
	import PdvLogin from '$lib/components/PdvLogin.svelte';
	import TrocarPinInicial from '$lib/components/TrocarPinInicial.svelte';
	import Navbar from '$lib/components/Navbar.svelte';
	import ToastContainer from '$lib/components/ToastContainer.svelte';
	import RadioPainel from '$lib/components/RadioPainel.svelte';
	import { radio } from '$lib/radio.svelte';
	import { page } from '$app/state';

	let { children } = $props();

	onMount(() => {
		auth.checkAuth();
	});

	// Sessão encerrada (sair, inatividade ou 401): a rádio para junto
	$effect(() => {
		if (!auth.user) {
			radio.parar();
			radio.aberto = false;
		}
	});
</script>

<svelte:head>
	<title>Projetos NOC</title>
</svelte:head>

<!-- Modo TV: tela própria, sem menu e sem exigir login (usa a chave da tela) -->
{#if page.url.pathname === '/tv' || page.url.pathname === '/plantao/tv'}
	{@render children()}
{:else if auth.loading}
	<div class="min-h-screen bg-paper flex items-center justify-center">
		<div class="flex items-center gap-3 text-sm text-ink-3">
			<div class="spinner size-5"></div>
			<p>Carregando…</p>
		</div>
	</div>
{:else if !auth.user}
	<PdvLogin />
{:else if auth.user.deve_trocar_pin}
	<TrocarPinInicial />
{:else}
	<div class="min-h-screen bg-paper text-ink">
		<Navbar />
		<!-- A barra lateral é fixa (w-60); o conteúdo ocupa todo o resto da tela -->
		<main class="flex-1 w-full min-w-0 min-h-screen lg:pl-60">
			<div class="w-full min-w-0 px-4 sm:px-6 lg:px-8 py-6 lg:py-8">
				{@render children()}
			</div>
		</main>
		<RadioPainel />
	</div>
{/if}

<ToastContainer />
