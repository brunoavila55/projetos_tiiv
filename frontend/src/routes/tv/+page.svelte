<script lang="ts">
	import { onMount } from 'svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import { ModoTV, lerChaveTV, esquecerChaveTV } from '$lib/modoTV.svelte';
	import { corDaPessoa, rotuloEscala, type Turno } from '$lib/plantao';
	import { Maximize, Minimize, ArrowLeft, WifiOff, Tv } from 'lucide-svelte';
	import '@fontsource-variable/archivo/wdth.css';
	import '$lib/tv.css';

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
	}

	const FUSO = 'America/Sao_Paulo';
	const INTERVALO_MS = 15_000;
	// Sem resposta do servidor por mais que isso, a tela avisa que os dados podem estar velhos
	const DADOS_VELHOS_MS = 60_000;

	let dados = $state<PainelTV | null>(null);
	let erro = $state<'nao_autorizada' | 'limite' | null>(null);
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
			if (erro === 'nao_autorizada') return;
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
</script>

<svelte:head>
	<title>{fora.length > 0 ? `(${fora.length}) ` : ''}Modo TV · Projetos NOC</title>
</svelte:head>

<div class="tv h-screen overflow-hidden flex flex-col {tv.controlesVisiveis ? '' : 'cursor-none'}">
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
		<!-- Faixa de estado: a peça que se lê do outro lado da sala. Acende inteira quando algo cai. -->
		<header
			class="faixa flex items-center gap-8 px-8 h-[7.5rem] shrink-0 {fora.length > 0 ? 'faixa-alarme' : ''}"
			role="status"
		>
			<div class="flex items-center gap-3 min-w-0 w-[24%]">
				<img src="/logo.svg" alt="" class="size-12 shrink-0" />
				<div class="min-w-0">
					<div class="text-[1.4rem] font-bold leading-tight">Projetos NOC</div>
					<div class="text-[1rem] opacity-75 truncate">{dados.tela}</div>
				</div>
			</div>

			<div class="flex-1 min-w-0 flex items-center gap-4">
				{#if dados.monitores.length === 0}
					<span class="tv-lampada text-ink-3"></span>
					<span class="tv-display text-[2.6rem] leading-none text-ink-2">Nenhum serviço monitorado</span>
				{:else if fora.length > 0}
					<span class="tv-lampada tv-pisca !size-[1.4rem] text-white"></span>
					<span class="tv-display text-[3.4rem] leading-none truncate">
						{fora.length === 1 ? `${fora[0].nome} fora do ar` : `${fora.length} serviços fora do ar`}
					</span>
				{:else}
					<span class="tv-lampada !size-[1.1rem] text-ok"></span>
					<span class="tv-display text-[2.6rem] leading-none">
						{noAr === dados.monitores.length
							? `Todos os ${noAr} serviços no ar`
							: `${noAr} de ${dados.monitores.length} serviços confirmados no ar`}
					</span>
				{/if}
			</div>

			<div class="text-right shrink-0">
				<div class="tv-display text-[3.6rem] leading-none">
					{hora}<span class="text-[1.8rem] opacity-60">:{segundos}</span>
				</div>
				<div class="mt-1 text-[1.05rem] opacity-75 first-letter:uppercase">{data}</div>
			</div>
		</header>

		<main class="flex-1 min-h-0 grid grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)]">
			<!-- Monitores: uma janela por serviço, preenchendo a altura toda -->
			<section
				class="min-h-0 grid gap-[3px] p-[3px] bg-line auto-rows-[minmax(0,1fr)]"
				style="grid-template-columns: repeat({colunas}, minmax(0, 1fr));"
				aria-label="Monitores"
			>
				{#each fora as m (m.id)}
					<article class="janela janela-alarme {colunas > 2 ? 'col-span-2' : ''}">
						<h2 class="tv-display text-[2.4rem] leading-[1.05] line-clamp-2">{m.nome}</h2>
						<p class="mt-auto text-[1.35rem] font-bold">Fora do ar há {duracao(m.status_desde)}</p>
						{#if m.ultimo_erro}
							<p class="text-[1.05rem] opacity-85 truncate">{m.ultimo_erro}</p>
						{/if}
					</article>
				{/each}
				{#each demais as m (m.id)}
					<article class="janela {m.status === 'pendente' ? 'janela-pendente' : ''}">
						<div class="flex items-start gap-3 min-w-0">
							<span class="tv-lampada mt-[0.55rem] {m.status === 'online' ? 'text-ok' : 'text-ink-3 !shadow-none'}"></span>
							<h2 class="tv-display text-[1.9rem] leading-[1.1] line-clamp-2 min-w-0">{m.nome}</h2>
						</div>
						<p class="mt-auto text-[1.05rem] text-ink-3 tabular truncate">
							{#if m.status === 'pendente'}
								Aguardando a primeira verificação
							{:else}
								{#if m.latencia_ms != null}<span class="text-ink-2">{m.latencia_ms} ms</span>{:else}No ar{/if}{#if m.disponibilidade_24h != null}
									<span class="ml-3">{formatarPct(m.disponibilidade_24h)} em 24 h</span>
								{/if}
							{/if}
						</p>
					</article>
				{/each}
				{#each { length: vazias } as _}
					<div class="bg-paper"></div>
				{/each}
			</section>

			<!-- Coluna lateral: plantão, fila e avisos -->
			<aside class="min-h-0 flex flex-col overflow-hidden border-l-[3px] border-line bg-sunken">
				<section class="px-7 pt-6 pb-6 border-b-[3px] border-line">
					<h2 class="bloco">De plantão hoje</h2>
					{#if dados.plantao_hoje.length === 0}
						<p class="mt-3 text-[1.15rem] text-warn">Ninguém escalado para hoje.</p>
					{:else}
						<ul class="mt-4 space-y-4">
							{#each dados.plantao_hoje as p (p.id)}
								<li class="flex items-center gap-4 min-w-0">
									<Avatar id={p.id} nome={p.nome} cor={corDaPessoa(p.nome)} class="size-12 text-[1rem]" />
									<div class="min-w-0">
										<div class="tv-display text-[2rem] leading-none truncate">{p.nome}</div>
										<div class="mt-1 text-[1rem] text-ink-3 truncate">
											{rotuloEscala(p)}{p.observacao ? `, ${p.observacao}` : ''}
										</div>
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<section class="px-7 pt-6 pb-5 min-h-0 flex flex-col overflow-hidden border-b-[3px] border-line last:border-b-0">
					<div class="flex items-baseline justify-between gap-3">
						<h2 class="bloco">Tickets na fila</h2>
						<span class="tv-display text-[3rem] leading-none {dados.tickets_total > 0 ? 'text-ink' : 'text-ink-3'}">{dados.tickets_total}</span>
					</div>
					{#if dados.tickets.length === 0}
						<p class="mt-2 text-[1.15rem] text-ink-3">Fila vazia.</p>
					{:else}
						<ul class="lista-inteira mt-3">
							{#each dados.tickets as tk (tk.numero)}
								<li class="py-2.5 border-t border-line first:border-t-0 min-w-0 flex gap-3">
									<span
										class="w-1.5 self-stretch rounded-full shrink-0 {tk.prioridade === 'alta'
											? 'bg-danger'
											: tk.prioridade === 'media'
												? 'bg-warn'
												: 'bg-line-strong'}"
										title={rotuloPrioridade[tk.prioridade]}
									></span>
									<div class="min-w-0">
										<div class="text-[1.2rem] font-semibold leading-snug truncate">{tk.titulo}</div>
										<div class="text-[0.95rem] text-ink-3 truncate tabular">
											#{tk.numero}, {tk.solicitante_nome}, aberto há {duracao(tk.criado_em)}
										</div>
									</div>
									{#if tk.prioridade === 'alta'}
										<span class="ml-auto shrink-0 self-center text-[0.95rem] font-bold text-danger">Alta</span>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				{#if dados.avisos.length > 0}
					<section class="px-7 pt-6 pb-5 min-h-0 flex flex-col overflow-hidden">
						<h2 class="bloco">Avisos</h2>
						<ul class="lista-inteira mt-3 gap-3">
							{#each dados.avisos as a (a.id)}
								<li
									class="pl-4 border-l-4 {a.nivel === 'critico'
										? 'border-danger'
										: a.nivel === 'atencao'
											? 'border-warn'
											: 'border-accent'}"
								>
									<div
										class="text-[1.2rem] font-bold leading-snug {a.nivel === 'critico'
											? 'text-danger'
											: a.nivel === 'atencao'
												? 'text-warn'
												: 'text-ink'}"
									>
										{a.titulo}
									</div>
									{#if a.mensagem}
										<p class="text-[1rem] text-ink-2 line-clamp-2">{a.mensagem}</p>
									{/if}
								</li>
							{/each}
						</ul>
					</section>
				{/if}
			</aside>
		</main>

		<footer class="flex items-center justify-between gap-4 px-8 h-11 shrink-0 border-t-[3px] border-line text-[0.95rem] text-ink-3 tabular">
			{#if dadosVelhos}
				<span class="flex items-center gap-2 font-semibold text-warn">
					<WifiOff class="size-4" /> Sem conexão com o servidor desde {horaCurta(ultimoSucesso)}. Os dados podem estar desatualizados.
				</span>
			{:else}
				<span>Atualizado às {horaCurta(ultimoSucesso)}, a cada {INTERVALO_MS / 1000} s</span>
			{/if}
			{#if erro === 'nao_autorizada'}
				<span class="font-semibold text-danger">{chave ? 'A chave desta tela foi revogada.' : 'A sessão foi encerrada.'}</span>
			{:else}
				<span>Plantão, tickets e avisos do setor {dados.setor}</span>
			{/if}
		</footer>
	{/if}

	<!-- Controles: aparecem ao mexer o mouse -->
	<div
		class="fixed bottom-14 right-4 flex gap-2 transition-opacity duration-300 {tv.controlesVisiveis ? 'opacity-100' : 'opacity-0 pointer-events-none'}"
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
	.faixa {
		background: var(--surface);
		border-bottom: 3px solid var(--line);
		color: var(--ink);
	}
	.faixa-alarme {
		background: var(--alarme);
		border-bottom-color: var(--alarme);
		color: var(--on-alarme);
	}
	.janela {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
		padding: 1.1rem 1.4rem;
		background: var(--paper);
	}
	.janela-pendente {
		color: var(--ink-2);
	}
	.janela-alarme {
		background: var(--alarme);
		color: var(--on-alarme);
		padding: 1.3rem 1.6rem;
	}
	/* Só mostra itens inteiros: o que não cabe quebra para uma coluna escondida */
	.lista-inteira {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-flow: column wrap;
		overflow: hidden;
	}
	.lista-inteira > :global(li) {
		width: 100%;
	}
	.bloco {
		font-size: 1.3rem;
		font-weight: 700;
		color: var(--ink-2);
	}
</style>
