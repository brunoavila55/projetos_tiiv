<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import EscalaModal from '$lib/components/plantao/EscalaModal.svelte';
	import { Calendar, DayGrid, Interaction, List } from '@event-calendar/core';
	import { Plus, CalendarDays, X, Trash2, Edit3, Tv, ShieldAlert, BellRing, ArrowRightLeft, Users, Coffee } from 'lucide-svelte';
	import {
		TIPO_PLANTAO,
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
	let carregandoResumo = $state(true);

	async function carregarResumo() {
		hoje = paraDia(new Date());
		const fim = paraDia(somarDias(diaLocal(hoje), DIAS_DASH - 1));
		try {
			proximos = await apiFetch<Turno[]>(`/api/plantoes?inicio=${hoje}&fim=${fim}`);
		} catch {
			// apiFetch já avisou
		} finally {
			carregandoResumo = false;
		}
	}

	const dias = $derived(Array.from({ length: DIAS_DASH }, (_, i) => paraDia(somarDias(diaLocal(hoje), i))));
	const plantaoHoje = $derived(proximos.filter((t) => t.tipo === 'plantao' && cobre(t, hoje)));
	const sobreavisoHoje = $derived(proximos.filter((t) => t.tipo === 'sobreaviso' && cobre(t, hoje)));
	const folgaHoje = $derived(proximos.filter((t) => deFolga(t, hoje)));
	// A listagem também traz turnos já passados que aparecem só pela folga
	const vigentes = $derived(proximos.filter((t) => t.fim >= hoje));
	const aComecar = $derived(proximos.filter((t) => t.inicio > hoje));
	const proximaTroca = $derived(aComecar.find((t) => t.tipo === 'plantao') ?? null);
	const meuProximo = $derived(auth.user ? (vigentes.find((t) => mesmaPessoa(t.nome, auth.user!.nome)) ?? null) : null);

	// Dias sem ninguém de plantão, agrupados em trechos seguidos
	const buracos = $derived.by(() => {
		const trechos: { inicio: string; fim: string }[] = [];
		for (const d of dias) {
			if (proximos.some((t) => t.tipo === 'plantao' && cobre(t, d))) continue;
			const ultimo = trechos.at(-1);
			if (ultimo && paraDia(somarDias(diaLocal(ultimo.fim), 1)) === d) ultimo.fim = d;
			else trechos.push({ inicio: d, fim: d });
		}
		return trechos;
	});
	const diasSemCobertura = $derived(buracos.reduce((n, b) => n + diasEntre(b.inicio, b.fim) + 1, 0));

	// Dias de plantão por pessoa na janela (só a parte do turno dentro dela)
	const carga = $derived.by(() => {
		const ultimoDia = dias[dias.length - 1];
		const porPessoa: { nome: string; dias: number }[] = [];
		for (const t of vigentes) {
			if (t.tipo !== 'plantao') continue;
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
	let carregandoCal = $state(false);

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
		// Plantão antes de sobreaviso em cada dia
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
			escala = await apiFetch<Turno[]>(`/api/plantoes?${params}`);
			const folgas = escala
				.filter((t) => t.folga_inicio && t.folga_fim)
				.map((t) => ({
					id: PREFIXO_FOLGA + t.id,
					title: `Folga · ${t.nome}`,
					start: t.folga_inicio!,
					end: paraDia(somarDias(diaLocal(t.folga_fim!), 1)),
					allDay: true,
					classNames: ['ec-folga'],
					styles: [`--cor-escala: ${corDaPessoa(t.nome)}`],
					extendedProps: { ordem: 2 }
				}));
			const turnos = escala.map((t) => ({
				id: PREFIXO + t.id,
				title: t.tipo === 'plantao' ? t.nome : `${t.nome} · sobreaviso`,
				start: t.inicio,
				end: paraDia(somarDias(diaLocal(t.fim), 1)),
				allDay: true,
				// Plantão: faixa listrada na cor da pessoa; sobreaviso: só contorno tracejado
				classNames: [t.tipo === 'plantao' ? 'ec-plantao' : 'ec-sobreaviso'],
				backgroundColor: t.tipo === 'plantao' ? corDaPessoa(t.nome) : 'transparent',
				styles: [`--cor-escala: ${corDaPessoa(t.nome)}`],
				extendedProps: { ordem: t.tipo === 'plantao' ? 0 : 1 }
			}));
			options.events = [...turnos, ...folgas];
		} catch {
			// apiFetch já avisou
		} finally {
			carregandoCal = false;
		}
	}

	$effect(() => {
		if (aba === 'calendario' && faixa) untrack(carregarCalendario);
	});
	$effect(() => {
		if (aba === 'resumo') untrack(carregarResumo);
	});

	// ---------- Turnos: detalhes, criar, editar, excluir ----------
	let selecionado = $state<Turno | null>(null);
	let novo = $state<{ inicio?: string; fim?: string } | null>(null);
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
			<p class="page-sub">Quem está de plantão e sobreaviso, e a escala das próximas semanas.</p>
		</div>

		<div class="flex flex-wrap lg:flex-nowrap lg:shrink-0 items-center gap-2">
			<div class="segmented shrink-0" role="group" aria-label="Visão">
				<button aria-pressed={aba === 'resumo'} onclick={() => trocarAba('resumo')}>Resumo</button>
				<button aria-pressed={aba === 'calendario'} onclick={() => trocarAba('calendario')}>Calendário</button>
			</div>
			<a href="/plantao/tv" class="btn btn-ghost" title="Abrir a escala em tela cheia para o telão">
				<Tv class="size-4" />
				<span class="hidden sm:inline">Modo TV</span>
			</a>
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
			<section aria-label="De plantão hoje" class="space-y-3">
				{#if plantaoHoje.length === 0}
					<div class="flex items-start gap-3 px-4 py-3.5 rounded-xl border border-warn/40 bg-warn-soft">
						<ShieldAlert class="size-5 mt-0.5 text-warn shrink-0" />
						<div class="text-sm">
							<p class="font-semibold text-ink">Ninguém de plantão hoje</p>
							<p class="text-ink-2">
								{sobreavisoHoje.length ? 'Quem está de sobreaviso aparece abaixo.' : ehAdmin ? 'Monte a escala para cobrir o dia.' : 'A escala de hoje está vazia.'}
							</p>
						</div>
					</div>
				{:else}
					<div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
						{#each plantaoHoje as t (t.id)}
							<button
								onclick={() => (selecionado = t)}
								class="panel text-left p-5 flex items-center gap-4 min-w-0 cursor-pointer hover:border-line-strong transition-colors"
								style="border-left: 4px solid {corDaPessoa(t.nome)};"
							>
								<Avatar id={t.id} nome={t.nome} cor={corDaPessoa(t.nome)} class="size-14 text-lg shrink-0" />
								<div class="min-w-0">
									<p class="text-[13px] font-semibold text-ink-3">De plantão hoje</p>
									<p class="text-xl font-bold text-ink leading-tight truncate">{t.nome}</p>
									<p class="mt-0.5 text-sm text-ink-2 tabular">
										{t.fim === hoje ? 'Até o fim do dia' : `Até ${diaSemana(t.fim)}`} · {quantoFalta(t.fim, hoje)}
									</p>
									{#if t.observacao}<p class="text-[13px] text-ink-3 truncate">{t.observacao}</p>{/if}
								</div>
							</button>
						{/each}
					</div>
				{/if}

				{#if sobreavisoHoje.length > 0}
					<div class="flex flex-wrap items-center gap-2">
						<span class="flex items-center gap-1.5 text-sm font-semibold text-ink-2"><BellRing class="size-4" /> Sobreaviso hoje</span>
						{#each sobreavisoHoje as t (t.id)}
							<button
								onclick={() => (selecionado = t)}
								class="inline-flex items-center gap-2 h-8 px-3 rounded-full border border-dashed text-[13px] font-semibold text-ink cursor-pointer hover:bg-sunken"
								style="border-color: {corDaPessoa(t.nome)};"
							>
								{t.nome}
								<span class="font-normal text-ink-3">{t.fim === hoje ? 'hoje' : `até ${diaSemana(t.fim)}`}</span>
							</button>
						{/each}
					</div>
				{/if}
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
						<p class="text-sm text-ink-2 tabular">{quandoComeca(proximaTroca.inicio, hoje)} · {diaSemana(proximaTroca.inicio)}</p>
					{:else}
						<p class="mt-1 text-lg font-bold text-ink-3">—</p>
						<p class="text-sm text-ink-3">Nenhuma nos próximos 30 dias</p>
					{/if}
				</div>
				<div class="panel p-4">
					<p class="flex items-center gap-1.5 text-[13px] font-semibold text-ink-3"><ShieldAlert class="size-4" /> Dias sem plantão</p>
					<p class="mt-1 text-lg font-bold tabular {diasSemCobertura > 0 ? 'text-warn' : 'text-ink'}">
						{diasSemCobertura} <span class="text-sm font-medium text-ink-3">de {DIAS_DASH}</span>
					</p>
					<p class="text-sm text-ink-2 truncate">
						{#if buracos.length === 0}
							Escala coberta
						{:else}
							Primeiro: {periodo(buracos[0])}
						{/if}
					</p>
				</div>
				<div class="panel p-4">
					<p class="flex items-center gap-1.5 text-[13px] font-semibold text-ink-3"><Users class="size-4" /> Seu próximo turno</p>
					{#if meuProximo}
						<p class="mt-1 text-lg font-bold text-ink">{TIPO_PLANTAO[meuProximo.tipo]}</p>
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
												{#if t.tipo === 'sobreaviso'}<span class="ml-1 font-normal text-ink-3">sobreaviso</span>{/if}
											</p>
											<p class="text-[13px] text-ink-3 tabular">
												{periodo(t)}{#if periodoFolga(t)}<span> · folga {periodoFolga(t)}</span>{/if}
											</p>
										</div>
										<span class="text-[13px] font-semibold text-ink-2 tabular shrink-0">{quandoComeca(t.inicio, hoje)}</span>
									</button>
								</li>
							{/each}
							{#each buracos as b (b.inicio)}
								<li class="flex items-center gap-3 px-5 py-3 bg-warn-soft/50">
									<ShieldAlert class="size-4 text-warn shrink-0" />
									<p class="flex-1 text-sm text-ink">Sem plantão: <span class="tabular">{periodo(b)}</span></p>
									{#if ehAdmin}
										<button onclick={() => (novo = { inicio: b.inicio, fim: b.fim })} class="btn btn-secondary btn-sm">Cobrir</button>
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
								<li title="{c.nome}: {c.dias} {c.dias === 1 ? 'dia' : 'dias'} de plantão">
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
		<div class="panel relative p-3 sm:p-5">
			{#if carregandoCal}
				<div class="absolute top-5 right-5 z-10"><div class="spinner size-5"></div></div>
			{/if}
			<div class="ec-theme-custom overflow-x-auto min-h-[600px]">
				<Calendar {plugins} {options} />
			</div>
		</div>
		<p class="text-[13px] text-ink-3">
			Plantão em faixa listrada, sobreaviso em contorno tracejado e folga em cinza, na cor de cada pessoa.
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
							<p class="text-[13px] font-semibold text-ink-3">{TIPO_PLANTAO[t.tipo]}</p>
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
