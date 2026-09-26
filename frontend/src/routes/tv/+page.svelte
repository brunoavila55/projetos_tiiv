<script lang="ts">
	import { onMount } from 'svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import { ModoTV, lerChaveTV, esquecerChaveTV } from '$lib/modoTV.svelte';
	import { corDaPessoa, rotuloEscala, type Turno } from '$lib/plantao';
	import { Maximize, Minimize, ArrowLeft, WifiOff, Tv } from 'lucide-svelte';
	import '$lib/tv.css';
	import type { Modulo } from '$lib/modulos';

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
		plantao_hoje: Turno[];
		avisos: { id: string; titulo: string; mensagem: string; nivel: 'info' | 'atencao' | 'critico'; criador_nome: string }[];
		// Módulos ligados no setor: os quadros dos desligados não aparecem
		modulos: Modulo[];
	}

	const FUSO = 'America/Sao_Paulo';
	const INTERVALO_MS = 15_000;
	// Sem resposta do servidor por mais que isso, a tela avisa que os dados podem estar velhos
	const DADOS_VELHOS_MS = 60_000;

	let dados = $state<PainelTV | null>(null);
	let erro = $state<'nao_autorizada' | 'desativado' | 'limite' | null>(null);
	let ultimoSucesso = $state(0);
	let falhando = $state(false);
	let agora = $state(Date.now());
	// Diferença entre o relógio do servidor e o da TV (TVs costumam ter o relógio errado)
	let desvio = 0;
	let chave = $state<string | null>(null);
	let temSessao = $state(false);

	const tv = new ModoTV();

	async function carregar() {
		try {
			const res = await fetch('/api/tv/painel', {
				credentials: 'include',
				headers: { Accept: 'application/json', ...(chave ? { 'X-TV-Chave': chave } : {}) }
			});
			if (res.status === 401) {
				// Chave revogada: esquece para não ficar tentando
				if (chave) esquecerChaveTV();
				erro = 'nao_autorizada';
				return;
			}
			if (res.status === 403) {
				// Modo TV desligado no setor da tela
				erro = 'desativado';
				dados = null;
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
		chave = lerChaveTV();
		const encerrarTV = tv.iniciar({ aoVoltarVisivel: carregar, podeRecarregar: () => !falhando });

		carregar();
		const timerDados = setInterval(() => {
			// Tela recusada: só volta com um novo link ou login
			if (erro === 'nao_autorizada' || erro === 'desativado') return;
			carregar();
		}, INTERVALO_MS);
		const timerRelogio = setInterval(() => (agora = Date.now()), 1000);
		return () => {
			clearInterval(timerDados);
			clearInterval(timerRelogio);
			encerrarTV();
		};
	});

	const agoraServidor = $derived(agora + desvio);
	const hora = $derived(new Date(agoraServidor).toLocaleTimeString('pt-BR', { timeZone: FUSO, hour: '2-digit', minute: '2-digit' }));
	const segundos = $derived(new Date(agoraServidor).toLocaleTimeString('pt-BR', { timeZone: FUSO, second: '2-digit' }).padStart(2, '0'));
	const data = $derived(new Date(agoraServidor).toLocaleDateString('pt-BR', { timeZone: FUSO, weekday: 'long', day: 'numeric', month: 'long' }));

	const fora = $derived(dados?.monitores.filter((m) => m.status === 'offline') ?? []);
	const demais = $derived(dados?.monitores.filter((m) => m.status !== 'offline') ?? []);
	const noAr = $derived(dados?.monitores.filter((m) => m.status === 'online').length ?? 0);
	// Serviços fora do ar ficam na frente e ocupam duas janelas
	const janelas = $derived(fora.length * 2 + demais.length);
	// Colunas da grade: o suficiente para as janelas preencherem a altura sem ficarem achatadas
	const colunas = $derived(janelas <= 4 ? 2 : janelas <= 9 ? 3 : janelas <= 16 ? 4 : janelas <= 25 ? 5 : 6);
	// Janelas vazias que completam a última linha
	const vazias = $derived((colunas - (janelas % colunas)) % colunas);
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

	const tem = (m: Modulo) => dados?.modulos.includes(m) ?? false;
</script>

<svelte:head>
	<title>{fora.length > 0 ? `(${fora.length}) ` : ''}Modo TV · Projetos NOC</title>
</svelte:head>

<div class="h-screen overflow-hidden bg-paper text-ink flex flex-col {tv.controlesVisiveis ? '' : 'cursor-none'}">
	{#if erro === 'nao_autorizada' && !dados}
		<div class="flex-1 grid place-items-center p-8">
			<div class="max-w-[34rem] text-center space-y-4">
				<Tv class="size-14 mx-auto text-ink-3" strokeWidth={1.5} />
				<h1 class="text-[2rem] font-bold leading-tight">Tela não autorizada</h1>
				<p class="text-[1.125rem] text-ink-2">
					Para usar o modo TV sem login, um administrador cadastra esta tela em
					<strong class="text-ink">Operadores → Telas de TV</strong> e abre aqui o link gerado.
				</p>
				<a href="/" class="btn btn-secondary btn-lg">Entrar no sistema</a>
			</div>
		</div>
	{:else if erro === 'desativado'}
		<div class="flex-1 grid place-items-center p-8">
			<div class="max-w-[34rem] text-center space-y-4">
				<Tv class="size-14 mx-auto text-ink-3" strokeWidth={1.5} />
				<h1 class="text-[2rem] font-bold leading-tight">Modo TV desativado</h1>
				<p class="text-[1.125rem] text-ink-2">O modo TV está desligado para o setor desta tela. O superadmin religa em Operadores → Setores.</p>
				<a href="/" class="btn btn-secondary btn-lg">Voltar ao sistema</a>
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
		<header class="flex items-center gap-6 px-6 pt-5 pb-4 shrink-0">
			<div class="flex items-center gap-3 min-w-0">
				<img src="/logo.svg" alt="" class="size-12 shrink-0" />
				<div class="min-w-0">
					<div class="text-[1.5rem] font-bold leading-tight tracking-[-0.01em]">Projetos NOC</div>
					<div class="text-[0.95rem] text-ink-3 truncate">{dados.setor}, {dados.tela}</div>
				</div>
			</div>

			<!-- Estado geral: a peça que se lê do outro lado da sala -->
			{#if tem('monitor')}
				<div
					class="estado flex-1 min-w-0 {dados.monitores.length === 0
						? 'bg-muted text-ink-2'
						: fora.length > 0
							? 'bg-danger text-paper'
							: 'bg-ok-soft text-ok'}"
					role="status"
				>
					{#if dados.monitores.length === 0}
						Nenhum serviço monitorado
					{:else if fora.length > 0}
						<span class="tv-lampada tv-pisca !size-[1.1rem]"></span>
						<span class="truncate">{fora.length === 1 ? `${fora[0].nome} fora do ar` : `${fora.length} serviços fora do ar`}</span>
					{:else}
						<span class="tv-lampada !size-[1.1rem]"></span>
						<span class="truncate">
							{noAr === dados.monitores.length
								? `Todos os ${noAr} serviços no ar`
								: `${noAr} de ${dados.monitores.length} serviços confirmados no ar`}
						</span>
					{/if}
				</div>
			{:else}
				<div class="flex-1"></div>
			{/if}

			<div class="text-right shrink-0">
				<div class="tv-display text-[3.5rem] leading-none">
					{hora}<span class="text-[1.75rem] text-ink-3 font-semibold">:{segundos}</span>
				</div>
				<div class="mt-1 text-[1.05rem] text-ink-2 first-letter:uppercase">{data}</div>
			</div>
		</header>

		<main class="flex-1 min-h-0 grid {tem('monitor') ? 'grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)]' : 'grid-cols-1'} gap-5 px-6 pb-3">
			<!-- Monitores: um cartão por serviço, preenchendo a altura toda -->
			{#if tem('monitor')}
				<section
					class="min-h-0 grid gap-3 auto-rows-[minmax(0,1fr)]"
					style="grid-template-columns: repeat({colunas}, minmax(0, 1fr));"
					aria-label="Monitores"
				>
					{#each fora as m (m.id)}
						<article class="servico bg-danger text-paper {colunas > 2 ? 'col-span-2' : ''}">
							<h2 class="tv-display text-[2.2rem] leading-[1.1] line-clamp-2">{m.nome}</h2>
							<p class="mt-auto text-[1.35rem] font-bold">Fora do ar há {duracao(m.status_desde)}</p>
							{#if m.ultimo_erro}
								<p class="text-[1.05rem] opacity-80 truncate">{m.ultimo_erro}</p>
							{/if}
						</article>
					{/each}
					{#each demais as m (m.id)}
						<article class="servico bg-surface border border-line">
							<div class="flex items-start gap-3 min-w-0">
								<span class="tv-lampada mt-[0.5rem] {m.status === 'online' ? 'text-ok' : 'text-ink-3'}"></span>
								<h2 class="tv-display text-[1.6rem] leading-[1.2] line-clamp-2 min-w-0 {m.status === 'pendente' ? 'text-ink-2' : ''}">{m.nome}</h2>
							</div>
							<p class="mt-auto text-[1.05rem] text-ink-3 tabular truncate">
								{#if m.status === 'pendente'}
									Aguardando a primeira verificação
								{:else}
									{#if m.latencia_ms != null}<span class="text-ink-2 font-semibold">{m.latencia_ms} ms</span>{:else}No ar{/if}{#if m.disponibilidade_24h != null}
										<span class="ml-3 {m.disponibilidade_24h < 99 ? 'text-warn' : ''}">{formatarPct(m.disponibilidade_24h)} em 24 h</span>
									{/if}
								{/if}
							</p>
						</article>
					{/each}
					{#each { length: vazias } as _}
						<div class="rounded-xl border border-dashed border-line"></div>
					{/each}
				</section>
			{/if}

			<!-- Coluna lateral: plantão, fila e avisos -->
			<aside class="min-h-0 flex flex-col gap-4 overflow-hidden">
				{#if tem('plantao')}
					<section class="panel px-5 py-4 shrink-0">
						<h2 class="bloco">De plantão hoje</h2>
						{#if dados.plantao_hoje.length === 0}
							<p class="mt-2 text-[1.1rem] text-warn">Ninguém escalado para hoje.</p>
						{:else}
							<ul class="mt-3 space-y-3">
								{#each dados.plantao_hoje as p (p.id)}
									<li class="flex items-center gap-3 min-w-0">
										<Avatar id={p.id} nome={p.nome} cor={corDaPessoa(p.nome)} class="size-11 text-[0.95rem]" />
										<div class="min-w-0">
											<div class="text-[1.4rem] font-bold leading-tight truncate">{p.nome}</div>
											<div class="text-[0.95rem] text-ink-3 truncate">
												{rotuloEscala(p)}{p.observacao ? `, ${p.observacao}` : ''}
											</div>
										</div>
									</li>
								{/each}
							</ul>
						{/if}
					</section>
				{/if}

				{#if tem('tickets')}
					<section class="panel px-5 py-4 min-h-0 flex flex-col">
						<div class="flex items-center justify-between gap-3">
							<h2 class="bloco">Tickets na fila</h2>
							<span
								class="min-w-[2.5rem] h-[2.5rem] px-2.5 rounded-full grid place-items-center text-[1.4rem] font-bold tabular {dados.tickets_total > 0
									? 'bg-accent text-on-accent'
									: 'bg-muted text-ink-3'}">{dados.tickets_total}</span
							>
						</div>
						{#if dados.tickets.length === 0}
							<p class="mt-2 text-[1.1rem] text-ink-3">Fila vazia.</p>
						{:else}
							<ul class="tv-lista mt-3">
								{#each dados.tickets as tk (tk.numero)}
									<li class="py-2 border-t border-line first:border-t-0 min-w-0 flex gap-3">
										<span
											class="w-1 self-stretch rounded-full shrink-0 {tk.prioridade === 'alta'
												? 'bg-danger'
												: tk.prioridade === 'media'
													? 'bg-warn'
													: 'bg-line-strong'}"
										></span>
										<div class="min-w-0 flex-1">
											<div class="text-[1.15rem] font-semibold leading-snug truncate">{tk.titulo}</div>
											<div class="text-[0.95rem] text-ink-3 truncate tabular">
												#{tk.numero}, {tk.solicitante_nome}, aberto há {duracao(tk.criado_em)}
											</div>
										</div>
										{#if tk.prioridade === 'alta'}
											<span class="tag tag-danger self-center shrink-0 !text-[0.9rem] !h-7">{rotuloPrioridade.alta}</span>
										{/if}
									</li>
								{/each}
							</ul>
						{/if}
					</section>
				{/if}

				{#if dados.avisos.length > 0}
					<section class="panel px-5 py-4 min-h-0 flex flex-col">
						<h2 class="bloco">Avisos</h2>
						<ul class="tv-lista mt-3 gap-2.5">
							{#each dados.avisos as a (a.id)}
								<li
									class="rounded-lg px-3 py-2 {a.nivel === 'critico'
										? 'bg-danger-soft text-danger'
										: a.nivel === 'atencao'
											? 'bg-warn-soft text-warn'
											: 'bg-sunken'}"
								>
									<div class="text-[1.15rem] font-semibold leading-snug {a.nivel === 'info' ? 'text-ink' : ''}">{a.titulo}</div>
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

		<footer class="flex items-center justify-between gap-4 px-6 pb-3 text-[0.9rem] text-ink-3 tabular">
			{#if dadosVelhos}
				<span class="flex items-center gap-2 font-semibold text-warn">
					<WifiOff class="size-4" /> Sem conexão com o servidor desde {horaCurta(ultimoSucesso)}. Os dados podem estar desatualizados.
				</span>
			{:else}
				<span>Atualizado às {horaCurta(ultimoSucesso)}, a cada {INTERVALO_MS / 1000} s</span>
			{/if}
			{#if erro === 'nao_autorizada'}
				<span class="font-semibold text-danger">{chave ? 'A chave desta tela foi revogada.' : 'A sessão foi encerrada.'}</span>
			{/if}
		</footer>
	{/if}

	<!-- Controles: aparecem ao mexer o mouse -->
	<div
		class="fixed bottom-4 right-4 flex gap-2 transition-opacity duration-300 {tv.controlesVisiveis ? 'opacity-100' : 'opacity-0 pointer-events-none'}"
	>
		{#if temSessao}
			<a href="/" class="btn btn-secondary"><ArrowLeft class="size-4" /> Voltar ao sistema</a>
		{/if}
		<button onclick={() => tv.alternarTelaCheia()} class="btn btn-secondary">
			{#if tv.telaCheia}
				<Minimize class="size-4" /> Sair da tela cheia
			{:else}
				<Maximize class="size-4" /> Tela cheia
			{/if}
		</button>
	</div>
</div>

<style>
	.estado {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.9rem;
		height: 4.5rem;
		padding: 0 1.75rem;
		border-radius: 0.9rem;
		font-size: 2.1rem;
		font-weight: 700;
		letter-spacing: -0.01em;
		line-height: 1.1;
	}
	.servico {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
		padding: 1rem 1.25rem;
		border-radius: 0.75rem;
	}
	.bloco {
		font-size: 1.2rem;
		font-weight: 700;
		color: var(--ink-2);
	}
</style>
