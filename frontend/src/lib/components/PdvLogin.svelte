<script lang="ts">
	import { onMount } from 'svelte';
	import { fly, fade } from 'svelte/transition';
	import { apiFetch, ApiError } from '$lib/api';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { Delete, AlertCircle, Users, ChevronRight, ArrowLeft, MessageSquareWarning, CircleCheck, Lock, X } from 'lucide-svelte';
	import Avatar from './Avatar.svelte';
	import TiraDuvidas from './TiraDuvidas.svelte';

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
		if (pin.length < 4 && !submitting) {
			pin += d;
			pinError = null;
			// PIN tem sempre 4 dígitos: entra assim que o último é digitado
			if (pin.length === 4) submeterPin();
		}
	}

	function apagarDigito() {
		if (pin.length > 0) {
			pin = pin.slice(0, -1);
			pinError = null;
		}
	}

	async function submeterPin() {
		if (!selectedUser || pin.length !== 4 || submitting) return;

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

	// Painel lateral discreto com a lista de operadores e o PIN
	let acessoAberto = $state(false);

	function abrirAcesso() {
		acessoAberto = true;
		carregarUsuarios();
	}

	function fecharAcesso() {
		fecharTeclado();
		acessoAberto = false;
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (!selectedUser) {
			if (e.key === 'Escape' && acessoAberto) {
				e.preventDefault();
				fecharAcesso();
			}
			return;
		}

		if (e.key >= '0' && e.key <= '9') {
			e.preventDefault();
			adicionarDigito(e.key);
		} else if (e.key === 'Backspace') {
			e.preventDefault();
			apagarDigito();
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (pin.length === 4) {
				submeterPin();
			}
		} else if (e.key === 'Escape') {
			e.preventDefault();
			fecharTeclado();
		}
	}

	// Reportar um problema (abre ticket sem login)
	let ticketEnviado = $state<number | null>(null);
	let enviandoTicket = $state(false);
	let ticketErro = $state<string | null>(null);
	let tkNome = $state('');
	let tkTitulo = $state('');
	let tkDescricao = $state('');
	let tkPrioridade = $state<'baixa' | 'media' | 'alta'>('media');

	// Setores que recebem pedidos, cada um com os módulos que atende por aqui
	// (tickets e/ou tira-dúvidas); com um só, a escolha nem aparece
	let setoresPedido = $state<{ id: string; nome: string; tickets: boolean; tira_duvidas: boolean }[]>([]);
	let setoresCarregados = $state(false);
	let tkSetor = $state('');
	const setoresTicket = $derived(setoresPedido.filter((s) => s.tickets));
	const setoresDuvida = $derived(setoresPedido.filter((s) => s.tira_duvidas));

	async function carregarSetores() {
		try {
			setoresPedido = await apiFetch<typeof setoresPedido>('/api/setores/publico', { silent: true });
			setoresCarregados = true;
			if (setoresTicket.length === 1) tkSetor = setoresTicket[0].id;
		} catch {
			// Sem a lista, o servidor ainda aceita o pedido quando há um setor só
		}
	}

	// O setor pode ter vindo do tira-dúvidas e não receber tickets
	const setorDoTicket = $derived(
		setoresTicket.some((s) => s.id === tkSetor) ? tkSetor : setoresTicket.length === 1 ? setoresTicket[0].id : ''
	);

	// Mantém o nome para quem abre vários tickets seguidos
	function novoTicket() {
		ticketEnviado = null;
		ticketErro = null;
		tkTitulo = '';
		tkDescricao = '';
		tkPrioridade = 'media';
	}

	async function enviarTicket(e: SubmitEvent) {
		e.preventDefault();
		if (enviandoTicket) return;
		enviandoTicket = true;
		ticketErro = null;
		try {
			const res = await apiFetch<{ numero: number }>('/api/tickets/publico', {
				method: 'POST',
				silent: true,
				body: JSON.stringify({
					solicitante_nome: tkNome,
					titulo: tkTitulo,
					descricao: tkDescricao,
					prioridade: tkPrioridade,
					setor_id: setorDoTicket || undefined
				})
			});
			ticketEnviado = res.numero;
		} catch (err: any) {
			ticketErro = err.message || 'Não foi possível enviar o ticket';
		} finally {
			enviandoTicket = false;
		}
	}

	// "Não resolveu? Abrir ticket" no tira-dúvidas: leva as perguntas para a descrição
	function ticketDoTiraDuvidas(descricao: string) {
		if (ticketEnviado !== null) novoTicket();
		tkDescricao = tkDescricao.trim() ? `${tkDescricao.trim()}\n\n${descricao}` : descricao;
		document.getElementById(tkNome.trim() ? 'tk-titulo' : 'tk-nome')?.focus();
	}

	// Relógio do terminal
	let agora = $state(new Date());

	onMount(() => {
		carregarSetores();
		const relogio = setInterval(() => (agora = new Date()), 15_000);
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

<div class="relative min-h-dvh text-ink">
	<!-- Foto de fundo -->
	<div class="fixed inset-0 bg-[#06122b]" aria-hidden="true">
		<img src="/login-bg.jpg" alt="" class="size-full object-cover" draggable="false" />
		<div class="absolute inset-0 bg-gradient-to-b from-black/40 via-black/10 to-black/45"></div>
	</div>

	<div class="relative min-h-dvh flex flex-col">
		<header class="flex items-center justify-between gap-4 px-5 sm:px-10 pt-5 sm:pt-7 text-white">
			<div class="flex items-center gap-2.5">
				<img src="/logo.svg" alt="" class="size-10 [filter:drop-shadow(0_1px_2px_rgb(0_0_0/0.35))]" />
				<span class="text-lg font-bold tracking-[-0.01em] [text-shadow:0_1px_2px_rgb(0_0_0/0.3)]">Projetos NOC</span>
			</div>
			<div class="flex items-center gap-4">
				<time class="hidden sm:block text-sm font-medium text-white/85 tabular [text-shadow:0_1px_2px_rgb(0_0_0/0.3)]">
					{agora.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
				</time>
				<button
					type="button"
					onclick={abrirAcesso}
					class="inline-flex items-center gap-2 h-9 px-3.5 rounded-full bg-white/15 hover:bg-white/25 backdrop-blur-sm ring-1 ring-white/30 text-sm font-medium text-white cursor-pointer transition-colors"
				>
					<Lock class="size-3.5" />
					Acesso da equipe
				</button>
			</div>
		</header>

		<!-- Reportar um problema -->
		<main class="flex-1 grid place-items-center px-4 py-8 sm:py-10">
			<section class="w-full max-w-xl rounded-2xl bg-surface shadow-float p-6 sm:p-9">
				{#if setoresCarregados && setoresTicket.length === 0}
					<div class="py-4 text-center">
						<MessageSquareWarning class="mx-auto size-12 text-ink-3" strokeWidth={1.75} />
						<h1 class="mt-4 text-[1.75rem] font-bold leading-tight tracking-[-0.02em]">Abertura de tickets indisponível</h1>
						<p class="mt-2 text-ink-2">Nenhum setor recebe tickets por esta tela no momento. Procure a equipe diretamente.</p>
					</div>
				{:else if ticketEnviado !== null}
					<div class="py-4 text-center" role="status" in:fade={{ duration: 150 }}>
						<CircleCheck class="mx-auto size-12 text-ok" strokeWidth={1.75} />
						<h1 class="mt-4 text-[1.75rem] font-bold leading-tight tracking-[-0.02em]">Ticket #{ticketEnviado} registrado</h1>
						<p class="mt-2 text-ink-2">O setor já recebeu seu pedido. Guarde o número para acompanhar com a equipe.</p>
						<button type="button" onclick={novoTicket} class="btn btn-primary mt-7 h-12 px-6">Reportar outro problema</button>
					</div>
				{:else}
					<div class="flex items-start gap-4">
						<span class="hidden sm:grid shrink-0 size-12 place-items-center rounded-xl bg-accent-soft text-accent">
							<MessageSquareWarning class="size-6" />
						</span>
						<div>
							<h1 class="text-[1.75rem] font-bold leading-tight tracking-[-0.02em]">Reportar um problema</h1>
							<p class="mt-1 text-ink-3">Não precisa de cadastro. O pedido vai direto para a fila do setor.</p>
						</div>
					</div>

					<form class="mt-7 space-y-4" onsubmit={enviarTicket}>
						{#if setoresTicket.length > 1}
							<div>
								<label class="label" for="tk-setor">Para qual setor?</label>
								<select id="tk-setor" bind:value={tkSetor} required class="field h-11">
									<option value="" disabled>Escolha o setor</option>
									{#each setoresTicket as st (st.id)}
										<option value={st.id}>{st.nome}</option>
									{/each}
								</select>
							</div>
						{/if}
						<div>
							<label class="label" for="tk-nome">Seu nome</label>
							<input id="tk-nome" bind:value={tkNome} required maxlength="120" autocomplete="name" class="field h-11" />
						</div>
						<div>
							<label class="label" for="tk-titulo">Assunto</label>
							<input id="tk-titulo" bind:value={tkTitulo} required maxlength="200" placeholder="Ex.: Impressora não imprime" class="field h-11" />
						</div>
						<div>
							<label class="label" for="tk-desc">O que está acontecendo?</label>
							<textarea id="tk-desc" bind:value={tkDescricao} required maxlength="4000" rows="4" class="field py-2.5 resize-y"></textarea>
						</div>
						<div>
							<span class="label">Urgência</span>
							<div class="segmented w-full" role="group" aria-label="Urgência">
								{#each [['baixa', 'Pode esperar'], ['media', 'Normal'], ['alta', 'Urgente']] as [valor, rotulo]}
									<button
										type="button"
										class="flex-1 px-2 whitespace-nowrap"
										aria-pressed={tkPrioridade === valor}
										onclick={() => (tkPrioridade = valor as 'baixa' | 'media' | 'alta')}>{rotulo}</button
									>
								{/each}
							</div>
						</div>

						{#if ticketErro}
							<p class="text-sm font-medium text-danger" role="alert">{ticketErro}</p>
						{/if}

						<button type="submit" disabled={enviandoTicket} class="btn btn-primary w-full h-12 text-base">
							{enviandoTicket ? 'Enviando…' : 'Enviar ticket'}
						</button>
					</form>
				{/if}
			</section>
		</main>

	</div>

	{#if !setoresCarregados || setoresDuvida.length > 0}
		<TiraDuvidas
			onAbrirTicket={ticketDoTiraDuvidas}
			podeAbrirTicket={!setoresCarregados || setoresTicket.length > 0}
			setores={setoresDuvida}
			bind:setorId={tkSetor}
		/>
	{/if}

	<!-- Acesso da equipe (painel lateral) -->
	{#if acessoAberto}
		<div class="fixed inset-0 z-40 bg-overlay" onclick={fecharAcesso} aria-hidden="true" transition:fade={{ duration: 150 }}></div>
		<div
			class="fixed inset-y-0 right-0 z-50 w-full sm:w-[27rem] flex flex-col bg-surface shadow-float select-none"
			role="dialog"
			aria-modal="true"
			aria-label="Acesso da equipe"
			transition:fly={{ x: 440, duration: 220 }}
		>
			<div class="flex items-center justify-between px-7 pt-6">
				<h2 class="text-lg font-bold">Acesso da equipe</h2>
				<button type="button" onclick={fecharAcesso} class="icon-btn -mr-2" aria-label="Fechar">
					<X class="size-5" />
				</button>
			</div>

			<div class="flex-1 overflow-y-auto px-7 pt-6 pb-6">
				{#if selectedUser}
					<div in:fade={{ duration: 120 }}>
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
								<p class="text-2xl font-bold leading-tight tracking-[-0.015em] truncate">{selectedUser.nome}</p>
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
							{#each Array(4) as _, i}
								<span
									class="size-3.5 rounded-full transition-colors duration-75 {i < pin.length
										? pinError ? 'bg-danger' : 'bg-ink'
										: 'border-2 border-line-strong'}"
								></span>
							{/each}
						</div>
						<p class="h-5 mt-2 text-sm {pinError ? 'text-danger font-medium' : 'text-ink-3'}" role={pinError ? 'alert' : undefined}>
							{pinError ?? '4 dígitos. O teclado físico também funciona.'}
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
							disabled={pin.length !== 4 || submitting}
							class="btn btn-primary w-full h-12 mt-5 text-base"
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
					<p class="text-ink-3">Escolha seu nome para entrar com o PIN.</p>
					<div class="mt-5">
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
							<p class="mt-3 text-lg font-bold">Nenhum operador ativo</p>
							<p class="mt-1 text-ink-3">Peça a um administrador para cadastrar ou reativar operadores.</p>
						{:else}
							<ul class="space-y-2">
								{#each usuarios as u (u.id)}
									<li>
										<button
											onclick={() => abrirTeclado(u)}
											class="group w-full h-14 flex items-center gap-3 px-3 rounded-lg bg-sunken border border-line text-left cursor-pointer transition-colors hover:border-accent hover:bg-surface active:scale-[0.99]"
										>
											<Avatar id={u.id} nome={u.nome} cor={u.cor} fotoVersao={u.foto_versao} class="size-9 text-xs" />
											<span class="flex-1 min-w-0 font-semibold text-ink truncate">{u.nome}</span>
											<ChevronRight class="size-5 text-ink-3 transition-transform group-hover:translate-x-0.5 group-hover:text-accent" />
										</button>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/if}
			</div>

			<p class="px-7 py-4 border-t border-line text-[13px] text-ink-3">A sessão termina sozinha após 30 minutos sem uso.</p>
		</div>
	{/if}
</div>
