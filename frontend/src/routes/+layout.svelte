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
	import { moduloDaRota } from '$lib/modulos';
	import { Blocks } from 'lucide-svelte';

	let { children } = $props();

	onMount(() => {
		auth.checkAuth();
	});

	// Sessão encerrada (sair, inatividade ou 401) ou rádio desligada no setor:
	// a rádio para junto
	$effect(() => {
		if (!auth.user || !auth.temModulo('radio')) {
			radio.parar();
			radio.aberto = false;
		}
	});

	// Tela de um módulo desligado no setor de trabalho
	const moduloBloqueado = $derived.by(() => {
		const m = moduloDaRota(page.url.pathname);
		return m && auth.user && !auth.temModulo(m.id) ? m : undefined;
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
				{#if moduloBloqueado}
					<div class="max-w-md mx-auto mt-16 text-center">
						<Blocks class="size-10 mx-auto text-ink-3" />
						<h1 class="mt-4 text-xl font-bold text-ink">{moduloBloqueado.nome} não está disponível</h1>
						<p class="mt-2 text-ink-2">
							Este módulo está desativado para o setor {auth.user.setor.nome}. Fale com o superadmin se precisar dele.
						</p>
						<a href="/" class="btn btn-secondary mt-6">Voltar ao painel</a>
					</div>
				{:else}
					{@render children()}
				{/if}
			</div>
		</main>
		{#if auth.temModulo('radio')}
			<RadioPainel />
		{/if}
	</div>
{/if}

<ToastContainer />
