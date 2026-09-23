<script lang="ts">
	import { auth } from '$lib/auth.svelte';
	import { 
		LayoutDashboard, 
		Calendar, 
		Headphones, 
		CheckSquare, 
		Package, 
		Users, 
		LogOut,
		Menu,
		X
	} from 'lucide-svelte';

	let mobileMenuOpen = $state(false);

	const links = [
		{ href: '/', label: 'Painel', icon: LayoutDashboard },
		{ href: '/calendario', label: 'Calendário', icon: Calendar },
		{ href: '/atendimentos', label: 'Atendimentos', icon: Headphones },
		{ href: '/tarefas', label: 'Tarefas', icon: CheckSquare },
		{ href: '/estoque', label: 'Estoque', icon: Package }
	];

	function getIniciais(nome: string): string {
		const partes = nome.trim().split(/\s+/);
		if (partes.length === 1) return partes[0].slice(0, 2).toUpperCase();
		return (partes[0][0] + partes[partes.length - 1][0]).toUpperCase();
	}
</script>

<header class="bg-white border-b border-slate-200 sticky top-0 z-40">
	<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
		<div class="flex items-center justify-between h-16">
			<!-- Logo e Links Desktop -->
			<div class="flex items-center gap-6">
				<a href="/" class="flex items-center gap-2 font-black text-xl text-blue-600 tracking-tight">
					<span class="w-8 h-8 rounded-lg bg-blue-600 text-white flex items-center justify-center font-black text-base shadow-sm">T</span>
					<span>TIIV</span>
				</a>

				<nav class="hidden md:flex items-center gap-1">
					{#each links as item}
						{@const Icon = item.icon}
						<a
							href={item.href}
							class="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium text-slate-600 hover:text-blue-600 hover:bg-slate-50 transition"
						>
							<Icon class="w-4 h-4" />
							<span>{item.label}</span>
						</a>
					{/each}

					{#if auth.user?.papel === 'admin'}
						<a
							href="/usuarios"
							class="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium text-purple-600 hover:bg-purple-50 transition"
						>
							<Users class="w-4 h-4" />
							<span>Usuários</span>
						</a>
					{/if}
				</nav>
			</div>

			<!-- Usuário atual e Botão Sair Sempre Visível -->
			{#if auth.user}
				<div class="flex items-center gap-3">
					<div class="flex items-center gap-2.5">
						<div
							class="w-8 h-8 rounded-full flex items-center justify-center text-white text-xs font-bold shadow-xs"
							style="background-color: {auth.user.cor || '#2563EB'};"
						>
							{getIniciais(auth.user.nome)}
						</div>
						<div class="hidden sm:block text-left">
							<div class="text-sm font-semibold text-slate-800 leading-tight">{auth.user.nome}</div>
							<div class="text-[11px] text-slate-400 capitalize">{auth.user.papel}</div>
						</div>
					</div>

					<!-- Botão Sair Sempre Visível -->
					<button
						onclick={() => auth.logout()}
						class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-red-200 text-red-600 hover:bg-red-50 text-xs sm:text-sm font-medium transition cursor-pointer"
						title="Sair do terminal"
					>
						<LogOut class="w-4 h-4" />
						<span class="hidden sm:inline">Sair</span>
					</button>

					<!-- Botão Menu Mobile -->
					<button
						onclick={() => (mobileMenuOpen = !mobileMenuOpen)}
						class="md:hidden p-2 rounded-lg text-slate-600 hover:bg-slate-100"
						aria-label="Abrir menu"
					>
						{#if mobileMenuOpen}
							<X class="w-6 h-6" />
						{:else}
							<Menu class="w-6 h-6" />
						{/if}
					</button>
				</div>
			{/if}
		</div>
	</div>

	<!-- Menu Mobile -->
	{#if mobileMenuOpen}
		<div class="md:hidden border-t border-slate-200 bg-white px-4 pt-2 pb-4 space-y-1">
			{#each links as item}
				{@const Icon = item.icon}
				<a
					href={item.href}
					onclick={() => (mobileMenuOpen = false)}
					class="flex items-center gap-3 px-3 py-2.5 rounded-lg text-base font-medium text-slate-700 hover:bg-slate-100"
				>
					<Icon class="w-5 h-5 text-slate-500" />
					<span>{item.label}</span>
				</a>
			{/each}

			{#if auth.user?.papel === 'admin'}
				<a
					href="/usuarios"
					onclick={() => (mobileMenuOpen = false)}
					class="flex items-center gap-3 px-3 py-2.5 rounded-lg text-base font-medium text-purple-700 hover:bg-purple-50"
				>
					<Users class="w-5 h-5 text-purple-600" />
					<span>Gestão de Usuários</span>
				</a>
			{/if}
		</div>
	{/if}
</header>
