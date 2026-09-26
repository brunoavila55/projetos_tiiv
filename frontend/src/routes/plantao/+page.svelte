<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import EscalaModal from '$lib/components/plantao/EscalaModal.svelte';
	import { Calendar, DayGrid, Interaction, List } from '@event-calendar/core';
	import { Plus, CalendarDays, X, Trash2, Edit3, Tv, ShieldAlert, ArrowRightLeft, Users, Coffee, Moon, Sun, MapPin } from 'lucide-svelte';
	import {
		CIDADES,
		ESCALAS,
		rotuloEscala,
		rotuloCurto,
		daEscala,
		ehDomingo,
		proximoDomingo,
		proximoDiaInterno,
		diaComFeriado,
		diasFeriado,
		type Feriado,
		diaMes,
		corDaPessoa,
		mesmaPessoa,
		cobre,
		deFolga,
		periodoFolga,
		diaLocal,
		paraDia,
		somarDias,
		diasEntre,
		diaSemana,
		periodo,
		quandoComeca,
		quantoFalta,
		type Cidade,
		PERIODOS,
		type Periodo,
		type Escala,
		type TipoPlantao,
		type Turno
	} from '$lib/plantao';

	// Janela da dash: hoje e os próximos 29 dias
	const DIAS_DASH = 30;
	const PREFIXO = 'plantao:';
	const PREFIXO_FOLGA = 'folga:';

	const ehAdmin = $derived(auth.ehAdmin);
	const aba = $derived(page.url.searchParams.get('aba') === 'calendario' ? 'calendario' : 'resumo');

	function trocarAba(nova: 'resumo' | 'calendario') {
		const url = new URL(page.url);
		if (nova === 'resumo') url.searchParams.delete('aba');
		else url.searchParams.set('aba', nova);
		goto(url, { replaceState: true, keepFocus: true, noScroll: true });
	}

	// ---------- Resumo ----------
	let hoje = $state(paraDia(new Date()));
	let proximos = $state<Turno[]>([]);
	// Feriados da janela: o plantão interno é aos domingos e feriados
	let feriados = $state(diasFeriado([]));
	let carregandoResumo = $state(true);

	async function carregarResumo() {
		hoje = paraDia(new Date());
		const fim = paraDia(somarDias(diaLocal(hoje), DIAS_DASH - 1));
		try {
			const [lista, fer] = await Promise.all([
				apiFetch<Turno[]>(`/api/plantoes?inicio=${hoje}&fim=${fim}`),
				apiFetch<Feriado[]>(`/api/plantoes/feriados?inicio=${hoje}&fim=${fim}`)
			]);
			proximos = lista;
			feriados = diasFeriado(fer);
		} catch {
			// apiFetch já avisou
		} finally {
			carregandoResumo = false;
		}
	}

	const dias = $derived(Array.from({ length: DIAS_DASH }, (_, i) => paraDia(somarDias(diaLocal(hoje), i))));
	// Plantão interno: o de hoje, se hoje for domingo ou feriado, ou o do próximo dia desses
	const diaInterno = $derived(proximoDiaInterno(hoje, feriados));
	const internoDoDia = $derived(proximos.filter((t) => t.tipo === 'interno' && cobre(t, diaInterno)));
	// Manhã e tarde, cada um com quem está escalado (pode ficar vazio)
	const internoPorPeriodo = $derived(PERIODOS.map((p) => ({ ...p, turnos: internoDoDia.filter((t) => t.periodo === p.id) })));
	// Técnicos externos: o noturno de hoje e o plantão do domingo (hoje ou o próximo)
	const domingo = $derived(proximoDomingo(hoje));
	const externos = $derived(
		CIDADES.map((c) => ({
			...c,
			noturno: proximos.filter((t) => daEscala(t, 'noturno', c.id) && cobre(t, hoje)),
			domingo: proximos.filter((t) => daEscala(t, 'domingo', c.id) && cobre(t, domingo))
		}))
	);
	const folgaHoje = $derived(proximos.filter((t) => deFolga(t, hoje)));
	// A listagem também traz turnos já passados que aparecem só pela folga
	const vigentes = $derived(proximos.filter((t) => t.fim >= hoje));
	const aComecar = $derived(proximos.filter((t) => t.inicio > hoje));
	const proximaTroca = $derived(aComecar[0] ?? null);
	const meuProximo = $derived(auth.user ? (vigentes.find((t) => mesmaPessoa(t.nome, auth.user!.nome)) ?? null) : null);

	// Dias sem ninguém em cada escala, agrupados em trechos seguidos (no de
	// domingo, domingos seguidos); dias = quantos dias da escala ficam sem ninguém
	interface Buraco {
		escala: Escala;
		inicio: string;
		fim: string;
		dias: number;
	}
	const buracos = $derived.by(() => {
		const trechos: Buraco[] = [];
		for (const e of ESCALAS) {
			let aberto: Buraco | null = null;
			for (const d of dias) {
				if (!e.precisa(d, feriados)) continue;
				if (proximos.some((t) => daEscala(t, e.tipo, e.cidade, e.periodo) && cobre(t, d))) {
					aberto = null;
				} else if (aberto) {
					aberto.fim = d;
					aberto.dias++;
				} else {
					aberto = { escala: e, inicio: d, fim: d, dias: 1 };
					trechos.push(aberto);
				}
			}
		}
		return trechos.sort((a, b) => a.inicio.localeCompare(b.inicio));
	});
	const diasSemCobertura = $derived(buracos.reduce((n, b) => n + b.dias, 0));
	const escalasDescobertas = $derived(new Set(buracos.map((b) => b.escala.rotulo)).size);

	// O noturno é todo dia: um período. O domingo e o interno contam os dias soltos
	function textoBuraco(b: Buraco): string {
		if (b.escala.tipo === 'noturno') return periodo(b);
		if (b.dias === 1) return diaComFeriado(b.inicio, feriados);
		const dias = b.escala.tipo === 'domingo' ? 'domingos' : 'domingos e feriados';
		return `${b.dias} ${dias}, ${diaMes(diaLocal(b.inicio))} a ${diaMes(diaLocal(b.fim))}`;
	}

	// Dias de plantão por pessoa na janela, somando as escalas (só a parte do turno dentro dela)
	const carga = $derived.by(() => {
		const ultimoDia = dias[dias.length - 1];
		const porPessoa: { nome: string; dias: number }[] = [];
		for (const t of vigentes) {
			const ini = t.inicio < hoje ? hoje : t.inicio;
			const fim = t.fim > ultimoDia ? ultimoDia : t.fim;
			const n = diasEntre(ini, fim) + 1;
			const p = porPessoa.find((x) => mesmaPessoa(x.nome, t.nome));
			if (p) p.dias += n;
			else porPessoa.push({ nome: t.nome, dias: n });
		}
		return porPessoa.sort((a, b) => b.dias - a.dias || a.nome.localeCompare(b.nome, 'pt-BR'));
	});
	const maiorCarga = $derived(Math.max(1, ...carga.map((c) => c.dias)));

	// ---------- Calendário ----------
	let faixa = $state<{ inicio: Date; fim: Date } | null>(null);
	let escala = $state<Turno[]>([]);
	let feriadosCal = $state<Feriado[]>([]);
	let carregandoCal = $state(false);
	// Filtro do calendário: todas as escalas, só o interno ou uma cidade
	let filtro = $state<'todas' | 'interno' | Cidade>('todas');

	const plugins = [DayGrid, Interaction, List];
	let options = $state({
		view: 'dayGridMonth',
		headerToolbar: { start: 'prev,next today', center: 'title', end: 'dayGridMonth,listMonth' },
		buttonText: { today: 'Hoje', dayGridMonth: 'Mês', listMonth: 'Lista' },
		locale: 'pt-br',
		firstDay: 0 as const,
		selectable: false,
		editable: false,
		events: [] as any[],
		// Interno, noturno e domingo, nessa ordem, em cada dia
		eventOrder: (a: any, b: any) => (a.extendedProps?.ordem ?? 0) - (b.extendedProps?.ordem ?? 0) || a.start - b.start,
		datesSet: (info: any) => {
			faixa = { inicio: info.start, fim: info.end };
		},
		select: (info: any) => {
			// No dayGrid o fim da seleção é exclusivo (dia seguinte ao último marcado)
			novo = { inicio: paraDia(info.start), fim: paraDia(somarDias(info.end, -1)) };
		},
		eventClick: (info: any) => {
			const t = escala.find((x) => PREFIXO + x.id === info.event.id || PREFIXO_FOLGA + x.id === info.event.id);
			if (t) selecionado = t;
		}
	});

	// Só admin monta a escala clicando/arrastando nos dias
	$effect(() => {
		options.selectable = ehAdmin;
	});

	async function carregarCalendario() {
		if (!faixa) return;
		carregandoCal = true;
		// O fim da faixa é exclusivo; a escala usa dias inclusivos
		const params = new URLSearchParams({ inicio: paraDia(faixa.inicio), fim: paraDia(somarDias(faixa.fim, -1)) });
		try {
			[escala, feriadosCal] = await Promise.all([
				apiFetch<Turno[]>(`/api/plantoes?${params}`),
				apiFetch<Feriado[]>(`/api/plantoes/feriados?${params}`)
			]);
		} catch {
			// apiFetch já avisou
		} finally {
			carregandoCal = false;
		}
	}

	const ORDEM_TIPO: Record<TipoPlantao, number> = { interno: 0, noturno: 1, domingo: 2 };

	function passaFiltro(t: Turno): boolean {
		if (filtro === 'todas') return true;
		if (filtro === 'interno') return t.tipo === 'interno';
		return t.cidade === filtro;
	}

	// Interno em faixa listrada, noturno cheio e domingo tracejado, na cor da pessoa
	$effect(() => {
		const visiveis = escala.filter(passaFiltro);
		const folgas = visiveis
			.filter((t) => t.folga_inicio && t.folga_fim)
			.map((t) => ({
				id: PREFIXO_FOLGA + t.id,
				title: `Folga · ${t.nome}`,
				start: t.folga_inicio!,
				end: paraDia(somarDias(diaLocal(t.folga_fim!), 1)),
				allDay: true,
				classNames: ['ec-folga'],
				styles: [`--cor-escala: ${corDaPessoa(t.nome)}`],
				extendedProps: { ordem: 3 }
			}));
		const turnos = visiveis.map((t) => ({
			id: PREFIXO + t.id,
			title: `${t.nome} · ${rotuloCurto(t)}`,
			start: t.inicio,
			end: paraDia(somarDias(diaLocal(t.fim), 1)),
			allDay: true,
			classNames: [`ec-${t.tipo}`],
			backgroundColor: t.tipo === 'domingo' ? 'transparent' : corDaPessoa(t.nome),
			styles: [`--cor-escala: ${corDaPessoa(t.nome)}`],
			// No mesmo dia, o interno da manhã vem antes do da tarde
			extendedProps: { ordem: ORDEM_TIPO[t.tipo] + (t.periodo === 'tarde' ? 0.5 : 0) }
		}));
		// Feriado no topo do dia, para ver onde o interno precisa de alguém
		const diasDeFeriado = feriadosCal.map((f) => ({
			id: 'feriado:' + f.dia,
			title: `Feriado · ${f.nome}`,
			start: f.dia,
			end: paraDia(somarDias(diaLocal(f.dia), 1)),
			allDay: true,
			classNames: ['ec-feriado'],
			extendedProps: { ordem: -1 }
		}));
		untrack(() => (options.events = [...diasDeFeriado, ...turnos, ...folgas]));
	});

	$effect(() => {
		if (aba === 'calendario' && faixa) untrack(carregarCalendario);
	});
	$effect(() => {
		if (aba === 'resumo') untrack(carregarResumo);
	});

	// ---------- Turnos: detalhes, criar, editar, excluir ----------
	let selecionado = $state<Turno | null>(null);
	let novo = $state<{ inicio?: string; fim?: string; tipo?: TipoPlantao; cidade?: Cidade | null; periodo?: Periodo | null } | null>(null);
	let editando = $state<Turno | null>(null);

	function aposSalvar() {
		novo = null;
		editando = null;
		recarregar();
	}

	function recarregar() {
		if (aba === 'calendario') carregarCalendario();
		else carregarResumo();
	}

	async function excluir(t: Turno) {
		if (!confirm(`Tirar ${t.nome} da escala de ${periodo(t)}?`)) return;
		try {
			await apiFetch(`/api/plantoes/${t.id}`, { method: 'DELETE' });
			selecionado = null;
			recarregar();
		} catch {
			// apiFetch já mostrou o erro
		}
	}
</script>

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Plantão</h1>
			<p class="page-sub">Plantão interno e dos técnicos externos por cidade, e a escala das próximas semanas.</p>
		</div>

		<div class="flex flex-wrap lg:flex-nowrap lg:shrink-0 items-center gap-2">
			<div class="segmented shrink-0" role="group" aria-label="Visão">
				<button aria-pressed={aba === 'resumo'} onclick={() => trocarAba('resumo')}>Resumo</button>
				<button aria-pressed={aba === 'calendario'} onclick={() => trocarAba('calendario')}>Calendário</button>
			</div>
			{#if auth.temModulo('tv')}
				<a href="/plantao/tv" class="btn btn-ghost" title="Abrir a escala em tela cheia para o telão">
					<Tv class="size-4" />
					<span class="hidden sm:inline">Modo TV</span>
				</a>
			{/if}
			{#if ehAdmin}
				<button onclick={() => (novo = {})} class="btn btn-primary">
					<Plus class="size-4" />
					<span class="hidden sm:inline">Montar escala</span>
				</button>
			{/if}
		</div>
	</div>

	{#if aba === 'resumo'}
		{#if carregandoResumo}
			<div class="flex items-center gap-3 text-sm text-ink-3"><div class="spinner size-5"></div> Carregando…</div>
		{:else}
			<!-- Agora -->
			<section aria-label="Plantão interno" class="space-y-3">
				<h2 class="text-[13px] font-bold uppercase tracking-wide text-ink-3">Plantão interno</h2>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					{#each internoPorPeriodo as p (p.id)}
						{#if p.turnos.length === 0}
							<div class="flex items-start gap-3 px-4 py-3.5 rounded-xl border border-warn/40 bg-warn-soft">
								<ShieldAlert class="size-5 mt-0.5 text-warn shrink-0" />
								<div class="text-sm flex-1">
									<p class="font-semibold text-ink">
										{p.nome}: {diaInterno === hoje ? 'ninguém no plantão interno hoje' : `ninguém no plantão interno de ${diaComFeriado(diaInterno, feriados)}`}
									</p>
									<p class="text-ink-2">O plantão interno é aos domingos e feriados, de manhã e à tarde.</p>
								</div>
								{#if ehAdmin}
									<button onclick={() => (novo = { tipo: 'interno', periodo: p.id, inicio: diaInterno, fim: diaInterno })} class="btn btn-secondary btn-sm">Escalar</button>
								{/if}
							</div>
						{:else}
							{#each p.turnos as t (t.id)}
								<button
									onclick={() => (selecionado = t)}
									class="panel text-left p-5 flex items-center gap-4 min-w-0 cursor-pointer hover:border-line-strong transition-colors"
									style="border-left: 4px solid {corDaPessoa(t.nome)};"
								>
									<Avatar id={t.id} nome={t.nome} cor={corDaPessoa(t.nome)} class="size-14 text-lg shrink-0" />
									<div class="min-w-0">
										<p class="text-[13px] font-semibold text-ink-3 truncate">
											{p.nome}, {diaInterno === hoje ? 'hoje' : diaComFeriado(diaInterno, feriados)}
										</p>
										<p class="text-xl font-bold text-ink leading-tight truncate">{t.nome}</p>
										<p class="mt-0.5 text-sm text-ink-2 tabular">
											{#if diaInterno === hoje}
												{feriados.get(hoje) ?? (t.fim === hoje ? 'Até o fim do dia' : `Até ${diaSemana(t.fim)}`)}
											{:else}
												{quandoComeca(diaInterno, hoje)}
											{/if}
										</p>
										{#if t.observacao}<p class="text-[13px] text-ink-3 truncate">{t.observacao}</p>{/if}
									</div>
								</button>
							{/each}
						{/if}
					{/each}
				</div>
			</section>

			<section aria-label="Técnicos externos" class="space-y-3">
				<h2 class="text-[13px] font-bold uppercase tracking-wide text-ink-3">Técnicos externos</h2>
				<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
					{#each externos as c (c.id)}
						<div class="panel p-4 space-y-3 min-w-0">
							<p class="flex items-center gap-1.5 text-[15px] font-bold text-ink"><MapPin class="size-4 text-accent" /> {c.nome}</p>
							{#each [{ tipo: 'noturno' as const, rotulo: 'Noturno hoje', turnos: c.noturno }, { tipo: 'domingo' as const, rotulo: ehDomingo(hoje) ? 'Domingo, hoje' : `Domingo, ${diaMes(diaLocal(domingo))}`, turnos: c.domingo }] as linha (linha.tipo)}
								<div class="flex items-start gap-2.5 min-w-0">
									{#if linha.tipo === 'noturno'}
										<Moon class="size-4 mt-0.5 text-ink-3 shrink-0" />
									{:else}
										<Sun class="size-4 mt-0.5 text-ink-3 shrink-0" />
									{/if}
									<div class="min-w-0 flex-1">
										<p class="text-[12px] font-semibold text-ink-3">{linha.rotulo}</p>
										{#if linha.turnos.length === 0}
											<p class="flex items-center gap-2 text-sm font-semibold text-warn">
												Sem técnico
												{#if ehAdmin}
													<button
														onclick={() => (novo = { tipo: linha.tipo, cidade: c.id, inicio: linha.tipo === 'noturno' ? hoje : domingo })}
														class="font-semibold text-accent hover:underline underline-offset-2 cursor-pointer">Escalar</button
													>
												{/if}
											</p>
										{:else}
											{#each linha.turnos as t (t.id)}
												<button onclick={() => (selecionado = t)} class="flex items-center gap-2 max-w-full text-left cursor-pointer hover:underline underline-offset-2">
													<span class="dot size-2 shrink-0" style="background-color: {corDaPessoa(t.nome)};"></span>
													<span class="text-[15px] font-semibold text-ink truncate">{t.nome}</span>
													{#if linha.tipo === 'noturno' && t.fim !== hoje}
														<span class="text-[13px] text-ink-3 tabular shrink-0">até {diaSemana(t.fim)}</span>
													{/if}
												</button>
											{/each}
										{/if}
									</div>
								</div>
							{/each}
						</div>
					{/each}
				</div>

				{#if folgaHoje.length > 0}
					<div class="flex flex-wrap items-center gap-2">
						<span class="flex items-center gap-1.5 text-sm font-semibold text-ink-2"><Coffee class="size-4" /> De folga hoje</span>
						{#each folgaHoje as t (t.id)}
							<button
								onclick={() => (selecionado = t)}
								class="inline-flex items-center gap-2 h-8 px-3 rounded-full bg-sunken text-[13px] font-semibold text-ink-2 cursor-pointer hover:bg-muted"
							>
								<span class="dot size-2" style="background-color: {corDaPessoa(t.nome)};"></span>
								{t.nome}
								<span class="font-normal text-ink-3">{t.folga_fim === hoje ? 'hoje' : `até ${diaSemana(t.folga_fim!)}`}</span>
							</button>
						{/each}
					</div>
				{/if}
			</section>

			<!-- Indicadores dos próximos 30 dias -->
			<section class="grid grid-cols-1 sm:grid-cols-3 gap-4" aria-label="Próximos 30 dias">
				<div class="panel p-4">
					<p class="flex items-center gap-1.5 text-[13px] font-semibold text-ink-3"><ArrowRightLeft class="size-4" /> Próxima troca</p>
					{#if proximaTroca}
						<p class="mt-1 text-lg font-bold text-ink truncate">{proximaTroca.nome}</p>
						<p class="text-sm text-ink-2 tabular truncate">
							{quandoComeca(proximaTroca.inicio, hoje)} · {diaSemana(proximaTroca.inicio)} · {rotuloCurto(proximaTroca)}
						</p>
					{:else}
						<p class="mt-1 text-lg font-bold text-ink-3">—</p>
						<p class="text-sm text-ink-3">Nenhuma nos próximos 30 dias</p>
					{/if}
				</div>
				<div class="panel p-4">
					<p class="flex items-center gap-1.5 text-[13px] font-semibold text-ink-3"><ShieldAlert class="size-4" /> Dias sem cobertura</p>
					<p class="mt-1 text-lg font-bold tabular {diasSemCobertura > 0 ? 'text-warn' : 'text-ink'}">
						{diasSemCobertura}
						{#if diasSemCobertura > 0}
							<span class="text-sm font-medium text-ink-3">em {escalasDescobertas} {escalasDescobertas === 1 ? 'escala' : 'escalas'}</span>
						{/if}
					</p>
					<p class="text-sm text-ink-2 truncate">
						{#if buracos.length === 0}
							Todas as escalas cobertas
						{:else}
							Primeiro: {diaSemana(buracos[0].inicio)} · {buracos[0].escala.rotulo}
						{/if}
					</p>
				</div>
				<div class="panel p-4">
					<p class="flex items-center gap-1.5 text-[13px] font-semibold text-ink-3"><Users class="size-4" /> Seu próximo turno</p>
					{#if meuProximo}
						<p class="mt-1 text-lg font-bold text-ink truncate">{rotuloEscala(meuProximo)}</p>
						<p class="text-sm text-ink-2 tabular">
							{cobre(meuProximo, hoje) ? `Em andamento, ${quantoFalta(meuProximo.fim, hoje)}` : `${quandoComeca(meuProximo.inicio, hoje)} · ${periodo(meuProximo)}`}
						</p>
					{:else}
						<p class="mt-1 text-lg font-bold text-ink-3">—</p>
						<p class="text-sm text-ink-3">Você não está na escala</p>
					{/if}
				</div>
			</section>

			<div class="grid grid-cols-1 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] gap-5 items-start">
				<!-- Próximos turnos -->
				<section class="panel">
					<div class="flex items-baseline justify-between gap-3 px-5 pt-4 pb-3 border-b border-line">
						<h2 class="text-[15px] font-bold text-ink">Próximos turnos</h2>
						<button onclick={() => trocarAba('calendario')} class="text-[13px] font-semibold text-accent hover:underline underline-offset-2 cursor-pointer">Ver calendário</button>
					</div>
					{#if aComecar.length === 0 && buracos.length === 0}
						<p class="px-5 py-8 text-center text-sm text-ink-3">Nada novo nos próximos 30 dias.</p>
					{:else}
						<ul class="divide-y divide-line">
							{#each aComecar as t (t.id)}
								<li>
									<button onclick={() => (selecionado = t)} class="w-full flex items-center gap-3 px-5 py-3 text-left hover:bg-sunken cursor-pointer">
										<span class="w-1 self-stretch rounded-full shrink-0" style="background-color: {corDaPessoa(t.nome)};"></span>
										<div class="min-w-0 flex-1">
											<p class="text-sm font-semibold text-ink truncate">
												{t.nome}
												<span class="ml-1 font-normal text-ink-3">{rotuloCurto(t)}</span>
											</p>
											<p class="text-[13px] text-ink-3 tabular">
												{periodo(t)}{#if periodoFolga(t)}<span> · folga {periodoFolga(t)}</span>{/if}
											</p>
										</div>
										<span class="text-[13px] font-semibold text-ink-2 tabular shrink-0">{quandoComeca(t.inicio, hoje)}</span>
									</button>
								</li>
							{/each}
							{#each buracos as b (b.escala.rotulo + b.inicio)}
								<li class="flex items-center gap-3 px-5 py-3 bg-warn-soft/50">
									<ShieldAlert class="size-4 text-warn shrink-0" />
									<p class="flex-1 min-w-0 text-sm text-ink">
										<span class="font-semibold">{b.escala.rotulo}</span> sem ninguém: <span class="tabular">{textoBuraco(b)}</span>
									</p>
									{#if ehAdmin}
										<button
											onclick={() =>
												(novo = {
													tipo: b.escala.tipo,
													cidade: b.escala.cidade,
													periodo: b.escala.periodo,
													inicio: b.inicio,
													// Domingo e interno: um turno por dia; o resto vai pelo rodízio
													fim: b.escala.tipo === 'noturno' ? b.fim : b.inicio
												})}
											class="btn btn-secondary btn-sm">Cobrir</button
										>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<!-- Dias de plantão por pessoa -->
				<section class="panel">
					<div class="px-5 pt-4 pb-3 border-b border-line">
						<h2 class="text-[15px] font-bold text-ink">Dias de plantão por pessoa</h2>
						<p class="text-[13px] text-ink-3">Nos próximos {DIAS_DASH} dias</p>
					</div>
					{#if carga.length === 0}
						<p class="px-5 py-8 text-center text-sm text-ink-3">Ninguém escalado.</p>
					{:else}
						<ul class="px-5 py-4 space-y-3">
							{#each carga as c (c.nome)}
								<li title="{c.nome}: {c.dias} {c.dias === 1 ? 'dia' : 'dias'} de plantão, somando as escalas">
									<div class="flex items-baseline justify-between gap-3 text-sm">
										<span class="flex items-center gap-2 min-w-0 font-medium text-ink">
											<span class="dot size-2 shrink-0" style="background-color: {corDaPessoa(c.nome)};"></span>
											<span class="truncate">{c.nome}</span>
										</span>
										<span class="text-ink-2 tabular shrink-0">{c.dias} {c.dias === 1 ? 'dia' : 'dias'}</span>
									</div>
									<div class="mt-1 h-2 rounded-full bg-sunken">
										<div class="h-2 rounded-full bg-accent" style="width: {(c.dias / maiorCarga) * 100}%;"></div>
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			</div>
		{/if}
	{:else}
		<div class="segmented" role="group" aria-label="Escala">
			<button aria-pressed={filtro === 'todas'} onclick={() => (filtro = 'todas')}>Todas</button>
			<button aria-pressed={filtro === 'interno'} onclick={() => (filtro = 'interno')}>Interno</button>
			{#each CIDADES as c (c.id)}
				<button aria-pressed={filtro === c.id} onclick={() => (filtro = c.id)}>{c.nome}</button>
			{/each}
		</div>
		<div class="panel relative p-3 sm:p-5">
			{#if carregandoCal}
				<div class="absolute top-5 right-5 z-10"><div class="spinner size-5"></div></div>
			{/if}
			<div class="ec-theme-custom overflow-x-auto min-h-[600px]">
				<Calendar {plugins} {options} />
			</div>
		</div>
		<p class="text-[13px] text-ink-3">
			Interno (domingos e feriados) em faixa listrada, noturno cheio, domingo em contorno tracejado e folga em cinza, na cor de cada pessoa.
			{#if ehAdmin}Selecione dias no calendário para escalar alguém.{/if}
		</p>
	{/if}

	<!-- Detalhes do turno -->
	{#if selecionado}
		{@const t = selecionado}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div class="flex gap-3 min-w-0">
						<span class="w-1 self-stretch rounded-full shrink-0" style="background-color: {corDaPessoa(t.nome)};"></span>
						<div class="min-w-0">
							<p class="text-[13px] font-semibold text-ink-3">{rotuloEscala(t)}</p>
							<h3 class="modal-title">{t.nome}</h3>
							<p class="mt-1 flex items-center gap-1.5 text-sm text-ink-2 tabular">
								<CalendarDays class="size-4 text-ink-3" />
								{periodo(t)}
							</p>
							{#if periodoFolga(t)}
								<p class="mt-1 flex items-center gap-1.5 text-sm text-ink-2 tabular">
									<Coffee class="size-4 text-ink-3" />
									Folga {periodoFolga(t)}
								</p>
							{/if}
						</div>
					</div>
					<button onclick={() => (selecionado = null)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>
				{#if t.observacao}
					<div class="modal-body">
						<p class="text-[15px] text-ink whitespace-pre-wrap">{t.observacao}</p>
					</div>
				{/if}
				<div class="modal-foot">
					{#if ehAdmin}
						<button onclick={() => excluir(t)} class="btn btn-danger mr-auto">
							<Trash2 class="size-4" />
							<span>Excluir</span>
						</button>
						<button
							onclick={() => {
								editando = t;
								selecionado = null;
							}}
							class="btn btn-primary"
						>
							<Edit3 class="size-4" />
							<span>Editar</span>
						</button>
					{:else}
						<span class="mr-auto text-[13px] text-ink-3">A escala é montada pelos administradores.</span>
						<button onclick={() => (selecionado = null)} class="btn btn-secondary">Fechar</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}

	{#if novo || editando}
		<EscalaModal
			turno={editando}
			tipo={novo?.tipo}
			cidade={novo?.cidade}
			periodo={novo?.periodo}
			inicio={novo?.inicio}
			fim={novo?.fim}
			onfechar={() => {
				novo = null;
				editando = null;
			}}
			onsalvo={aposSalvar}
		/>
	{/if}
</div>
