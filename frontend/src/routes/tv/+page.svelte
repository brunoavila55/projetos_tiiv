<script lang="ts">
	import { onMount } from 'svelte';
	import { themeStore } from '$lib/theme.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import { Maximize, Minimize, ArrowLeft, Inbox, ShieldCheck, Megaphone, TriangleAlert, WifiOff, Tv } from 'lucide-svelte';

	type Status = 'pendente' | 'online' | 'offline';

	interface MonitorTV {
		id: string;
		nome: string;
		tipo: string;
		status: Status;
		latencia_ms: number | null;
		ultimo_erro: string;
		status_desde: string;
		disponibilidade_24h: number | null;
	}

	interface PainelTV {
		tela: string;
		// Tickets, plantão e avisos são deste setor; monitores são de todos
		setor: string;
		agora_servidor: string;
		monitores: MonitorTV[];
		tickets_total: number;
		tickets: { numero: number; titulo: string; solicitante_nome: string; prioridade: 'baixa' | 'media' | 'alta'; criado_em: string }[];
		plantao_hoje: { id: string; usuario_id: string; usuario_nome: string; usuario_cor: string; tipo: 'plantao' | 'sobreaviso'; observacao: string }[];
		avisos: { id: string; titulo: string; mensagem: string; nivel: 'info' | 'atencao' | 'critico'; criador_nome: string }[];
	}

	const FUSO = 'America/Sao_Paulo';
	const CHAVE_STORAGE = 'tiiv_tv_chave';
	const INTERVALO_MS = 15_000;
	// Sem resposta do servidor por mais que isso, a tela avisa que os dados podem estar velhos
	const DADOS_VELHOS_MS = 60_000;
	// Recarrega a página de tempos em tempos para pegar atualizações do sistema
	const RECARREGAR_MS = 6 * 60 * 60 * 1000;

	let dados = $state<PainelTV | null>(null);
	let erro = $state<'nao_autorizada' | 'limite' | null>(null);
	let ultimoSucesso = $state(0);
	let falhando = $state(false);
	let agora = $state(Date.now());
	// Diferença entre o relógio do servidor e o da TV (TVs costumam ter o relógio errado)
	let desvio = 0;
	let chave = $state<string | null>(null);
	let temSessao = $state(false);

	let telaCheia = $state(false);
	let controlesVisiveis = $state(true);

	function lerChave(): string | null {
		// O link da tela traz a chave no fragmento (#chave=...), que não vai ao servidor;
		// guarda e limpa a barra de endereço
		const m = location.hash.match(/chave=([0-9a-f]{64})/i);
		if (m) {
			try {
				localStorage.setItem(CHAVE_STORAGE, m[1]);
			} catch {
				// sem armazenamento: vale só enquanto a página estiver aberta
			}
			history.replaceState(null, '', location.pathname);
			return m[1];
		}
		try {
			return localStorage.getItem(CHAVE_STORAGE);
		} catch {
			return null;
		}
	}

	async function carregar() {
		try {
			const res = await fetch('/api/tv/painel', {
				credentials: 'include',
				headers: { Accept: 'application/json', ...(chave ? { 'X-TV-Chave': chave } : {}) }
			});
			if (res.status === 401) {
				// Chave revogada: esquece para não ficar tentando
				if (chave) {
					try {
						localStorage.removeItem(CHAVE_STORAGE);
					} catch {
						// nada a limpar
					}
				}
				erro = 'nao_autorizada';
				return;
			}
			if (res.status === 429) {
				erro = 'limite';
				return;
			}
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			const d: PainelTV = await res.json();
			desvio = new Date(d.agora_servidor).getTime() - Date.now();
			dados = d;
			erro = null;
			falhando = false;
			ultimoSucesso = Date.now();
			temSessao = !chave;
		} catch {
			// Servidor fora ou rede caída: mantém os últimos dados e sinaliza
			falhando = true;
		}
	}

	onMount(() => {
		chave = lerChave();

		// TV sempre no tema escuro e com a escala acompanhando a largura da tela
		const html = document.documentElement;
		const tamanhoAnterior = html.style.fontSize;
		html.classList.add('dark');
		html.style.fontSize = 'clamp(13px, 0.85vw, 40px)';

		carregar();
		const timerDados = setInterval(() => {
			// Tela recusada: só volta com um novo link ou login
			if (erro === 'nao_autorizada') return;
			carregar();
		}, INTERVALO_MS);
		const timerRelogio = setInterval(() => (agora = Date.now()), 1000);
		const timerRecarga = setTimeout(function recarregar() {
			if (!falhando) location.reload();
			else setTimeout(recarregar, 60_000);
		}, RECARREGAR_MS);

		// Mantém a tela ligada (quando o navegador permite)
		let wakeLock: { release: () => Promise<void> } | null = null;
		const pedirTelaLigada = async () => {
			try {
				wakeLock = await (navigator as any).wakeLock?.request('screen');
			} catch {
				// sem suporte ou sem permissão
			}
		};
		const aoVoltarVisivel = () => {
			if (document.visibilityState === 'visible') {
				pedirTelaLigada();
				carregar();
			}
		};
		pedirTelaLigada();
		document.addEventListener('visibilitychange', aoVoltarVisivel);

		// Controles e cursor somem sozinhos
		let timerControles: ReturnType<typeof setTimeout>;
		const mostrarControles = () => {
			controlesVisiveis = true;
			clearTimeout(timerControles);
			timerControles = setTimeout(() => (controlesVisiveis = false), 3000);
		};
		mostrarControles();
		window.addEventListener('mousemove', mostrarControles);
		window.addEventListener('keydown', mostrarControles);
		const aoMudarTelaCheia = () => (telaCheia = !!document.fullscreenElement);
		document.addEventListener('fullscreenchange', aoMudarTelaCheia);

		return () => {
			clearInterval(timerDados);
			clearInterval(timerRelogio);
			clearTimeout(timerRecarga);
			clearTimeout(timerControles);
			wakeLock?.release().catch(() => {});
			document.removeEventListener('visibilitychange', aoVoltarVisivel);
			window.removeEventListener('mousemove', mostrarControles);
			window.removeEventListener('keydown', mostrarControles);
			document.removeEventListener('fullscreenchange', aoMudarTelaCheia);
			html.style.fontSize = tamanhoAnterior;
			themeStore.applyTheme();
		};
	});

	function alternarTelaCheia() {
		if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
		else document.documentElement.requestFullscreen().catch(() => {});
	}

	const agoraServidor = $derived(agora + desvio);
	const hora = $derived(new Date(agoraServidor).toLocaleTimeString('pt-BR', { timeZone: FUSO, hour: '2-digit', minute: '2-digit' }));
	const segundos = $derived(new Date(agoraServidor).toLocaleTimeString('pt-BR', { timeZone: FUSO, second: '2-digit' }).padStart(2, '0'));
	const data = $derived(new Date(agoraServidor).toLocaleDateString('pt-BR', { timeZone: FUSO, weekday: 'long', day: 'numeric', month: 'long' }));

	const fora = $derived(dados?.monitores.filter((m) => m.status === 'offline') ?? []);
	const demais = $derived(dados?.monitores.filter((m) => m.status !== 'offline') ?? []);
	const noAr = $derived(dados?.monitores.filter((m) => m.status === 'online').length ?? 0);
	const dadosVelhos = $derived(ultimoSucesso > 0 && agora - ultimoSucesso > DADOS_VELHOS_MS);

	function duracao(desdeIso: string): string {
		const min = Math.max(0, Math.floor((agoraServidor - new Date(desdeIso).getTime()) / 60000));
		if (min < 1) return 'menos de 1 min';
		if (min < 60) return `${min} min`;
		const h = Math.floor(min / 60);
		if (h < 24) return `${h} h${min % 60 ? ` ${min % 60} min` : ''}`;
		const d = Math.floor(h / 24);
		return `${d} ${d === 1 ? 'dia' : 'dias'}${h % 24 ? ` ${h % 24} h` : ''}`;
	}

	function formatarPct(p: number): string {
		return p.toLocaleString('pt-BR', { maximumFractionDigits: p >= 99.9 && p < 100 ? 2 : 1 }) + '%';
	}

	function horaCurta(ms: number): string {
		return new Date(ms + desvio).toLocaleTimeString('pt-BR', { timeZone: FUSO, hour: '2-digit', minute: '2-digit' });
	}

	const rotuloPrioridade = { alta: 'Alta', media: 'Média', baixa: 'Baixa' } as const;
</script>

<svelte:head>
	<title>{fora.length > 0 ? `(${fora.length}) ` : ''}Modo TV · Projetos NOC</title>
</svelte:head>

<div class="h-screen overflow-hidden bg-paper text-ink flex flex-col {controlesVisiveis ? '' : 'cursor-none'}">
	{#if erro === 'nao_autorizada' && !dados}
		<div class="flex-1 grid place-items-center p-8">
			<div class="max-w-[34rem] text-center space-y-4">
				<Tv class="size-14 mx-auto text-ink-3" strokeWidth={1.5} />
				<h1 class="text-[2rem] font-bold leading-tight">Tela não autorizada</h1>
				<p class="text-[1.125rem] text-ink-2">
					Para usar o modo TV sem login, um administrador cadastra esta tela em
					<strong class="text-ink">Monitor → Telas de TV</strong> e abre aqui o link gerado.
				</p>
				<a href="/" class="btn btn-secondary btn-lg">Entrar no sistema</a>
			</div>
		</div>
	{:else if !dados}
		<div class="flex-1 grid place-items-center">
			<div class="flex items-center gap-3 text-ink-3">
				<div class="spinner size-6"></div>
				<p class="text-[1.125rem]">{falhando ? 'Sem conexão com o servidor. Tentando de novo…' : 'Carregando…'}</p>
			</div>
		</div>
	{:else}
		<!-- Cabeçalho: marca, estado geral e relógio -->
		<header class="flex items-center gap-6 px-8 pt-6 pb-5">
			<div class="flex items-center gap-3 min-w-0 w-[26%]">
				<svg viewBox="0 0 28 28" class="size-10 shrink-0" aria-hidden="true">
					<rect width="28" height="28" rx="7" fill="var(--accent)" />
					<path d="M8 9h12M14 9v11" stroke="var(--on-accent)" stroke-width="2.6" stroke-linecap="round" />
				</svg>
				<div class="min-w-0">
					<div class="text-[1.5rem] font-bold leading-tight tracking-[-0.01em]">Projetos NOC</div>
					<div class="text-[0.95rem] text-ink-3 truncate">{dados.setor} · {dados.tela}</div>
				</div>
			</div>

			<div class="flex-1 flex justify-center">
				{#if dados.monitores.length === 0}
					<div class="estado bg-muted text-ink-2">Nenhum serviço monitorado</div>
				{:else if fora.length > 0}
					<div class="estado bg-danger text-paper pulso" role="status">
						<TriangleAlert class="size-[1.6rem]" strokeWidth={2.5} />
						{fora.length === 1 ? '1 serviço fora do ar' : `${fora.length} serviços fora do ar`}
					</div>
				{:else}
					<div class="estado bg-ok-soft text-ok" role="status">
						<ShieldCheck class="size-[1.6rem]" strokeWidth={2.5} />
						{noAr === dados.monitores.length ? `Todos os ${noAr} serviços no ar` : `${noAr} de ${dados.monitores.length} serviços confirmados no ar`}
					</div>
				{/if}
			</div>

			<div class="w-[26%] text-right">
				<div class="text-[3.5rem] font-bold leading-none tabular tracking-[-0.02em]">
					{hora}<span class="text-[1.75rem] text-ink-3 font-semibold">:{segundos}</span>
				</div>
				<div class="mt-1 text-[1.05rem] text-ink-2 first-letter:uppercase">{data}</div>
			</div>
		</header>

		<main class="flex-1 min-h-0 grid grid-cols-[minmax(0,2fr)_minmax(0,1fr)] gap-6 px-8 pb-4">
			<!-- Monitores -->
			<section class="min-h-0 flex flex-col gap-4 overflow-hidden" aria-label="Monitores">
				{#if fora.length > 0}
					<div class="grid grid-cols-[repeat(auto-fill,minmax(22rem,1fr))] gap-4">
						{#each fora as m (m.id)}
							<article class="rounded-2xl border-2 border-danger bg-danger-soft px-5 py-4">
								<div class="flex items-center gap-3">
									<span class="size-4 rounded-full bg-danger shrink-0 pulso"></span>
									<h2 class="text-[1.5rem] font-bold leading-tight truncate">{m.nome}</h2>
								</div>
								<p class="mt-2 text-[1.25rem] font-semibold text-danger">Fora do ar há {duracao(m.status_desde)}</p>
								{#if m.ultimo_erro}
									<p class="mt-1 text-[1rem] text-ink-2 truncate">{m.ultimo_erro}</p>
								{/if}
							</article>
						{/each}
					</div>
				{/if}

				{#if demais.length > 0}
					<div class="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] gap-3 content-start">
						{#each demais as m (m.id)}
							<article class="rounded-xl bg-surface border border-line px-4 py-3 flex items-center gap-3 min-w-0">
								<span class="size-3 rounded-full shrink-0 {m.status === 'online' ? 'bg-ok' : 'bg-ink-3'}"></span>
								<div class="min-w-0 flex-1">
									<div class="text-[1.1rem] font-semibold truncate">{m.nome}</div>
									<div class="text-[0.9rem] text-ink-3 tabular">
										{#if m.status === 'pendente'}
											Aguardando verificação
										{:else}
											{m.latencia_ms != null ? `${m.latencia_ms} ms` : 'No ar'}{m.disponibilidade_24h != null
												? ` · ${formatarPct(m.disponibilidade_24h)} em 24 h`
												: ''}
										{/if}
									</div>
								</div>
							</article>
						{/each}
					</div>
				{/if}
			</section>

			<!-- Coluna lateral: plantão, tickets e avisos -->
			<aside class="min-h-0 flex flex-col gap-4 overflow-hidden">
				<section class="rounded-2xl bg-surface border border-line px-5 py-4">
					<h2 class="titulo-bloco">De plantão hoje</h2>
					{#if dados.plantao_hoje.length === 0}
						<p class="mt-2 text-[1.05rem] text-ink-3">Ninguém escalado para hoje.</p>
					{:else}
						<ul class="mt-3 space-y-2.5">
							{#each dados.plantao_hoje as p (p.id)}
								<li class="flex items-center gap-3 min-w-0">
									<Avatar id={p.usuario_id} nome={p.usuario_nome} cor={p.usuario_cor} class="size-11 text-[0.95rem]" />
									<div class="min-w-0">
										<div class="text-[1.25rem] font-semibold leading-tight truncate">{p.usuario_nome}</div>
										<div class="text-[0.9rem] text-ink-3 truncate">
											{p.tipo === 'plantao' ? 'Plantão' : 'Sobreaviso'}{p.observacao ? ` · ${p.observacao}` : ''}
										</div>
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<section class="rounded-2xl bg-surface border border-line px-5 py-4 min-h-0 flex flex-col">
					<div class="flex items-center justify-between gap-3">
						<h2 class="titulo-bloco">Tickets aguardando</h2>
						<span
							class="min-w-[2.25rem] h-[2.25rem] px-2 rounded-full grid place-items-center text-[1.25rem] font-bold tabular {dados.tickets_total > 0
								? 'bg-accent text-on-accent'
								: 'bg-muted text-ink-3'}">{dados.tickets_total}</span
						>
					</div>
					{#if dados.tickets.length === 0}
						<p class="mt-2 text-[1.05rem] text-ink-3 flex items-center gap-2"><Inbox class="size-5" /> Fila vazia.</p>
					{:else}
						<ul class="mt-3 space-y-2.5 overflow-hidden">
							{#each dados.tickets as tk (tk.numero)}
								<li class="min-w-0 border-l-4 pl-3 {tk.prioridade === 'alta' ? 'border-danger' : tk.prioridade === 'media' ? 'border-warn' : 'border-line-strong'}">
									<div class="text-[1.1rem] font-semibold leading-snug truncate">
										<span class="text-ink-3 tabular">#{tk.numero}</span>
										{tk.titulo}
									</div>
									<div class="text-[0.9rem] text-ink-3 truncate">
										{tk.solicitante_nome} · há {duracao(tk.criado_em)} · {rotuloPrioridade[tk.prioridade]}
									</div>
								</li>
							{/each}
						</ul>
						{#if dados.tickets_total > dados.tickets.length}
							<p class="mt-2 text-[0.9rem] text-ink-3">e mais {dados.tickets_total - dados.tickets.length}</p>
						{/if}
					{/if}
				</section>

				{#if dados.avisos.length > 0}
					<section class="rounded-2xl bg-surface border border-line px-5 py-4 min-h-0 flex flex-col overflow-hidden">
						<h2 class="titulo-bloco flex items-center gap-2"><Megaphone class="size-5" /> Avisos</h2>
						<ul class="mt-3 space-y-3 overflow-hidden">
							{#each dados.avisos as a (a.id)}
								<li
									class="rounded-lg px-3 py-2 {a.nivel === 'critico'
										? 'bg-danger-soft text-danger'
										: a.nivel === 'atencao'
											? 'bg-warn-soft text-warn'
											: 'bg-sunken'}"
								>
									<div class="text-[1.1rem] font-semibold leading-snug {a.nivel === 'info' ? 'text-ink' : ''}">{a.titulo}</div>
									{#if a.mensagem}
										<p class="text-[0.95rem] text-ink-2 line-clamp-2">{a.mensagem}</p>
									{/if}
								</li>
							{/each}
						</ul>
					</section>
				{/if}
			</aside>
		</main>

		<footer class="flex items-center justify-between gap-4 px-8 pb-4 text-[0.9rem] text-ink-3 tabular">
			{#if dadosVelhos}
				<span class="flex items-center gap-2 font-semibold text-warn">
					<WifiOff class="size-4" /> Sem conexão com o servidor desde {horaCurta(ultimoSucesso)}. Os dados podem estar desatualizados.
				</span>
			{:else}
				<span>Atualizado às {horaCurta(ultimoSucesso)} · atualiza a cada {INTERVALO_MS / 1000} s</span>
			{/if}
			{#if erro === 'nao_autorizada'}
				<span class="font-semibold text-danger">{chave ? 'A chave desta tela foi revogada.' : 'A sessão foi encerrada.'}</span>
			{/if}
		</footer>
	{/if}

	<!-- Controles: aparecem ao mexer o mouse -->
	<div
		class="fixed bottom-4 right-4 flex gap-2 transition-opacity duration-300 {controlesVisiveis ? 'opacity-100' : 'opacity-0 pointer-events-none'}"
	>
		{#if temSessao}
			<a href="/" class="btn btn-secondary"><ArrowLeft class="size-4" /> Voltar ao sistema</a>
		{/if}
		<button onclick={alternarTelaCheia} class="btn btn-secondary">
			{#if telaCheia}
				<Minimize class="size-4" /> Sair da tela cheia
			{:else}
				<Maximize class="size-4" /> Tela cheia
			{/if}
		</button>
	</div>
</div>

<style>
	.estado {
		display: inline-flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.7rem 1.6rem;
		border-radius: 999px;
		font-size: 1.6rem;
		font-weight: 700;
		line-height: 1.2;
	}
	.titulo-bloco {
		font-size: 1rem;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--ink-3);
	}
	.pulso {
		animation: pulso 2s ease-in-out infinite;
	}
	@keyframes pulso {
		50% {
			opacity: 0.7;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.pulso {
			animation: none;
		}
	}
</style>
