<script lang="ts">
	import { onMount } from 'svelte';
	import { ModoTV, lerChaveTV, esquecerChaveTV } from '$lib/modoTV.svelte';
	import {
		CIDADES,
		daEscala,
		ehDomingo,
		proximoDomingo,
		proximoDiaInterno,
		diaDoInterno,
		diaComFeriado,
		diasFeriado,
		type Feriado,
		corDaPessoa,
		cobre,
		deFolga,
		diaLocal,
		paraDia,
		somarDias,
		diaSemana,
		quandoComeca,
		quantoFalta,
		nomeCidade,
		type Turno
	} from '$lib/plantao';
	import { Maximize, Minimize, ArrowLeft, WifiOff, Tv } from 'lucide-svelte';
	import '@fontsource-variable/archivo/wdth.css';
	import '$lib/tv.css';

	interface PlantaoTV {
		tela: string;
		setor: string;
		agora_servidor: string;
		hoje: string; // AAAA-MM-DD, no fuso do servidor
		escala: Turno[];
		feriados: Feriado[];
	}

	const FUSO = 'America/Sao_Paulo';
	const INTERVALO_MS = 60_000;
	// Sem resposta do servidor por mais que isso, a tela avisa que os dados podem estar velhos
	const DADOS_VELHOS_MS = 3 * 60_000;
	// Dias na faixa da escala, no rodapé
	const DIAS_FAIXA = 14;

	let dados = $state<PlantaoTV | null>(null);
	let erro = $state<'nao_autorizada' | 'limite' | null>(null);
	let ultimoSucesso = $state(0);
	let falhando = $state(false);
	let agora = $state(Date.now());
	// Diferença entre o relógio do servidor e o da TV
	let desvio = 0;
	let chave = $state<string | null>(null);
	let temSessao = $state(false);

	const tv = new ModoTV();

	async function carregar() {
		try {
			const res = await fetch('/api/tv/plantao', {
				credentials: 'include',
				headers: { Accept: 'application/json', ...(chave ? { 'X-TV-Chave': chave } : {}) }
			});
			if (res.status === 401) {
				if (chave) esquecerChaveTV();
				erro = 'nao_autorizada';
				return;
			}
			if (res.status === 429) {
				erro = 'limite';
				return;
			}
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			const d: PlantaoTV = await res.json();
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
	const dadosVelhos = $derived(ultimoSucesso > 0 && agora - ultimoSucesso > DADOS_VELHOS_MS);

	const hoje = $derived(dados?.hoje ?? paraDia(new Date()));
	const escala = $derived(dados?.escala ?? []);
	const feriados = $derived(diasFeriado(dados?.feriados ?? []));
	// Plantão interno (domingos e feriados): o de hoje ou o do próximo dia desses
	const diaInterno = $derived(proximoDiaInterno(hoje, feriados));
	const internoDoDia = $derived(escala.filter((t) => t.tipo === 'interno' && cobre(t, diaInterno)));
	// Técnicos externos: o noturno de hoje (o destaque da tela) e o plantão do
	// domingo (hoje ou o próximo)
	const domingo = $derived(proximoDomingo(hoje));
	const externos = $derived(
		CIDADES.map((c) => ({
			...c,
			noturno: escala.filter((t) => daEscala(t, 'noturno', c.id) && cobre(t, hoje)),
			domingo: escala.filter((t) => daEscala(t, 'domingo', c.id) && cobre(t, domingo))
		}))
	);
	const folgaAgora = $derived(dados?.escala.filter((t) => deFolga(t, hoje)) ?? []);
	// Próximas entradas na escala (turnos que ainda vão começar)
	// Próximas trocas do noturno; o interno e o domingo aparecem na faixa
	const proximos = $derived(escala.filter((t) => t.tipo === 'noturno' && t.inicio > hoje).slice(0, 7));
	const noturnoAgora = $derived(externos.flatMap((c) => c.noturno));

	const faixa = $derived(
		Array.from({ length: DIAS_FAIXA }, (_, i) => {
			const dia = paraDia(somarDias(diaLocal(hoje), i));
			const d = diaLocal(dia);
			return {
				dia,
				semana: d.toLocaleDateString('pt-BR', { weekday: 'short' }).replace('.', ''),
				numero: d.getDate(),
				fimDeSemana: d.getDay() === 0 || d.getDay() === 6,
				domingo: ehDomingo(dia),
				interno: escala.filter((t) => t.tipo === 'interno' && cobre(t, dia)),
				cidades: CIDADES.map((c) => ({
					noturno: escala.filter((t) => daEscala(t, 'noturno', c.id) && cobre(t, dia)),
					domingo: escala.filter((t) => daEscala(t, 'domingo', c.id) && cobre(t, dia))
				})),
				feriado: feriados.get(dia) ?? null,
				temInterno: diaDoInterno(dia, feriados),
				folga: escala.filter((t) => deFolga(t, dia))
			};
		})
	);

	// Nome do noturno o maior possível sem quebrar palavra: a maior palavra cabe
	// na largura da coluna (cqi), com teto quando divide a coluna com outro
	function tamanhoNome(nome: string, naColuna: number): string {
		const maiorPalavra = Math.max(...nome.trim().split(/\s+/).map((p) => p.length));
		const teto = naColuna > 1 ? 3.2 : 5.2;
		return `font-size: min(${teto}rem, ${Math.round(160 / Math.max(maiorPalavra, 4))}cqi);`;
	}

	function primeiroNome(nome: string): string {
		return nome.trim().split(/\s+/)[0];
	}

	function horaCurta(ms: number): string {
		return new Date(ms + desvio).toLocaleTimeString('pt-BR', { timeZone: FUSO, hour: '2-digit', minute: '2-digit' });
	}
</script>

<svelte:head>
	<title>{noturnoAgora.length ? `${noturnoAgora.map((t) => t.nome).join(', ')} · ` : ''}Plantão · Projetos NOC</title>
</svelte:head>

<div class="tv h-screen overflow-hidden flex flex-col {tv.controlesVisiveis ? '' : 'cursor-none'}">
	{#if erro === 'nao_autorizada' && !dados}
		<div class="flex-1 grid place-items-center p-8">
			<div class="max-w-[34rem] text-center space-y-4">
				<Tv class="size-14 mx-auto text-ink-3" strokeWidth={1.5} />
				<h1 class="text-[2rem] font-bold leading-tight">Tela não autorizada</h1>
				<p class="text-[1.125rem] text-ink-2">
					Para usar a TV do plantão sem login, um administrador cadastra esta tela em
					<strong class="text-ink">Monitor → Telas de TV</strong> e abre aqui o link do plantão.
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
		<header class="flex items-center gap-8 px-8 h-[7.5rem] shrink-0 bg-surface border-b-[3px] border-line">
			<div class="flex items-center gap-3 min-w-0 flex-1">
				<img src="/logo.svg" alt="" class="size-12 shrink-0" />
				<div class="min-w-0">
					<div class="tv-display text-[2.6rem] leading-none">Plantão noturno de hoje</div>
					<div class="mt-1 text-[1rem] text-ink-3 truncate">{dados.setor}, {dados.tela}</div>
				</div>
			</div>
			<div class="text-right shrink-0">
				<div class="tv-display text-[3.6rem] leading-none">
					{hora}<span class="text-[1.8rem] text-ink-3">:{segundos}</span>
				</div>
				<div class="mt-1 text-[1.05rem] text-ink-2 first-letter:uppercase">{data}</div>
			</div>
		</header>

		<main class="flex-1 min-h-0 grid grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)]">
			<!-- Destaque: quem está no noturno hoje, uma coluna por cidade -->
			<section class="min-h-0 grid grid-cols-3 gap-[3px] bg-line" aria-label="Plantão noturno hoje">
				{#each externos as c (c.id)}
					{#if c.noturno.length === 0}
						<article class="cidade bg-warn-soft">
							<h2 class="text-[1.5rem] font-bold text-warn">{c.nome}</h2>
							<div class="flex-1 flex flex-col justify-center">
								<p class="tv-display text-[4rem] leading-[1] text-warn">Sem técnico</p>
								<p class="mt-3 text-[1.25rem] text-warn/85">Ninguém escalado no noturno de hoje.</p>
							</div>
						</article>
					{:else}
						<article class="cidade bg-paper" style="--cor: {corDaPessoa(c.noturno[0].nome)};">
							<h2 class="text-[1.5rem] font-bold text-ink-2">{c.nome}</h2>
							<div class="flex-1 flex flex-col justify-center gap-7 min-w-0">
								{#each c.noturno as t (t.id)}
									<div class="min-w-0 pl-5 border-l-[0.4rem]" style="border-color: {corDaPessoa(t.nome)};">
										<p class="tv-display leading-[0.95] [overflow-wrap:anywhere] line-clamp-3" style={tamanhoNome(t.nome, c.noturno.length)}>
											{t.nome}
										</p>
										<p class="mt-3 text-[1.35rem] text-ink-2 tabular">
											{t.fim === hoje ? 'Última noite' : `Até ${diaSemana(t.fim)}, ${quantoFalta(t.fim, hoje)}`}
										</p>
										{#if t.observacao}
											<p class="mt-1 text-[1.2rem] text-ink-3 truncate">{t.observacao}</p>
										{/if}
									</div>
								{/each}
							</div>
						</article>
					{/if}
				{/each}
			</section>

			<!-- Lateral: domingo e interno, folga e próximas trocas -->
			<aside class="min-h-0 flex flex-col overflow-hidden border-l-[3px] border-line bg-sunken">
				<section class="px-7 pt-6 pb-6 border-b-[3px] border-line" aria-label="Plantão de domingo e interno">
					<h2 class="bloco">
						{ehDomingo(hoje) ? 'Hoje é domingo' : `Domingo, ${diaSemana(domingo).replace(/^\S+\s/, '')}`}
					</h2>
					<dl class="mt-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-5 gap-y-1 items-baseline">
						{#each externos as c (c.id)}
							<dt class="text-[1.1rem] text-ink-3">{c.nome}</dt>
							<dd class="tv-display text-[1.6rem] leading-tight truncate {c.domingo.length ? '' : 'text-warn'}">
								{c.domingo.length ? c.domingo.map((t) => t.nome).join(', ') : 'Ninguém escalado'}
							</dd>
						{/each}
						<dt class="text-[1.1rem] text-ink-3">
							Interno{diaInterno !== domingo ? `, ${diaInterno === hoje ? 'hoje' : diaComFeriado(diaInterno, feriados)}` : ''}
						</dt>
						<dd class="tv-display text-[1.6rem] leading-tight truncate {internoDoDia.length ? '' : 'text-warn'}">
							{internoDoDia.length ? internoDoDia.map((t) => t.nome).join(', ') : 'Ninguém escalado'}
						</dd>
					</dl>
				</section>

				{#if folgaAgora.length > 0}
					<section class="px-7 pt-5 pb-5 border-b-[3px] border-line" aria-label="De folga">
						<h2 class="bloco">De folga</h2>
						<ul class="mt-2 space-y-1.5">
							{#each folgaAgora as t (t.id)}
								<li class="flex items-baseline gap-3 min-w-0">
									<span class="size-3 rounded-full shrink-0 self-center" style="background-color: {corDaPessoa(t.nome)};"></span>
									<span class="tv-display text-[1.7rem] leading-tight text-ink-2 truncate">{t.nome}</span>
									<span class="ml-auto shrink-0 text-[1.05rem] text-ink-3 tabular">{t.folga_fim === hoje ? 'volta amanhã' : `até ${diaSemana(t.folga_fim!)}`}</span>
								</li>
							{/each}
						</ul>
					</section>
				{/if}

				<section class="px-7 pt-5 pb-5 min-h-0 flex flex-col overflow-hidden" aria-label="Próximas trocas do noturno">
					<h2 class="bloco">Próximas trocas do noturno</h2>
					{#if proximos.length === 0}
						<p class="mt-2 text-[1.15rem] text-ink-3">Nenhuma troca nas próximas semanas.</p>
					{:else}
						<ul class="lista-inteira mt-2">
							{#each proximos as t (t.id)}
								<li class="py-1.5 flex items-baseline gap-3 min-w-0">
									<span class="size-3 rounded-full shrink-0 self-center" style="background-color: {corDaPessoa(t.nome)};"></span>
									<span class="tv-display text-[1.6rem] leading-tight truncate">{t.nome}</span>
									<span class="text-[1.05rem] text-ink-3 truncate">{nomeCidade(t.cidade)}</span>
									<span class="ml-auto text-[1.1rem] font-bold text-ink-2 tabular shrink-0">{quandoComeca(t.inicio, hoje)}</span>
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			</aside>
		</main>

		<!-- Faixa das próximas duas semanas: por cidade, noturno em cheio e domingo tracejado; depois o interno -->
		<section class="shrink-0 border-t-[3px] border-line bg-sunken px-6 pt-3 pb-3" aria-label="Escala das próximas duas semanas">
			<div class="grid gap-x-[3px] gap-y-1" style="grid-template-columns: 8rem repeat({DIAS_FAIXA}, minmax(0, 1fr));">
				<span></span>
				{#each faixa as d (d.dia)}
					<div
						class="text-center pt-1 pb-1.5 rounded-t-md {d.dia === hoje ? 'bg-accent-soft text-accent' : d.feriado ? 'text-warn' : d.fimDeSemana ? 'text-ink-3' : 'text-ink-2'}"
						title={d.feriado ?? undefined}
					>
						<div class="text-[0.9rem] font-semibold">{d.dia === hoje ? 'hoje' : d.feriado ? 'feriado' : d.semana}</div>
						<div class="tv-display text-[1.5rem] leading-none">{d.numero}</div>
					</div>
				{/each}

				{#each CIDADES as c, i (c.id)}
					<span class="self-center text-[1rem] font-semibold text-ink-3 truncate">{c.nome}</span>
					{#each faixa as d (d.dia)}
						{@const cel = d.cidades[i]}
						{@const falta = cel.noturno.length === 0 || (d.domingo && cel.domingo.length === 0)}
						<div class="celula {falta ? 'bg-warn-soft' : d.dia === hoje ? 'bg-accent-soft' : ''} {d.dia === hoje ? 'hoje' : ''}">
							{#each cel.noturno as t (t.id)}
								<span class="chip" style="background-color: {corDaPessoa(t.nome)}" title="{t.nome}, noturno">{primeiroNome(t.nome)}</span>
							{/each}
							{#each cel.domingo as t (t.id)}
								<span class="chip chip-domingo" style="border-color: {corDaPessoa(t.nome)}" title="{t.nome}, domingo">{primeiroNome(t.nome)}</span>
							{/each}
							{#if falta && cel.noturno.length + cel.domingo.length === 0}
								<span class="text-center text-[0.95rem] font-bold text-warn">—</span>
							{/if}
						</div>
					{/each}
				{/each}

				<span class="self-center text-[1rem] font-semibold text-ink-3">Interno</span>
				{#each faixa as d (d.dia)}
					<div class="celula {d.temInterno && d.interno.length === 0 ? 'bg-warn-soft' : d.dia === hoje ? 'bg-accent-soft' : ''} {d.dia === hoje ? 'hoje' : ''}">
						{#each d.interno as t (t.id)}
							<span class="chip" style="background-color: {corDaPessoa(t.nome)}" title={t.nome}>{primeiroNome(t.nome)}</span>
						{:else}
							{#if d.temInterno}<span class="text-center text-[0.95rem] font-bold text-warn">—</span>{/if}
						{/each}
					</div>
				{/each}

				{#if faixa.some((d) => d.folga.length > 0)}
					<span class="self-center text-[1rem] font-semibold text-ink-3">Folga</span>
					{#each faixa as d (d.dia)}
						<div class="celula !min-h-[2rem] {d.dia === hoje ? 'bg-accent-soft rounded-b-md' : ''}">
							{#each d.folga as t (t.id)}
								<span class="flex items-center gap-1.5 min-w-0 text-[0.95rem] text-ink-2 leading-snug" title={t.nome}>
									<span class="size-2 rounded-full shrink-0" style="background-color: {corDaPessoa(t.nome)};"></span>
									<span class="truncate">{primeiroNome(t.nome)}</span>
								</span>
							{/each}
						</div>
					{/each}
				{/if}
			</div>
		</section>

		<footer class="flex items-center justify-between gap-4 px-8 h-11 shrink-0 border-t-[3px] border-line text-[0.95rem] text-ink-3 tabular">
			{#if dadosVelhos}
				<span class="flex items-center gap-2 font-semibold text-warn">
					<WifiOff class="size-4" /> Sem conexão com o servidor desde {horaCurta(ultimoSucesso)}. A escala pode estar desatualizada.
				</span>
			{:else}
				<span>Atualizado às {horaCurta(ultimoSucesso)}, a cada minuto</span>
			{/if}
			{#if erro === 'nao_autorizada'}
				<span class="font-semibold text-danger">{chave ? 'A chave desta tela foi revogada.' : 'A sessão foi encerrada.'}</span>
			{:else}
				<span class="flex items-center gap-5">
					<span class="flex items-center gap-2"><span class="chip !inline-block w-6 bg-ink-3"></span> noturno</span>
					<span class="flex items-center gap-2"><span class="chip chip-domingo !inline-block w-6 border-ink-3"></span> domingo</span>
					<span>Interno aos domingos e feriados</span>
				</span>
			{/if}
		</footer>
	{/if}

	<div
		class="fixed bottom-14 right-4 flex gap-2 transition-opacity duration-300 {tv.controlesVisiveis ? 'opacity-100' : 'opacity-0 pointer-events-none'}"
	>
		{#if temSessao}
			<a href="/plantao" class="btn btn-secondary"><ArrowLeft class="size-4" /> Voltar ao plantão</a>
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
	.cidade {
		container-type: inline-size;
		display: flex;
		flex-direction: column;
		gap: 1rem;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
		padding: 1.6rem 2rem 2rem;
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
	.celula {
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: 2px;
		min-height: 2.6rem;
		padding: 0.2rem 0.3rem;
		overflow: hidden;
		border-radius: 0.3rem;
	}
	/* Hoje continua marcado mesmo quando a célula está em alerta */
	.celula.hoje {
		box-shadow: inset 0 0 0 2px var(--accent);
	}
	.chip {
		display: block;
		min-height: 1.1rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		border-radius: 0.25rem;
		padding: 0 0.4rem;
		font-family: 'Archivo Variable', var(--font-sans);
		font-stretch: 85%;
		font-size: 1.05rem;
		font-weight: 700;
		line-height: 1.45;
		color: #fff;
	}
	.chip-domingo {
		background: transparent;
		border: 2px dashed;
		color: var(--ink);
		line-height: 1.3;
	}
</style>
