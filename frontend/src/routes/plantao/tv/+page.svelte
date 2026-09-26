<script lang="ts">
	import { onMount } from 'svelte';
	import { ModoTV, lerChaveTV, esquecerChaveTV } from '$lib/modoTV.svelte';
	import {
		CIDADES,
		PERIODOS,
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
		type Turno
	} from '$lib/plantao';
	import { Maximize, Minimize, ArrowLeft, WifiOff, Tv } from 'lucide-svelte';
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
	// Só São Gabriel tem plantão noturno; as outras cidades têm só o de domingo
	const CIDADE_NOTURNO = 'sao_gabriel';

	let dados = $state<PlantaoTV | null>(null);
	let erro = $state<'nao_autorizada' | 'desativado' | 'limite' | null>(null);
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
			if (res.status === 403) {
				// Modo TV ou plantão desligado no setor da tela
				erro = 'desativado';
				dados = null;
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
	const dadosVelhos = $derived(ultimoSucesso > 0 && agora - ultimoSucesso > DADOS_VELHOS_MS);

	const hoje = $derived(dados?.hoje ?? paraDia(new Date()));
	const escala = $derived(dados?.escala ?? []);
	const feriados = $derived(diasFeriado(dados?.feriados ?? []));
	// Plantão interno (domingos e feriados): o de hoje ou o do próximo dia desses
	const diaInterno = $derived(proximoDiaInterno(hoje, feriados));
	const internoDoDia = $derived(escala.filter((t) => t.tipo === 'interno' && cobre(t, diaInterno)));
	const internoPorPeriodo = $derived(PERIODOS.map((p) => ({ ...p, turnos: internoDoDia.filter((t) => t.periodo === p.id) })));
	// Técnicos externos: o plantão do domingo (hoje ou o próximo), o destaque da
	// tela junto com o interno, e o noturno de hoje, que só existe em São Gabriel
	const domingo = $derived(proximoDomingo(hoje));
	const externos = $derived(
		CIDADES.map((c) => ({ ...c, domingo: escala.filter((t) => daEscala(t, 'domingo', c.id) && cobre(t, domingo)) }))
	);
	const noturnoAgora = $derived(escala.filter((t) => daEscala(t, 'noturno', CIDADE_NOTURNO) && cobre(t, hoje)));
	const folgaAgora = $derived(dados?.escala.filter((t) => deFolga(t, hoje)) ?? []);
	// Próximas trocas do noturno; o interno e o domingo aparecem na faixa
	const proximos = $derived(escala.filter((t) => daEscala(t, 'noturno', CIDADE_NOTURNO) && t.inicio > hoje).slice(0, 7));

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
				interno: PERIODOS.map((p) => escala.filter((t) => daEscala(t, 'interno', null, p.id) && cobre(t, dia))),
				cidades: CIDADES.map((c) => ({
					temNoturno: c.id === CIDADE_NOTURNO,
					noturno: c.id === CIDADE_NOTURNO ? escala.filter((t) => daEscala(t, 'noturno', c.id) && cobre(t, dia)) : [],
					domingo: escala.filter((t) => daEscala(t, 'domingo', c.id) && cobre(t, dia))
				})),
				feriado: feriados.get(dia) ?? null,
				temInterno: diaDoInterno(dia, feriados),
				folga: escala.filter((t) => deFolga(t, dia))
			};
		})
	);

	// Nome nos cartões o maior possível sem quebrar palavra: a maior palavra cabe
	// na largura do cartão (cqi), com teto quando divide o cartão com outro
	function tamanhoNome(nome: string, noCartao: number): string {
		const maiorPalavra = Math.max(...nome.trim().split(/\s+/).map((p) => p.length));
		const teto = noCartao > 1 ? 2.2 : 3.4;
		return `font-size: min(${teto}rem, ${Math.round(135 / Math.max(maiorPalavra, 4))}cqi);`;
	}

	function primeiroNome(nome: string): string {
		return nome.trim().split(/\s+/)[0];
	}

	function horaCurta(ms: number): string {
		return new Date(ms + desvio).toLocaleTimeString('pt-BR', { timeZone: FUSO, hour: '2-digit', minute: '2-digit' });
	}
</script>

{#snippet cartao(titulo: string, turnos: Turno[])}
	{#if turnos.length === 0}
		<article class="cartao bg-warn-soft border-2 border-warn">
			<p class="text-[1.25rem] font-bold text-warn">{titulo}</p>
			<div class="flex-1 flex flex-col justify-center">
				<p class="tv-display text-[2.4rem] leading-[1.05] text-warn">Sem escala</p>
				<p class="mt-1 text-[1.05rem] text-warn">Ninguém escalado.</p>
			</div>
		</article>
	{:else}
		<article class="cartao bg-surface border border-line">
			<p class="text-[1.25rem] font-bold text-ink-2">{titulo}</p>
			<div class="flex-1 flex flex-col justify-center gap-4 min-w-0">
				{#each turnos as t (t.id)}
					<div class="min-w-0 pl-4 border-l-[0.4rem] rounded-l-sm" style="border-color: {corDaPessoa(t.nome)};">
						<p class="tv-display leading-[1.05] [overflow-wrap:anywhere] line-clamp-2" style={tamanhoNome(t.nome, turnos.length)}>
							{t.nome}
						</p>
						{#if t.observacao}
							<p class="mt-1 text-[1.05rem] text-ink-3 truncate">{t.observacao}</p>
						{/if}
					</div>
				{/each}
			</div>
		</article>
	{/if}
{/snippet}

<svelte:head>
	<title>{noturnoAgora.length ? `${noturnoAgora.map((t) => t.nome).join(', ')} · ` : ''}Plantão · Projetos NOC</title>
</svelte:head>

<div class="h-screen overflow-hidden bg-paper text-ink flex flex-col {tv.controlesVisiveis ? '' : 'cursor-none'}">
	{#if erro === 'nao_autorizada' && !dados}
		<div class="flex-1 grid place-items-center p-8">
			<div class="max-w-[34rem] text-center space-y-4">
				<Tv class="size-14 mx-auto text-ink-3" strokeWidth={1.5} />
				<h1 class="text-[2rem] font-bold leading-tight">Tela não autorizada</h1>
				<p class="text-[1.125rem] text-ink-2">
					Para usar a TV do plantão sem login, um administrador cadastra esta tela em
					<strong class="text-ink">Operadores → Telas de TV</strong> e abre aqui o link do plantão.
				</p>
				<a href="/" class="btn btn-secondary btn-lg">Entrar no sistema</a>
			</div>
		</div>
	{:else if erro === 'desativado'}
		<div class="flex-1 grid place-items-center p-8">
			<div class="max-w-[34rem] text-center space-y-4">
				<Tv class="size-14 mx-auto text-ink-3" strokeWidth={1.5} />
				<h1 class="text-[2rem] font-bold leading-tight">TV do plantão desativada</h1>
				<p class="text-[1.125rem] text-ink-2">O modo TV ou o plantão está desligado para o setor desta tela.</p>
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
			<div class="flex items-center gap-3 min-w-0 flex-1">
				<img src="/logo.svg" alt="" class="size-12 shrink-0" />
				<div class="min-w-0">
					<div class="text-[1.5rem] font-bold leading-tight tracking-[-0.01em]">Plantão</div>
					<div class="text-[0.95rem] text-ink-3 truncate">{dados.setor}, {dados.tela}</div>
				</div>
			</div>
			<div class="text-right shrink-0">
				<div class="tv-display text-[3.5rem] leading-none">
					{hora}<span class="text-[1.75rem] text-ink-3 font-semibold">:{segundos}</span>
				</div>
				<div class="mt-1 text-[1.05rem] text-ink-2 first-letter:uppercase">{data}</div>
			</div>
		</header>

		<main class="flex-1 min-h-0 grid grid-cols-[minmax(0,2.2fr)_minmax(0,1fr)] gap-5 px-6">
			<!-- Destaque: o próximo plantão de domingo (externos por cidade) e o interno -->
			<div class="min-h-0 flex flex-col gap-4">
				<section class="flex-1 min-h-0 flex flex-col gap-3" aria-label="Plantão de domingo">
					<h2 class="bloco">
						{ehDomingo(hoje) ? 'Plantão de domingo, hoje' : `Plantão de domingo, ${diaSemana(domingo).replace(/^\S+\s/, '')}`}
					</h2>
					<div class="flex-1 min-h-0 grid grid-cols-3 gap-4">
						{#each externos as c (c.id)}
							{@render cartao(c.nome, c.domingo)}
						{/each}
					</div>
				</section>
				<section class="flex-1 min-h-0 flex flex-col gap-3" aria-label="Plantão interno">
					<h2 class="bloco">
						Plantão interno, {diaInterno === hoje ? 'hoje' : diaComFeriado(diaInterno, feriados)}
					</h2>
					<div class="flex-1 min-h-0 grid grid-cols-2 gap-4">
						{#each internoPorPeriodo as p (p.id)}
							{@render cartao(p.nome, p.turnos)}
						{/each}
					</div>
				</section>
			</div>

			<!-- Lateral: noturno de São Gabriel, folga e próximas trocas -->
			<aside class="panel min-h-0 flex flex-col overflow-hidden divide-y divide-line">
				<section class="px-5 py-4" aria-label="Plantão noturno hoje">
					<h2 class="bloco">Noturno hoje, São Gabriel</h2>
					{#if noturnoAgora.length === 0}
						<p class="mt-2 text-[1.3rem] font-bold text-warn">Ninguém escalado</p>
					{:else}
						<ul class="mt-2 space-y-1.5">
							{#each noturnoAgora as t (t.id)}
								<li class="flex items-center gap-3 min-w-0">
									<span class="dot !size-3" style="background-color: {corDaPessoa(t.nome)};"></span>
									<span class="text-[1.3rem] font-bold truncate">{t.nome}</span>
									<span class="ml-auto shrink-0 text-[1rem] text-ink-3 tabular">{t.fim === hoje ? 'última noite' : `até ${diaSemana(t.fim)}`}</span>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				{#if folgaAgora.length > 0}
					<section class="px-5 py-4" aria-label="De folga">
						<h2 class="bloco">De folga</h2>
						<ul class="mt-2 space-y-1.5">
							{#each folgaAgora as t (t.id)}
								<li class="flex items-center gap-3 min-w-0">
									<span class="dot !size-3" style="background-color: {corDaPessoa(t.nome)};"></span>
									<span class="text-[1.3rem] font-bold text-ink-2 truncate">{t.nome}</span>
									<span class="ml-auto shrink-0 text-[1rem] text-ink-3 tabular">{t.folga_fim === hoje ? 'volta amanhã' : `até ${diaSemana(t.folga_fim!)}`}</span>
								</li>
							{/each}
						</ul>
					</section>
				{/if}

				<section class="px-5 py-4 min-h-0 flex-1 flex flex-col" aria-label="Próximas trocas do noturno">
					<h2 class="bloco">Próximas trocas do noturno</h2>
					{#if proximos.length === 0}
						<p class="mt-2 text-[1.1rem] text-ink-3">Nenhuma troca nas próximas semanas.</p>
					{:else}
						<ul class="tv-lista mt-2">
							{#each proximos as t (t.id)}
								<li class="py-1.5 flex items-center gap-3 min-w-0">
									<span class="dot !size-3" style="background-color: {corDaPessoa(t.nome)};"></span>
									<div class="min-w-0 flex-1">
										<div class="text-[1.25rem] font-bold leading-tight truncate">{t.nome}</div>
										<div class="text-[0.95rem] text-ink-3 truncate">{diaSemana(t.inicio)}</div>
									</div>
									<span class="tag !h-7 !text-[0.95rem] shrink-0">{quandoComeca(t.inicio, hoje)}</span>
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			</aside>
		</main>

		<!-- Faixa das próximas duas semanas: por cidade, noturno em cheio e domingo tracejado; depois o interno -->
		<section class="shrink-0 px-6 pt-4 pb-2" aria-label="Escala das próximas duas semanas">
			<div class="panel px-3 py-2.5">
				<div class="grid gap-x-1.5 gap-y-1" style="grid-template-columns: 8rem repeat({DIAS_FAIXA}, minmax(0, 1fr));">
					<span></span>
					{#each faixa as d (d.dia)}
						<div
							class="text-center py-1 rounded-md {d.dia === hoje ? 'bg-accent-soft text-accent' : d.feriado ? 'text-warn' : d.fimDeSemana ? 'text-ink-3' : 'text-ink-2'}"
							title={d.feriado ?? undefined}
						>
							<div class="text-[0.85rem] font-semibold">{d.dia === hoje ? 'hoje' : d.feriado ? 'feriado' : d.semana}</div>
							<div class="text-[1.25rem] font-bold tabular leading-tight">{d.numero}</div>
						</div>
					{/each}

					{#each CIDADES as c, i (c.id)}
						<span class="self-center pl-1 text-[0.95rem] font-semibold text-ink-3 truncate">{c.nome}</span>
						{#each faixa as d (d.dia)}
							{@const cel = d.cidades[i]}
							{@const falta = (cel.temNoturno && cel.noturno.length === 0) || (d.domingo && cel.domingo.length === 0)}
							<div class="celula {falta ? 'bg-warn-soft' : ''} {d.dia === hoje ? 'ring-2 ring-accent' : ''}">
								{#each cel.noturno as t (t.id)}
									<span class="chip text-white" style="background-color: {corDaPessoa(t.nome)}" title="{t.nome}, noturno">{primeiroNome(t.nome)}</span>
								{/each}
								{#each cel.domingo as t (t.id)}
									<span class="chip border border-dashed" style="border-color: {corDaPessoa(t.nome)}" title="{t.nome}, domingo">{primeiroNome(t.nome)}</span>
								{/each}
								{#if falta && cel.noturno.length + cel.domingo.length === 0}
									<span class="text-center text-[0.85rem] text-warn">—</span>
								{/if}
							</div>
						{/each}
					{/each}

					{#each PERIODOS as p, i (p.id)}
						<span class="self-center pl-1 text-[0.95rem] font-semibold text-ink-3 truncate">Interno {p.nome.toLowerCase()}</span>
						{#each faixa as d (d.dia)}
							{@const turnos = d.interno[i]}
							<div class="celula {d.temInterno && turnos.length === 0 ? 'bg-warn-soft' : ''} {d.dia === hoje ? 'ring-2 ring-accent' : ''}">
								{#each turnos as t (t.id)}
									<span class="chip text-white" style="background-color: {corDaPessoa(t.nome)}" title="{t.nome}, interno {p.nome.toLowerCase()}">{primeiroNome(t.nome)}</span>
								{:else}
									{#if d.temInterno}<span class="text-center text-[0.85rem] text-warn">—</span>{/if}
								{/each}
							</div>
						{/each}
					{/each}

					{#if faixa.some((d) => d.folga.length > 0)}
						<span class="self-center pl-1 text-[0.95rem] font-semibold text-ink-3">Folga</span>
						{#each faixa as d (d.dia)}
							<div class="celula !min-h-[2.1rem] {d.dia === hoje ? 'ring-2 ring-accent' : ''}">
								{#each d.folga as t (t.id)}
									<span class="flex items-center gap-1.5 min-w-0 text-[0.85rem] text-ink-2 leading-snug" title={t.nome}>
										<span class="dot !size-2" style="background-color: {corDaPessoa(t.nome)};"></span>
										<span class="truncate">{primeiroNome(t.nome)}</span>
									</span>
								{/each}
							</div>
						{/each}
					{/if}
				</div>
			</div>
		</section>

		<footer class="flex items-center justify-between gap-4 px-6 pb-3 text-[0.9rem] text-ink-3 tabular">
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
					<span class="flex items-center gap-2"><span class="inline-block w-6 h-3.5 rounded bg-ink-3"></span> noturno</span>
					<span class="flex items-center gap-2"><span class="inline-block w-6 h-3.5 rounded border border-dashed border-ink-3"></span> domingo</span>
					<span>Interno aos domingos e feriados</span>
				</span>
			{/if}
		</footer>
	{/if}

	<div
		class="fixed bottom-4 right-4 flex gap-2 transition-opacity duration-300 {tv.controlesVisiveis ? 'opacity-100' : 'opacity-0 pointer-events-none'}"
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
	.cartao {
		container-type: inline-size;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
		padding: 1rem 1.5rem 1.25rem;
		border-radius: 1rem;
	}
	.bloco {
		font-size: 1.2rem;
		font-weight: 700;
		color: var(--ink-2);
	}
	.celula {
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: 2px;
		min-height: 2.6rem;
		padding: 0.2rem 0.35rem;
		overflow: hidden;
		border-radius: 0.5rem;
	}
	.chip {
		display: block;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		border-radius: 0.25rem;
		padding: 0 0.4rem;
		font-size: 0.9rem;
		font-weight: 600;
		line-height: 1.4;
	}
</style>
