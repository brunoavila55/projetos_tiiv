<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, ApiError } from '$lib/api';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { Delete, AlertCircle, Users, ChevronRight, ArrowLeft } from 'lucide-svelte';
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
</script>

{#snippet tecla(conteudo: string, acao: () => void)}
	<button
		type="button"
		onclick={acao}
		disabled={submitting}
		class="h-14 rounded-lg bg-sunken border border-line text-2xl font-semibold text-ink tabular flex items-center justify-center cursor-pointer select-none transition-[transform,background-color] hover:bg-muted active:scale-[0.97] disabled:opacity-50"
	>
		{conteudo}
	</button>
{/snippet}

<div class="min-h-dvh bg-surface text-ink lg:grid lg:grid-cols-[minmax(0,1fr)_34rem] select-none">
	<!-- Foto -->
	<div class="relative h-40 sm:h-56 lg:h-dvh lg:sticky lg:top-0 overflow-hidden bg-[#6f8fa8]">
		<img
			src="/login-bg.jpg"
			alt="Baía calma entre montanhas ao entardecer"
			class="absolute inset-0 size-full object-cover"
			draggable="false"
		/>
		<div class="absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-black/35 to-transparent"></div>
		<p class="absolute left-5 bottom-4 lg:left-12 lg:bottom-10 text-[13px] text-white/90 [text-shadow:0_1px_2px_rgb(0_0_0/0.35)]">
			Foto de
			<a
				href="https://www.pexels.com/photo/photo-of-sea-and-mountain-906961/"
				target="_blank"
				rel="noopener noreferrer"
				class="underline underline-offset-2 hover:text-white">Stefanos Martimianakis</a
			>
			no Pexels
		</p>
	</div>

	<!-- Painel de acesso -->
	<main class="flex flex-col min-h-[calc(100dvh-10rem)] sm:min-h-[calc(100dvh-14rem)] lg:min-h-dvh px-6 sm:px-12 lg:px-16 pt-8 lg:pt-14 pb-8">
		<header class="flex items-center gap-3">
			<svg viewBox="0 0 28 28" class="size-11" aria-hidden="true">
				<rect width="28" height="28" rx="8" fill="var(--accent)" />
				<path d="M8 9h12M14 9v11" stroke="var(--on-accent)" stroke-width="2.6" stroke-linecap="round" />
			</svg>
			<span class="text-[22px] font-bold tracking-[-0.01em]">TIIV</span>
		</header>

		<div class="w-full max-w-sm mx-auto lg:mx-0 flex-1 mt-10 lg:mt-[9vh]">
			{#if selectedUser}
				<div style="animation: modal-in 160ms ease-out;">
					<button
						type="button"
						onclick={fecharTeclado}
						class="inline-flex items-center gap-1.5 -ml-1 px-1 py-1 rounded text-sm font-semibold text-accent hover:underline underline-offset-2 cursor-pointer"
					>
						<ArrowLeft class="size-4" />
						Trocar operador
					</button>

					<div class="mt-5 flex items-center gap-3.5">
						<Avatar id={selectedUser.id} nome={selectedUser.nome} cor={selectedUser.cor} fotoVersao={selectedUser.foto_versao} class="size-14 text-lg" />
						<div class="min-w-0">
							<p class="text-sm text-ink-3">Olá,</p>
							<h1 class="text-2xl font-bold leading-tight tracking-[-0.015em] truncate">{selectedUser.nome}</h1>
						</div>
					</div>

					<p class="mt-7 mb-2 text-sm font-medium text-ink-2">PIN</p>
					<div
						class="h-14 rounded-lg bg-sunken border flex items-center justify-center gap-3.5 {pinError
							? 'border-danger animate-shake'
							: 'border-line'}"
						aria-live="polite"
						aria-label="{pin.length} dígitos digitados"
					>
						{#each Array(6) as _, i}
							<span
								class="size-3 rounded-full transition-colors duration-75 {i < pin.length
									? pinError ? 'bg-danger' : 'bg-ink'
									: i < 4
										? 'border-2 border-line-strong'
										: 'border-2 border-dashed border-line-strong'}"
							></span>
						{/each}
					</div>
					<p class="h-5 mt-2 text-sm {pinError ? 'text-danger font-medium' : 'text-ink-3'}" role={pinError ? 'alert' : undefined}>
						{pinError ?? 'De 4 a 6 dígitos. O teclado físico também funciona.'}
					</p>

					<div class="mt-4 grid grid-cols-3 gap-2">
						{#each ['1', '2', '3', '4', '5', '6', '7', '8', '9'] as digit}
							{@render tecla(digit, () => adicionarDigito(digit))}
						{/each}
						<div></div>
						{@render tecla('0', () => adicionarDigito('0'))}
						<button
							type="button"
							onclick={apagarDigito}
							disabled={submitting || pin.length === 0}
							class="h-14 rounded-lg text-ink-2 hover:bg-muted flex items-center justify-center cursor-pointer transition-colors disabled:opacity-30 disabled:cursor-default"
							aria-label="Apagar dígito"
						>
							<Delete class="size-6" />
						</button>
					</div>

					<button
						type="button"
						onclick={submeterPin}
						disabled={pin.length < 4 || submitting}
						class="btn btn-primary w-full h-13 mt-5 text-base"
					>
						{#if submitting}
							<span class="size-4 rounded-full border-2 border-on-accent/40 border-t-on-accent animate-spin"></span>
							<span>Verificando…</span>
						{:else}
							Entrar
						{/if}
					</button>
				</div>
			{:else}
				<h1 class="text-[1.75rem] font-bold leading-tight tracking-[-0.02em]">Que bom te ver de novo</h1>
				<p class="mt-1.5 text-ink-3">Escolha seu nome para entrar com o PIN.</p>

				<div class="mt-8">
					{#if loading}
						<div class="flex items-center gap-3 text-sm text-ink-3 py-6">
							<div class="spinner size-5"></div>
							<span>Carregando operadores…</span>
						</div>
					{:else if errorMsg}
						<div class="alert bg-danger-soft text-danger justify-between">
							<div class="flex items-center gap-2">
								<AlertCircle class="size-5 shrink-0" />
								<span>{errorMsg}</span>
							</div>
							<button onclick={carregarUsuarios} class="btn btn-sm btn-danger underline underline-offset-2">Tentar de novo</button>
						</div>
					{:else if usuarios.length === 0}
						<Users class="size-9 text-ink-3" strokeWidth={1.5} />
						<h2 class="mt-3 text-lg font-bold">Nenhum operador ativo</h2>
						<p class="mt-1 text-ink-3">Peça a um administrador para cadastrar ou reativar operadores.</p>
					{:else}
						<p class="mb-2 text-sm font-medium text-ink-2">Operador</p>
						<ul class="space-y-2 max-h-[52vh] overflow-y-auto -mx-1 px-1 pb-1">
							{#each usuarios as u (u.id)}
								<li>
									<button
										onclick={() => abrirTeclado(u)}
										class="group w-full h-16 flex items-center gap-3.5 px-3.5 rounded-lg bg-sunken border border-line text-left cursor-pointer transition-colors hover:border-accent hover:bg-surface active:scale-[0.99]"
									>
										<Avatar id={u.id} nome={u.nome} cor={u.cor} fotoVersao={u.foto_versao} class="size-10 text-sm" />
										<span class="flex-1 min-w-0 text-base font-semibold text-ink truncate">{u.nome}</span>
										<ChevronRight class="size-5 text-ink-3 transition-transform group-hover:translate-x-0.5 group-hover:text-accent" />
									</button>
								</li>
							{/each}
						</ul>
					{/if}
				</div>
			{/if}
		</div>

		<footer class="mt-10 pt-5 border-t border-line flex items-end justify-between gap-4 text-[13px] text-ink-3">
			<div>
				<time class="block text-lg font-bold text-ink tabular leading-none">
					{agora.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
				</time>
				<span class="first-letter:uppercase inline-block mt-1">
					{agora.toLocaleDateString('pt-BR', { weekday: 'long', day: 'numeric', month: 'long' })}
				</span>
			</div>
			<p class="text-right">Sessão encerra após<br />30 min sem uso</p>
		</footer>
	</main>
</div>
