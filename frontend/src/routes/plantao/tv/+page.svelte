<script lang="ts">
	import { onMount } from 'svelte';
	import Avatar from '$lib/components/Avatar.svelte';
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
		periodo,
		quandoComeca,
		quantoFalta,
		nomeCidade,
		type Turno
	} from '$lib/plantao';
	import { Maximize, Minimize, ArrowLeft, ShieldCheck, ShieldAlert, WifiOff, Tv, Coffee, Moon, Sun, MapPin } from 'lucide-svelte';

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

	function horaCurta(ms: number): string {
		return new Date(ms + desvio).toLocaleTimeString('pt-BR', { timeZone: FUSO, hour: '2-digit', minute: '2-digit' });
	}
</script>

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
		<header class="flex items-center gap-6 px-8 pt-6 pb-5">
			<div class="flex items-center gap-3 min-w-0 flex-1">
				<span class="grid place-items-center size-10 rounded-[0.6rem] bg-accent text-on-accent shrink-0">
					<ShieldCheck class="size-6" strokeWidth={2.4} />
				</span>
				<div class="min-w-0">
					<div class="text-[1.5rem] font-bold leading-tight tracking-[-0.01em]">Plantão</div>
					<div class="text-[0.95rem] text-ink-3 truncate">{dados.setor} · {dados.tela}</div>
				</div>
			</div>
			<div class="text-right">
				<div class="text-[3.5rem] font-bold leading-none tabular tracking-[-0.02em]">
					{hora}<span class="text-[1.75rem] text-ink-3 font-semibold">:{segundos}</span>
				</div>
				<div class="mt-1 text-[1.05rem] text-ink-2 first-letter:uppercase">{data}</div>
			</div>
		</header>

		<main class="flex-1 min-h-0 grid grid-cols-[minmax(0,2fr)_minmax(0,1fr)] gap-6 px-8">
			<div class="min-h-0 flex flex-col gap-5 overflow-hidden">
				<!-- Destaque: quem está no plantão noturno hoje, por cidade -->
				<section class="flex-1 min-h-0 flex flex-col gap-3" aria-label="Plantão noturno hoje">
					<h2 class="titulo-bloco flex items-center gap-2"><Moon class="size-5" /> Plantão noturno hoje</h2>
					<div class="flex-1 min-h-0 grid grid-cols-3 gap-4">
						{#each externos as c (c.id)}
							{#if c.noturno.length === 0}
								<article class="rounded-2xl border-2 border-warn bg-warn-soft px-6 py-5 min-w-0 flex flex-col gap-3 overflow-hidden">
									<p class="flex items-center gap-2 text-[1.35rem] font-bold leading-tight">
										<MapPin class="size-5 text-warn shrink-0" />
										<span class="truncate">{c.nome}</span>
									</p>
									<p class="flex-1 flex items-center justify-center gap-3 text-[2.4rem] font-bold leading-tight text-warn">
										<ShieldAlert class="size-10 shrink-0" strokeWidth={1.8} /> Sem técnico
									</p>
								</article>
							{:else}
								<article
									class="rounded-2xl bg-surface border border-line px-6 py-5 min-w-0 flex flex-col gap-4 overflow-hidden"
									style="border-top: 0.5rem solid {corDaPessoa(c.noturno[0].nome)};"
								>
									<p class="flex items-center gap-2 text-[1.35rem] font-bold leading-tight">
										<MapPin class="size-5 text-accent shrink-0" />
										<span class="truncate">{c.nome}</span>
									</p>
									<div class="flex-1 flex flex-col justify-center gap-5 min-w-0">
										{#each c.noturno as t (t.id)}
											<div class="flex flex-col items-center text-center gap-3 min-w-0">
												<Avatar id={t.id} nome={t.nome} cor={corDaPessoa(t.nome)} class="{c.noturno.length > 1 ? 'size-16 text-[1.4rem]' : 'size-28 text-[2.4rem]'} shrink-0" />
												<div class="min-w-0 max-w-full">
													<p class="{c.noturno.length > 1 ? 'text-[2.2rem]' : 'text-[3rem]'} font-bold leading-[1.05] tracking-[-0.02em] truncate">{t.nome}</p>
													<p class="mt-1.5 text-[1.25rem] text-ink-2 tabular truncate">
														{t.fim === hoje ? 'Última noite' : `Até ${diaSemana(t.fim)} · ${quantoFalta(t.fim, hoje)}`}
													</p>
													{#if t.observacao}
														<p class="mt-1 text-[1.1rem] text-ink-3 truncate">{t.observacao}</p>
													{/if}
												</div>
											</div>
										{/each}
									</div>
								</article>
							{/if}
						{/each}
					</div>
				</section>

				<!-- Secundário: plantão interno e de domingo -->
				<section class="rounded-2xl bg-surface border border-line px-5 py-3.5 flex flex-col gap-2.5" aria-label="Plantão interno e de domingo">
					<div class="flex items-baseline gap-3 min-w-0 text-[1.15rem]">
						<span class="flex items-center gap-2 w-[13rem] shrink-0 font-semibold text-ink-3"><ShieldCheck class="size-4.5" /> Interno</span>
						<span class="text-ink-3 tabular shrink-0">{diaInterno === hoje ? 'hoje' : diaComFeriado(diaInterno, feriados)}</span>
						<span class="min-w-0 truncate font-semibold {internoDoDia.length ? 'text-ink' : 'text-warn'}">
							{internoDoDia.length ? internoDoDia.map((t) => t.nome).join(', ') : 'ninguém escalado'}
						</span>
					</div>
					<div class="flex items-baseline gap-3 min-w-0 text-[1.15rem]">
						<span class="flex items-center gap-2 w-[13rem] shrink-0 font-semibold text-ink-3"><Sun class="size-4.5" /> Domingo</span>
						<span class="text-ink-3 tabular shrink-0">{ehDomingo(hoje) ? 'hoje' : diaSemana(domingo)}</span>
						<span class="min-w-0 truncate">
							{#each externos as c, i (c.id)}
								{#if i > 0}<span class="text-ink-3">{' · '}</span>{/if}
								<span class="text-ink-3">{c.nome}</span>
								<span class="font-semibold {c.domingo.length ? 'text-ink' : 'text-warn'}">{c.domingo.length ? c.domingo.map((t) => t.nome).join(', ') : '—'}</span>
							{/each}
						</span>
					</div>
				</section>

				{#if folgaAgora.length > 0}
					<div class="flex items-center gap-4 flex-wrap">
						<span class="flex items-center gap-2 text-[1.15rem] font-semibold text-ink-2">
							<Coffee class="size-5" /> De folga
						</span>
						{#each folgaAgora as t (t.id)}
							<span class="inline-flex items-center gap-3 rounded-full bg-surface border border-line px-4 py-1.5">
								<span class="size-3 rounded-full" style="background-color: {corDaPessoa(t.nome)};"></span>
								<span class="text-[1.35rem] font-semibold text-ink-2">{t.nome}</span>
								<span class="text-[1rem] text-ink-3 tabular">{t.folga_fim === hoje ? 'hoje' : `até ${diaSemana(t.folga_fim!)}`}</span>
							</span>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Próximas trocas do noturno -->
			<aside class="min-h-0 rounded-2xl bg-surface border border-line px-5 py-4 flex flex-col overflow-hidden" aria-label="Próximas trocas do noturno">
				<h2 class="titulo-bloco">Próximas trocas do noturno</h2>
				{#if proximos.length === 0}
					<p class="mt-2 text-[1.05rem] text-ink-3">Nenhuma troca nas próximas semanas.</p>
				{:else}
					<ul class="mt-3 space-y-3 overflow-hidden">
						{#each proximos as t (t.id)}
							<li class="flex items-center gap-3 min-w-0">
								<span class="w-1.5 self-stretch rounded-full shrink-0" style="background-color: {corDaPessoa(t.nome)};"></span>
								<div class="min-w-0 flex-1">
									<div class="text-[1.25rem] font-semibold leading-tight truncate">{t.nome}</div>
									<div class="text-[0.95rem] text-ink-3 tabular truncate">{nomeCidade(t.cidade)} · {periodo(t)}</div>
								</div>
								<span class="text-[1rem] font-semibold text-ink-2 tabular shrink-0">{quandoComeca(t.inicio, hoje)}</span>
							</li>
						{/each}
					</ul>
				{/if}
			</aside>
		</main>

		<!-- Faixa das próximas duas semanas: por cidade, noturno (cheio) e domingo (tracejado); depois o interno -->
		<section class="px-8 pt-5 pb-3" aria-label="Escala das próximas duas semanas">
			<div class="grid gap-1.5" style="grid-template-columns: 8.5rem repeat({DIAS_FAIXA}, minmax(0, 1fr));">
				<span></span>
				{#each faixa as d (d.dia)}
					<div
						class="text-center pb-1 {d.dia === hoje ? 'text-accent' : d.feriado ? 'text-warn' : d.fimDeSemana ? 'text-ink-3' : 'text-ink-2'}"
						title={d.feriado ?? undefined}
					>
						<div class="text-[0.85rem] font-semibold uppercase">{d.feriado ? 'Feriado' : d.semana}</div>
						<div class="text-[1.25rem] font-bold tabular leading-tight">{d.numero}</div>
					</div>
				{/each}

				{#each CIDADES as c, i (c.id)}
					<span class="self-center text-[0.95rem] font-semibold text-ink-3 truncate">{c.nome}</span>
					{#each faixa as d (d.dia)}
						{@const cel = d.cidades[i]}
						{@const falta = cel.noturno.length === 0 || (d.domingo && cel.domingo.length === 0)}
						<div
							class="min-h-[2.75rem] rounded-lg px-1.5 py-1 flex flex-col justify-center gap-0.5 overflow-hidden {d.dia === hoje ? 'ring-2 ring-accent' : ''} {falta ? 'bg-warn-soft' : ''}"
						>
							{#each cel.noturno as t (t.id)}
								<span class="block truncate rounded px-1.5 text-[0.9rem] font-semibold leading-snug text-white" style="background-color: {corDaPessoa(t.nome)}" title="{t.nome} · noturno"
									>{t.nome.split(' ')[0]}</span
								>
							{/each}
							{#each cel.domingo as t (t.id)}
								<span class="block truncate rounded px-1.5 text-[0.9rem] font-semibold leading-snug border border-dashed" style="border-color: {corDaPessoa(t.nome)}" title="{t.nome} · domingo"
									>{t.nome.split(' ')[0]}</span
								>
							{/each}
							{#if falta && cel.noturno.length + cel.domingo.length === 0}
								<span class="text-center text-[0.85rem] text-warn">—</span>
							{/if}
						</div>
					{/each}
				{/each}

				<span class="self-center text-[0.95rem] font-semibold text-ink-3">Interno</span>
				{#each faixa as d (d.dia)}
					<div
						class="min-h-[2.75rem] rounded-lg px-1.5 py-1 flex flex-col justify-center gap-0.5 overflow-hidden {d.dia === hoje ? 'ring-2 ring-accent' : ''} {d.temInterno && d.interno.length === 0 ? 'bg-warn-soft' : ''}"
					>
						{#each d.interno as t (t.id)}
							<span class="block truncate rounded px-1.5 text-[0.9rem] font-semibold leading-snug text-white" style="background-color: {corDaPessoa(t.nome)}" title={t.nome}
								>{t.nome.split(' ')[0]}</span
							>
						{:else}
							{#if d.temInterno}<span class="text-center text-[0.85rem] text-warn">—</span>{/if}
						{/each}
					</div>
				{/each}

				{#if faixa.some((d) => d.folga.length > 0)}
					<span class="self-center text-[0.95rem] font-semibold text-ink-3">Folga</span>
					{#each faixa as d (d.dia)}
						<div class="min-h-[2.25rem] rounded-lg px-1.5 py-1 flex flex-col justify-center gap-0.5 overflow-hidden {d.dia === hoje ? 'ring-2 ring-accent' : ''}">
							{#each d.folga as t (t.id)}
								<span class="flex items-center gap-1.5 min-w-0 text-[0.85rem] text-ink-2 leading-snug" title={t.nome}>
									<span class="size-2 rounded-full shrink-0" style="background-color: {corDaPessoa(t.nome)};"></span>
									<span class="truncate">{t.nome.split(' ')[0]}</span>
								</span>
							{/each}
						</div>
					{/each}
				{/if}
			</div>
		</section>

		<footer class="flex items-center justify-between gap-4 px-8 pb-4 text-[0.9rem] text-ink-3 tabular">
			{#if dadosVelhos}
				<span class="flex items-center gap-2 font-semibold text-warn">
					<WifiOff class="size-4" /> Sem conexão com o servidor desde {horaCurta(ultimoSucesso)}. A escala pode estar desatualizada.
				</span>
			{:else}
				<span>Atualizado às {horaCurta(ultimoSucesso)} · atualiza a cada minuto · interno aos domingos e feriados · nas cidades, noturno em cheio e domingo tracejado</span>
			{/if}
			{#if erro === 'nao_autorizada'}
				<span class="font-semibold text-danger">{chave ? 'A chave desta tela foi revogada.' : 'A sessão foi encerrada.'}</span>
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
	.titulo-bloco {
		font-size: 1rem;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--ink-3);
	}
</style>
