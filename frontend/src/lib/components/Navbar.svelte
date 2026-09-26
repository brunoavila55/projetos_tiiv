<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ticketsStore } from '$lib/tickets.svelte';
	import { monitoresStore } from '$lib/monitores.svelte';
	import { auth, type Setor } from '$lib/auth.svelte';
	import { apiFetch } from '$lib/api';
	import { themeStore } from '$lib/theme.svelte';
	import { radio } from '$lib/radio.svelte';
	import type { Modulo } from '$lib/modulos';
	import Avatar from './Avatar.svelte';
	import {
		LayoutDashboard,
		Calendar,
		CheckSquare,
		Package,
		HardHat,
		Inbox,
		Users,
		BookOpen,
		LogOut,
		Menu,
		X,
		Sun,
		Moon,
		Monitor,
		Radio,
		Activity,
		Link,
		Tv,
		Building2,
		ShieldCheck
	} from 'lucide-svelte';

	let mobileMenuOpen = $state(false);

	// Superadmin escolhe qual setor está vendo
	let setores = $state<Setor[]>([]);
	let trocandoSetor = $state(false);
	$effect(() => {
		if (auth.ehSuperadmin) {
			apiFetch<Setor[]>('/api/setores', { silent: true })
				.then((lista) => (setores = lista))
				.catch(() => {});
		}
	});

	async function trocarSetor(id: string) {
		trocandoSetor = true;
		try {
			await auth.trocarSetor(id);
		} catch {
			trocandoSetor = false;
		}
	}

	const papelLabel = { superadmin: 'Superadmin', admin: 'Administrador', usuario: 'Operador' } as const;

	// Contadores de tickets abertos e monitores fora do ar, atualizados a cada minuto
	onMount(() => {
		const atualizar = () => {
			if (auth.temModulo('tickets')) ticketsStore.atualizar();
			if (auth.temModulo('monitor')) monitoresStore.atualizar();
		};
		atualizar();
		const timer = setInterval(atualizar, 60_000);
		return () => clearInterval(timer);
	});

	// Itens com módulo só aparecem se ele está ligado no setor
	type ItemMenu = { href: string; label: string; icon: typeof LayoutDashboard; modulo?: Modulo };

	const todosLinks: ItemMenu[] = [
		{ href: '/', label: 'Painel', icon: LayoutDashboard },
		{ href: '/tickets', label: 'Tickets', icon: Inbox, modulo: 'tickets' },
		{ href: '/tarefas', label: 'Tarefas', icon: CheckSquare, modulo: 'tarefas' },
		{ href: '/calendario', label: 'Calendário', icon: Calendar, modulo: 'calendario' },
		{ href: '/plantao', label: 'Plantão', icon: ShieldCheck, modulo: 'plantao' },
		{ href: '/estoque', label: 'Estoque', icon: Package, modulo: 'estoque' },
		{ href: '/tecnicos', label: 'Técnicos', icon: HardHat, modulo: 'tecnicos' },
		{ href: '/monitor', label: 'Monitor', icon: Activity, modulo: 'monitor' },
		{ href: '/links', label: 'Links úteis', icon: Link, modulo: 'links' }
	];

	const todosAdminLinks: ItemMenu[] = [
		{ href: '/usuarios', label: 'Operadores', icon: Users },
		{ href: '/procedimentos', label: 'Contexto LLM', icon: BookOpen, modulo: 'tira_duvidas' }
	];

	const links = $derived(todosLinks.filter((l) => !l.modulo || auth.temModulo(l.modulo)));
	const adminLinks = $derived(todosAdminLinks.filter((l) => !l.modulo || auth.temModulo(l.modulo)));

	const temaLabel = { claro: 'Claro', escuro: 'Escuro', sistema: 'Automático' } as const;

	function isActive(href: string): boolean {
		const path = page.url.pathname;
		return href === '/' ? path === '/' : path.startsWith(href);
	}

	function toggleTheme() {
		if (themeStore.tema === 'claro') {
			themeStore.setTema('escuro');
		} else if (themeStore.tema === 'escuro') {
			themeStore.setTema('sistema');
		} else {
			themeStore.setTema('claro');
		}
	}
</script>

{#snippet marca()}
	<a href="/" class="flex items-center gap-2.5 text-ink" onclick={() => (mobileMenuOpen = false)}>
		<img src="/logo.svg" alt="" class="size-8" />
		<span class="text-[17px] font-bold tracking-[-0.01em]">Projetos NOC</span>
	</a>
{/snippet}

{#snippet setorAtual()}
	{#if auth.user}
		<div class="px-3 mb-4">
			{#if auth.ehSuperadmin && setores.length > 1}
				<label class="block">
					<span class="flex items-center gap-1.5 text-[13px] font-semibold text-ink-3 mb-1">
						<Building2 class="size-3.5" /> Setor
					</span>
					<select
						class="field field-sm"
						value={auth.user.setor.id}
						disabled={trocandoSetor}
						onchange={(e) => trocarSetor(e.currentTarget.value)}
					>
						{#each setores as s (s.id)}
							<option value={s.id}>{s.nome}</option>
						{/each}
					</select>
				</label>
			{:else}
				<div class="flex items-center gap-1.5 text-[13px] text-ink-3">
					<Building2 class="size-3.5" />
					<span>Setor</span>
					<span class="font-semibold text-ink truncate">{auth.user.setor.nome}</span>
				</div>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet navegacao()}
	{@render setorAtual()}
	<nav class="flex flex-col gap-0.5" aria-label="Principal">
		{#each links as item}
			{@const Icon = item.icon}
			{@const ativo = isActive(item.href)}
			<a
				href={item.href}
				onclick={() => (mobileMenuOpen = false)}
				aria-current={ativo ? 'page' : undefined}
				class="flex items-center gap-3 h-10 px-3 rounded-lg text-[15px] font-semibold transition-colors {ativo
					? 'bg-surface text-ink shadow-[0_1px_2px_rgb(0_0_0/0.06)] ring-1 ring-line'
					: 'text-ink-2 hover:bg-muted hover:text-ink'}"
			>
				<Icon class="size-[18px] {ativo ? 'text-accent' : 'text-ink-3'}" strokeWidth={ativo ? 2.25 : 2} />
				<span>{item.label}</span>
				{#if item.href === '/tickets' && ticketsStore.abertos > 0}
					<span
						class="ml-auto min-w-5 h-5 px-1.5 rounded-full bg-accent text-on-accent text-xs font-bold grid place-items-center tabular"
						aria-label="{ticketsStore.abertos} tickets abertos">{ticketsStore.abertos}</span
					>
				{/if}
				{#if item.href === '/monitor' && monitoresStore.offline > 0}
					<span
						class="ml-auto min-w-5 h-5 px-1.5 rounded-full bg-danger text-surface text-xs font-bold grid place-items-center tabular"
						aria-label="{monitoresStore.offline} fora do ar">{monitoresStore.offline}</span
					>
				{/if}
			</a>
		{/each}

		{#if auth.ehAdmin}
			<div class="mt-4 mb-1 px-3 text-[13px] font-semibold text-ink-3">Administração</div>
			{#each adminLinks as item}
				{@const Icon = item.icon}
				{@const ativo = isActive(item.href)}
				<a
					href={item.href}
					onclick={() => (mobileMenuOpen = false)}
					aria-current={ativo ? 'page' : undefined}
					class="flex items-center gap-3 h-10 px-3 rounded-lg text-[15px] font-semibold transition-colors {ativo
						? 'bg-surface text-ink shadow-[0_1px_2px_rgb(0_0_0/0.06)] ring-1 ring-line'
						: 'text-ink-2 hover:bg-muted hover:text-ink'}"
				>
					<Icon class="size-[18px] {ativo ? 'text-accent' : 'text-ink-3'}" />
					<span>{item.label}</span>
				</a>
			{/each}
		{/if}
	</nav>
{/snippet}

{#snippet rodape()}
	{#if auth.user}
		<div class="space-y-1">
			{#if auth.temModulo('radio')}
				<button
					onclick={() => {
						radio.aberto = !radio.aberto;
						mobileMenuOpen = false;
					}}
					class="flex w-full items-center gap-3 h-9 px-3 rounded-lg text-sm font-medium transition-colors cursor-pointer {radio.aberto
						? 'bg-muted text-ink'
						: 'text-ink-2 hover:bg-muted hover:text-ink'}"
					aria-expanded={radio.aberto}
				>
					<Radio class="size-4 {radio.estado === 'tocando' ? 'text-accent' : 'text-ink-3'}" />
					<span class="truncate">
						{radio.estado === 'tocando' && radio.atual ? radio.atual.name.trim() : 'Rádio'}
					</span>
					{#if radio.estado === 'tocando'}
						<span class="ml-auto flex items-end gap-[2px] h-3" aria-label="Tocando">
							<span class="eq-bar"></span><span class="eq-bar [animation-delay:-0.3s]"></span><span
								class="eq-bar [animation-delay:-0.6s]"
							></span>
						</span>
					{/if}
				</button>
			{/if}
			{#if auth.temModulo('tv')}
				<a
					href="/tv"
					onclick={() => (mobileMenuOpen = false)}
					class="flex w-full items-center gap-3 h-9 px-3 rounded-lg text-sm font-medium text-ink-2 hover:bg-muted hover:text-ink transition-colors"
				>
					<Tv class="size-4 text-ink-3" />
					<span>Modo TV</span>
				</a>
			{/if}
			<button
				onclick={toggleTheme}
				class="flex w-full items-center gap-3 h-9 px-3 rounded-lg text-sm font-medium text-ink-2 hover:bg-muted hover:text-ink transition-colors cursor-pointer"
				aria-label="Alternar tema (atual: {temaLabel[themeStore.tema]})"
			>
				{#if themeStore.tema === 'claro'}
					<Sun class="size-4 text-ink-3" />
				{:else if themeStore.tema === 'escuro'}
					<Moon class="size-4 text-ink-3" />
				{:else}
					<Monitor class="size-4 text-ink-3" />
				{/if}
				<span>Tema: {temaLabel[themeStore.tema]}</span>
			</button>

			<div class="flex items-center gap-2.5 pt-3 mt-2 border-t border-line">
				<Avatar id={auth.user.id} nome={auth.user.nome} cor={auth.user.cor} fotoVersao={auth.user.foto_versao} class="size-9 text-xs" />
				<div class="min-w-0 flex-1">
					<div class="text-sm font-semibold text-ink leading-tight truncate">{auth.user.nome}</div>
					<div class="text-xs text-ink-3">{papelLabel[auth.user.papel]}</div>
				</div>
				<button
					onclick={() => auth.logout()}
					class="icon-btn icon-btn-danger"
					title="Sair do terminal"
					aria-label="Sair do terminal"
				>
					<LogOut class="size-[18px]" />
				</button>
			</div>
		</div>
	{/if}
{/snippet}

<!-- Barra lateral (desktop) -->
<aside class="hidden lg:flex fixed inset-y-0 left-0 z-30 w-60 flex-col border-r border-line bg-paper px-3 py-5">
	<div class="px-3 mb-7">{@render marca()}</div>
	<div class="flex-1 overflow-y-auto">{@render navegacao()}</div>
	{@render rodape()}
</aside>

<!-- Barra superior (celular / tablet) -->
<header class="lg:hidden sticky top-0 z-40 flex items-center justify-between h-14 px-4 border-b border-line bg-surface/95 backdrop-blur">
	{@render marca()}
	<button
		onclick={() => (mobileMenuOpen = !mobileMenuOpen)}
		class="icon-btn size-10"
		aria-label={mobileMenuOpen ? 'Fechar menu' : 'Abrir menu'}
		aria-expanded={mobileMenuOpen}
	>
		{#if mobileMenuOpen}
			<X class="size-5" />
		{:else}
			<Menu class="size-5" />
		{/if}
	</button>
</header>

{#if mobileMenuOpen}
	<div class="lg:hidden fixed inset-0 top-14 z-30 bg-overlay" onclick={() => (mobileMenuOpen = false)} aria-hidden="true"></div>
	<div class="lg:hidden fixed top-14 inset-x-0 z-40 border-b border-line bg-paper px-3 pt-3 pb-4 shadow-float">
		{@render navegacao()}
		<div class="mt-3">{@render rodape()}</div>
	</div>
{/if}

<style>
	.eq-bar {
		width: 3px;
		height: 100%;
		border-radius: 1px;
		background: var(--accent);
		transform-origin: bottom;
		animation: eq 0.9s ease-in-out infinite;
	}
	@keyframes eq {
		0%, 100% { transform: scaleY(0.3); }
		50% { transform: scaleY(1); }
	}
	@media (prefers-reduced-motion: reduce) {
		.eq-bar { animation: none; transform: scaleY(0.6); }
	}
</style>
