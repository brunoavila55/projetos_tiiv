<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, ApiError } from '$lib/api';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { Delete, AlertCircle, Users } from 'lucide-svelte';
	import Avatar from './Avatar.svelte';

	let usuarios = $state<UsuarioPublico[]>([]);
	let loading = $state<boolean>(true);
	let errorMsg = $state<string | null>(null);

	// Estado do modal de PIN
	let selectedUser = $state<UsuarioPublico | null>(null);
	let pin = $state<string>('');
	let pinError = $state<string | null>(null);
	let submitting = $state<boolean>(false);

	async function carregarUsuarios() {
		loading = true;
		errorMsg = null;
		try {
			usuarios = await apiFetch<UsuarioPublico[]>('/api/auth/usuarios');
		} catch (err: any) {
			errorMsg = err.message || 'Erro ao carregar usuários';
		} finally {
			loading = false;
		}
	}

	function abrirTeclado(u: UsuarioPublico) {
		selectedUser = u;
		pin = '';
		pinError = null;
	}

	function fecharTeclado() {
		selectedUser = null;
		pin = '';
		pinError = null;
	}

	function adicionarDigito(d: string) {
		if (pin.length < 6) {
			pin += d;
			pinError = null;
			// Se atingir 4 a 6 dígitos, o usuário pode clicar em Entrar ou digitar Enter
		}
	}

	function apagarDigito() {
		if (pin.length > 0) {
			pin = pin.slice(0, -1);
			pinError = null;
		}
	}

	async function submeterPin() {
		if (!selectedUser || pin.length < 4 || submitting) return;

		submitting = true;
		pinError = null;

		try {
			await auth.login(selectedUser.id, pin);
			selectedUser = null;
			pin = '';
		} catch (err: any) {
			pinError = err.message || 'Erro ao autenticar';
			pin = ''; // limpa pin em caso de erro
		} finally {
			submitting = false;
		}
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (!selectedUser) return;

		if (e.key >= '0' && e.key <= '9') {
			e.preventDefault();
			adicionarDigito(e.key);
		} else if (e.key === 'Backspace') {
			e.preventDefault();
			apagarDigito();
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (pin.length >= 4) {
				submeterPin();
			}
		} else if (e.key === 'Escape') {
			e.preventDefault();
			fecharTeclado();
		}
	}

	// Relógio do terminal
	let agora = $state(new Date());

	onMount(() => {
		const relogio = setInterval(() => (agora = new Date()), 15_000);
		carregarUsuarios();
		window.addEventListener('keydown', handleKeyDown);
		return () => {
			clearInterval(relogio);
			window.removeEventListener('keydown', handleKeyDown);
		};
	});

	function getIniciais(nome: string): string {
		const partes = nome.trim().split(/\s+/);
		if (partes.length === 1) return partes[0].slice(0, 2).toUpperCase();
		return (partes[0][0] + partes[partes.length - 1][0]).toUpperCase();
	}
</script>

{#snippet tecla(conteudo: string, acao: () => void)}
	<button
		type="button"
		onclick={acao}
		disabled={submitting}
		class="h-[3.75rem] rounded-xl bg-surface border border-line-strong border-b-[3px] text-[26px] font-semibold text-ink tabular flex items-center justify-center cursor-pointer select-none transition-[transform,background-color] hover:bg-sunken active:translate-y-[2px] active:border-b active:bg-muted disabled:opacity-50"
	>
		{conteudo}
	</button>
{/snippet}

{#snippet teclado(u: UsuarioPublico)}
	<div class="flex flex-col items-center text-center">
		<Avatar id={u.id} nome={u.nome} cor={u.cor} fotoVersao={u.foto_versao} class="size-20 text-2xl" />
		<h2 class="mt-3 text-xl font-bold text-ink leading-tight">{u.nome}</h2>

		<div
			class="mt-5 flex items-center gap-3 h-5 {pinError ? 'animate-shake' : ''}"
			aria-live="polite"
			aria-label="{pin.length} dígitos digitados"
		>
			{#each Array(6) as _, i}
				<span
					class="size-3.5 rounded-full transition-colors duration-75 {i < pin.length
						? pinError ? 'bg-danger' : 'bg-ink'
						: i < 4
							? 'border-2 border-line-strong'
							: 'border-2 border-dashed border-line-strong'}"
				></span>
			{/each}
		</div>
		<p class="h-5 mt-2.5 text-sm font-medium {pinError ? 'text-danger' : 'text-ink-3'}" role={pinError ? 'alert' : undefined}>
			{pinError ?? 'Digite seu PIN'}
		</p>
	</div>

	<div class="mt-5 grid grid-cols-3 gap-2.5">
		{#each ['1', '2', '3', '4', '5', '6', '7', '8', '9'] as digit}
			{@render tecla(digit, () => adicionarDigito(digit))}
		{/each}
		<div></div>
		{@render tecla('0', () => adicionarDigito('0'))}
		<button
			type="button"
			onclick={apagarDigito}
			disabled={submitting || pin.length === 0}
			class="h-[3.75rem] rounded-xl text-ink-2 hover:bg-muted flex items-center justify-center cursor-pointer transition-colors disabled:opacity-30 disabled:cursor-default"
			aria-label="Apagar dígito"
		>
			<Delete class="size-7" />
		</button>
	</div>

	<button
		type="button"
		onclick={submeterPin}
		disabled={pin.length < 4 || submitting}
		class="btn btn-primary w-full h-13 mt-4 text-base"
	>
		{#if submitting}
			<span class="size-4 rounded-full border-2 border-on-accent/40 border-t-on-accent animate-spin"></span>
			<span>Verificando…</span>
		{:else}
			Entrar
		{/if}
	</button>
	<button type="button" onclick={fecharTeclado} class="btn btn-ghost w-full mt-1.5">Não sou eu</button>
{/snippet}

<div class="min-h-dvh bg-paper text-ink lg:grid lg:grid-cols-[minmax(0,1fr)_26rem] select-none">
	<!-- Mural de operadores -->
	<main class="flex flex-col min-w-0 px-5 sm:px-10 lg:px-14 pt-6 sm:pt-10 pb-10">
		<header class="flex items-center justify-between gap-4">
			<div class="flex items-center gap-2.5">
				<svg viewBox="0 0 28 28" class="size-8" aria-hidden="true">
					<rect width="28" height="28" rx="7" fill="var(--accent)" />
					<path d="M8 9h12M14 9v11" stroke="var(--on-accent)" stroke-width="2.6" stroke-linecap="round" />
				</svg>
				<span class="text-lg font-bold tracking-[-0.01em]">TIIV</span>
			</div>
			<time class="lg:hidden text-2xl font-bold tracking-[-0.02em] tabular">
				{agora.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
			</time>
		</header>

		<div class="w-full max-w-4xl mt-10 sm:mt-16 lg:mt-[10vh]">
			<h1 class="text-[2rem] sm:text-[2.75rem] font-bold leading-[1.05] tracking-[-0.025em]">Quem está no terminal?</h1>

			<div class="mt-8 sm:mt-10">
				{#if loading}
					<div class="flex items-center gap-3 text-sm text-ink-3 py-10">
						<div class="spinner size-5"></div>
						<span>Carregando operadores…</span>
					</div>
				{:else if errorMsg}
					<div class="alert bg-danger-soft text-danger max-w-lg justify-between">
						<div class="flex items-center gap-2">
							<AlertCircle class="size-5 shrink-0" />
							<span>{errorMsg}</span>
						</div>
						<button onclick={carregarUsuarios} class="btn btn-sm btn-danger underline underline-offset-2">Tentar de novo</button>
					</div>
				{:else if usuarios.length === 0}
					<div class="max-w-md">
						<Users class="size-9 text-ink-3" strokeWidth={1.5} />
						<h2 class="mt-3 text-lg font-bold">Nenhum operador ativo</h2>
						<p class="mt-1 text-ink-3">Peça a um administrador para cadastrar ou reativar operadores.</p>
					</div>
				{:else}
					<ul class="grid grid-cols-2 sm:grid-cols-3 xl:grid-cols-4 gap-x-4 gap-y-6 sm:gap-x-6 sm:gap-y-8">
						{#each usuarios as u (u.id)}
							{@const selecionado = selectedUser?.id === u.id}
							<li>
								<button
									onclick={() => abrirTeclado(u)}
									aria-pressed={selecionado}
									class="group w-full text-left cursor-pointer transition-opacity {selectedUser && !selecionado ? 'opacity-45 hover:opacity-80' : ''}"
								>
									<div
										class="relative aspect-square overflow-hidden rounded-2xl transition-[box-shadow,transform] group-active:scale-[0.97] {selecionado
											? 'ring-[3px] ring-accent ring-offset-[3px] ring-offset-paper'
											: 'group-hover:shadow-float'}"
										style="background-color: {u.cor || '#1f5c5a'};"
									>
										{#if u.foto_versao}
											<img
												src="/api/auth/usuarios/{u.id}/foto?v={u.foto_versao}"
												alt=""
												class="absolute inset-0 size-full object-cover"
												draggable="false"
											/>
										{:else}
											<span class="absolute left-4 bottom-3 text-white/95 text-4xl sm:text-5xl font-bold tracking-[-0.03em] leading-none [text-shadow:0_1px_2px_rgb(0_0_0/0.2)]">
												{getIniciais(u.nome)}
											</span>
										{/if}
									</div>
									<span class="block mt-2.5 px-0.5 text-[15px] sm:text-base font-semibold leading-snug text-ink line-clamp-2">{u.nome}</span>
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	</main>

	<!-- Painel do terminal (desktop) -->
	<aside class="hidden lg:flex flex-col h-dvh sticky top-0 bg-surface border-l border-line px-9 py-10">
		<div>
			<time class="block text-[3.5rem] font-bold leading-none tracking-[-0.035em] tabular">
				{agora.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
			</time>
			<p class="mt-2 text-ink-3 first-letter:uppercase">
				{agora.toLocaleDateString('pt-BR', { weekday: 'long', day: 'numeric', month: 'long' })}
			</p>
		</div>

		<div class="flex-1 flex flex-col justify-center py-8">
			{#if selectedUser}
				<div style="animation: modal-in 160ms ease-out;">
					{@render teclado(selectedUser)}
				</div>
			{:else}
				<div class="text-center">
					<div class="mx-auto size-20 rounded-full border-2 border-dashed border-line-strong"></div>
					<p class="mt-4 text-lg font-semibold text-ink">Toque no seu nome</p>
					<p class="mt-1 text-sm text-ink-3">Depois, digite o PIN no teclado que aparece aqui.</p>
				</div>
			{/if}
		</div>

		<p class="text-[13px] text-ink-3">A sessão termina sozinha após 30 minutos sem uso.</p>
	</aside>

	<!-- Teclado em folha inferior (celular e tablet) -->
	{#if selectedUser}
		<div
			class="lg:hidden modal-backdrop"
			onclick={(e) => { if (e.target === e.currentTarget) fecharTeclado(); }}
			aria-hidden="true"
		>
			<div class="modal max-w-sm px-5 pt-6 pb-5" role="dialog" aria-modal="true" aria-label="Digite o PIN de {selectedUser.nome}">
				{@render teclado(selectedUser)}
			</div>
		</div>
	{/if}
</div>
