<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { TIPO_PLANTAO, corDaPessoa, mesmaPessoa, nomeCidade, nomePeriodo, rotuloCurto, type Turno } from '$lib/plantao';
	import { Calendar, CalendarClock, CheckSquare, Plus, Flag, Megaphone, Pencil, Trash2, X, Activity, CircleAlert, ShieldCheck } from 'lucide-svelte';

	type NivelAviso = 'info' | 'atencao' | 'critico';

	interface Aviso {
		id: string;
		titulo: string;
		mensagem: string;
		nivel: NivelAviso;
		expira_em: string | null;
		criador_nome: string;
		criador_cor: string;
		criado_em: string;
		pode_editar: boolean;
	}

	interface PainelDados {
		avisos: Aviso[];
		monitores: {
			ativos: number;
			online: number;
			offline: number;
			fora: { id: string; nome: string; alvo: string; ultimo_erro: string; status_desde: string }[];
		};
		plantao: {
			hoje: Turno[];
			meu_proximo: Turno | null;
		};
		proximos_eventos: {
			id: string;
			titulo: string;
			inicio: string;
			fim: string;
			dia_inteiro: boolean;
			criador_nome: string;
			criador_cor: string;
		}[];
		tarefas_pendentes: {
			id: string;
			titulo: string;
			prioridade: string;
			prazo: string | null;
			atrasada: boolean;
		}[];
	}

	let dados = $state<PainelDados | null>(null);
	let loading = $state(true);

	async function carregarPainel() {
		loading = true;
		try {
			// Uma única chamada GET /api/painel conforme requisito do P1.6
			dados = await apiFetch<PainelDados>('/api/painel');
		} catch (err) {
			console.error('Erro ao carregar dados do painel:', err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		carregarPainel();
	});

	// Mural de avisos
	const NIVEIS: Record<NivelAviso, { rotulo: string; tag: string; borda: string }> = {
		critico: { rotulo: 'Crítico', tag: 'tag-danger', borda: 'border-danger' },
		atencao: { rotulo: 'Atenção', tag: 'tag-warn', borda: 'border-warn' },
		info: { rotulo: 'Informativo', tag: 'tag-accent', borda: 'border-accent' }
	};

	let avisoModalAberto = $state(false);
	let avisoSalvando = $state(false);
	let avisoId = $state<string | null>(null);
	let avisoTitulo = $state('');
	let avisoMensagem = $state('');
	let avisoNivel = $state<NivelAviso>('info');
	let avisoExpira = $state('');

	// YYYY-MM-DD no fuso local (o <input type="date"> não entende ISO com hora)
	function dataLocalISO(d: Date): string {
		const pad = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
	}

	function abrirNovoAviso() {
		avisoId = null;
		avisoTitulo = '';
		avisoMensagem = '';
		avisoNivel = 'info';
		const emUmaSemana = new Date();
		emUmaSemana.setDate(emUmaSemana.getDate() + 7);
		avisoExpira = dataLocalISO(emUmaSemana);
		avisoModalAberto = true;
	}

	function abrirEditarAviso(a: Aviso) {
		avisoId = a.id;
		avisoTitulo = a.titulo;
		avisoMensagem = a.mensagem;
		avisoNivel = a.nivel;
		avisoExpira = a.expira_em ? dataLocalISO(new Date(a.expira_em)) : '';
		avisoModalAberto = true;
	}

	async function salvarAviso() {
		if (!avisoTitulo.trim()) {
			alert('O título do aviso é obrigatório.');
			return;
		}
		const payload = {
			titulo: avisoTitulo.trim(),
			mensagem: avisoMensagem.trim(),
			nivel: avisoNivel,
			// Vale até o fim do dia escolhido
			expira_em: avisoExpira ? new Date(avisoExpira + 'T23:59:59').toISOString() : null
		};
		avisoSalvando = true;
		try {
			await apiFetch(avisoId ? `/api/avisos/${avisoId}` : '/api/avisos', {
				method: avisoId ? 'PUT' : 'POST',
				body: JSON.stringify(payload)
			});
			avisoModalAberto = false;
			await carregarPainel();
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			avisoSalvando = false;
		}
	}

	async function excluirAviso(a: Aviso) {
		if (!confirm(`Tirar o aviso "${a.titulo}" do mural?`)) return;
		try {
			await apiFetch(`/api/avisos/${a.id}`, { method: 'DELETE' });
			await carregarPainel();
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	const hoje = new Date();

	function saudacao(): string {
		const h = hoje.getHours();
		if (h < 12) return 'Bom dia';
		if (h < 18) return 'Boa tarde';
		return 'Boa noite';
	}

	function primeiroNome(nome?: string): string {
		return nome?.trim().split(/\s+/)[0] ?? '';
	}

	// Dias de calendário entre hoje e a data (0 = hoje; negativo = já começou)
	function diasAte(iso: string): number {
		const d = new Date(iso);
		const alvo = new Date(d.getFullYear(), d.getMonth(), d.getDate());
		const base = new Date(hoje.getFullYear(), hoje.getMonth(), hoje.getDate());
		return Math.round((alvo.getTime() - base.getTime()) / 86_400_000);
	}

	function rotuloDia(iso: string): string {
		const n = diasAte(iso);
		if (n <= 0) return 'Hoje';
		if (n === 1) return 'Amanhã';
		return new Date(iso).toLocaleDateString('pt-BR', { weekday: 'short' }).replace('.', '');
	}

	function dataCurta(iso: string): string {
		return new Date(iso).toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' });
	}

	function haQuanto(iso: string): string {
		const min = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60000));
		if (min < 1) return 'agora';
		if (min < 60) return `há ${min} min`;
		const h = Math.floor(min / 60);
		if (h < 24) return `há ${h} h`;
		const d = Math.floor(h / 24);
		return `há ${d} ${d === 1 ? 'dia' : 'dias'}`;
	}

	// Turno do usuário: em andamento ou quando começa (datas AAAA-MM-DD, fim inclusivo)
	function textoMeuTurno(t: Turno): string {
		// "plantão interno da manhã", "plantão noturno em Bagé"
		const tipo =
			TIPO_PLANTAO[t.tipo].toLowerCase() +
			(t.cidade ? ` em ${nomeCidade(t.cidade)}` : t.periodo ? ` da ${nomePeriodo(t.periodo).toLowerCase()}` : '');
		const dia = (d: string) =>
			new Date(d + 'T00:00:00').toLocaleDateString('pt-BR', { weekday: 'short', day: '2-digit', month: '2-digit' }).replace('.', '');
		const inicio = diasAte(t.inicio + 'T00:00:00');
		if (inicio <= 0) return `Você está de ${tipo} ${t.fim === dataLocalISO(hoje) ? 'até o fim do dia' : `até ${dia(t.fim)}`}`;
		const quando = inicio === 1 ? 'amanhã' : `em ${inicio} dias`;
		return `Seu próximo ${tipo} começa ${quando}: ${t.inicio === t.fim ? dia(t.inicio) : `${dia(t.inicio)} a ${dia(t.fim)}`}`;
	}

	const atalhos = $derived(
		[
			{ href: '/tarefas', label: 'Tarefas', desc: 'Quadro da equipe', icon: CheckSquare, modulo: 'tarefas' as const },
			{ href: '/calendario', label: 'Calendário', desc: 'Marcações do mês', icon: Calendar, modulo: 'calendario' as const }
		].filter((a) => auth.temModulo(a.modulo))
	);

	function quandoFalta(iso: string): string {
		const n = diasAte(iso);
		if (n < 0) return 'em andamento';
		if (n === 0) return 'é hoje';
		if (n === 1) return 'é amanhã';
		return `em ${n} dias`;
	}
</script>

{#snippet cabecalho(titulo: string, total: number | null, href: string, linkTexto: string)}
	<div class="flex items-baseline justify-between gap-3 px-5 pt-4 pb-3 border-b border-line">
		<h2 class="text-[15px] font-bold text-ink">
			{titulo}
			{#if total}
				<span class="ml-1 font-semibold text-ink-3 tabular">{total}</span>
			{/if}
		</h2>
		<a href={href} class="text-[13px] font-semibold text-accent hover:underline underline-offset-2">{linkTexto}</a>
	</div>
{/snippet}

{#snippet vazio(texto: string)}
	<p class="px-5 py-10 text-sm text-ink-3 text-center">{texto}</p>
{/snippet}

<div class="space-y-8">
	<div class="page-head">
		<div>
			<p class="text-sm text-ink-3 first-letter:uppercase">
				{hoje.toLocaleDateString('pt-BR', { weekday: 'long', day: 'numeric', month: 'long' })}
			</p>
			<h1 class="page-title mt-0.5">{saudacao()}, {primeiroNome(auth.user?.nome)}</h1>
		</div>
		{#if auth.temModulo('tarefas')}
			<div class="flex flex-wrap gap-2">
				<a href="/tarefas" class="btn btn-primary">
					<Plus class="size-4" />
					<span>Nova tarefa</span>
				</a>
			</div>
		{/if}
	</div>

	{#if loading}
		<div class="flex justify-center py-20"><div class="spinner"></div></div>
	{:else if dados}
		<!-- Mural de avisos -->
		{#if auth.temModulo('avisos')}
			<section class="panel">
				<div class="flex items-center justify-between gap-3 px-5 pt-4 pb-3 border-b border-line">
					<h2 class="flex items-center gap-2 text-[15px] font-bold text-ink">
						<Megaphone class="size-4 text-ink-3" />
						Mural de avisos
						{#if dados.avisos.length}
							<span class="font-semibold text-ink-3 tabular">{dados.avisos.length}</span>
						{/if}
					</h2>
					<button onclick={abrirNovoAviso} class="btn btn-sm btn-soft">
						<Plus class="size-4" />
						<span>Novo aviso</span>
					</button>
				</div>
				{#if dados.avisos.length === 0}
					<p class="px-5 py-6 text-sm text-ink-3 text-center">Nenhum aviso no mural.</p>
				{:else}
					<ul class="divide-y divide-line">
						{#each dados.avisos as a (a.id)}
							{@const nivel = NIVEIS[a.nivel]}
							<li class="group flex items-start gap-3 px-5 py-3">
								<div class="min-w-0 flex-1 border-l-[3px] pl-3 {nivel.borda}">
									<div class="flex flex-wrap items-center gap-2">
										<span class="font-semibold text-ink text-sm leading-snug">{a.titulo}</span>
										{#if a.nivel !== 'info'}<span class="tag {nivel.tag}">{nivel.rotulo}</span>{/if}
									</div>
									{#if a.mensagem}
										<p class="text-sm text-ink-2 mt-1 whitespace-pre-line break-words">{a.mensagem}</p>
									{/if}
									<div class="flex flex-wrap gap-x-3 text-[13px] text-ink-3 mt-1">
										<span>{a.criador_nome}</span>
										<span>{a.expira_em ? `até ${dataCurta(a.expira_em)}` : 'sem prazo'}</span>
									</div>
								</div>
								{#if a.pode_editar}
									<div class="flex shrink-0 gap-1">
										<button onclick={() => abrirEditarAviso(a)} class="icon-btn" title="Editar" aria-label="Editar aviso">
											<Pencil class="size-4" />
										</button>
										<button onclick={() => excluirAviso(a)} class="icon-btn icon-btn-danger" title="Excluir" aria-label="Excluir aviso">
											<Trash2 class="size-4" />
										</button>
									</div>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		{/if}

		<!-- Monitor de disponibilidade (vem vazio se o módulo está desligado) -->
		{#if dados.monitores.fora.length > 0}
			<section class="rounded-xl border border-danger/40 bg-danger-soft">
				<a href="/monitor" class="flex items-center gap-2 px-4 pt-3.5 pb-2 text-sm font-bold text-danger hover:underline underline-offset-2">
					<CircleAlert class="size-5 shrink-0" />
					{dados.monitores.fora.length === 1 ? '1 serviço fora do ar' : `${dados.monitores.fora.length} serviços fora do ar`}
				</a>
				<ul class="px-4 pb-3.5 pl-11 space-y-1 text-sm">
					{#each dados.monitores.fora as m (m.id)}
						<li class="text-ink">
							<span class="font-semibold">{m.nome}</span>
							<span class="text-ink-2">· {haQuanto(m.status_desde)}{m.ultimo_erro ? ` · ${m.ultimo_erro}` : ''}</span>
						</li>
					{/each}
				</ul>
			</section>
		{:else if dados.monitores.ativos > 0}
			<a href="/monitor" class="flex items-center gap-2.5 px-4 py-3 rounded-xl border border-line hover:border-line-strong text-sm transition-colors">
				<Activity class="size-4 text-ok shrink-0" />
				<span class="text-ink-2">
					{dados.monitores.online === dados.monitores.ativos
						? `Todos os ${dados.monitores.ativos} serviços monitorados estão no ar.`
						: `${dados.monitores.online} de ${dados.monitores.ativos} serviços confirmados no ar.`}
				</span>
			</a>
		{/if}

		<!-- Escala de plantão -->
		{#if dados.plantao.hoje.length > 0 || dados.plantao.meu_proximo}
			<a href="/plantao" class="flex flex-col sm:flex-row sm:items-center gap-x-5 gap-y-2 px-4 py-3 rounded-xl border border-line hover:border-line-strong transition-colors">
				<div class="flex items-center gap-2.5 min-w-0 flex-1 flex-wrap">
					<ShieldCheck class="size-4 text-accent shrink-0" />
					<span class="text-sm font-semibold text-ink">De plantão hoje</span>
					{#if dados.plantao.hoje.length === 0}
						<span class="text-sm text-ink-3">ninguém escalado</span>
					{:else}
						{#each dados.plantao.hoje as t (t.id)}
							<span class="tag h-7 px-2.5 text-[13px] {auth.user && mesmaPessoa(t.nome, auth.user.nome) ? 'tag-accent' : ''}">
								<span class="dot size-2" style="background-color: {corDaPessoa(t.nome)};"></span>
								{t.nome}
								<span class="font-normal text-ink-3">{t.tipo === 'interno' ? nomePeriodo(t.periodo).toLowerCase() : rotuloCurto(t)}</span>
							</span>
						{/each}
					{/if}
				</div>
				{#if dados.plantao.meu_proximo}
					<span class="text-[13px] text-ink-2 sm:text-right">{textoMeuTurno(dados.plantao.meu_proximo)}</span>
				{/if}
			</a>
		{/if}

		{#if dados.proximos_eventos.length > 0}
			{@const proximo = dados.proximos_eventos[0]}
			<a href="/calendario" class="flex items-start gap-3 px-4 py-3.5 rounded-xl border border-accent/30 bg-accent-soft hover:border-accent/50 transition-colors">
				<CalendarClock class="size-5 mt-0.5 text-accent shrink-0" />
				<div class="min-w-0 text-sm text-ink">
					<div class="font-semibold">
						{dados.proximos_eventos.length === 1
							? 'Você tem 1 marcação nos próximos 7 dias'
							: `Você tem ${dados.proximos_eventos.length} marcações nos próximos 7 dias`}
					</div>
					<div class="text-ink-2 mt-0.5">
						A próxima, <span class="font-semibold">{proximo.titulo}</span>, {quandoFalta(proximo.inicio)}
						({rotuloDia(proximo.inicio).toLowerCase()}, {dataCurta(proximo.inicio)}).
					</div>
				</div>
			</a>
		{/if}

		<div class="grid grid-cols-1 {auth.temModulo('calendario') && auth.temModulo('tarefas') ? 'lg:grid-cols-2' : ''} gap-5 items-start">
			<!-- Agenda -->
			{#if auth.temModulo('calendario')}
				<section class="panel">
					{@render cabecalho('Próximos 7 dias', dados.proximos_eventos.length, '/calendario', 'Calendário')}
					{#if dados.proximos_eventos.length === 0}
						{@render vazio('Nada marcado para a próxima semana.')}
					{:else}
						<ul class="divide-y divide-line">
							{#each dados.proximos_eventos.slice(0, 6) as ev (ev.id)}
								<li class="flex gap-4 px-5 py-3">
									<div class="w-14 shrink-0 tabular">
										<div class="text-[13px] font-semibold text-ink-3 first-letter:uppercase">{rotuloDia(ev.inicio)}</div>
										<div class="text-[15px] font-bold text-ink">{dataCurta(ev.inicio)}</div>
									</div>
									<div class="min-w-0 flex-1 border-l-[3px] pl-3" style="border-color: {ev.criador_cor};">
										<div class="font-semibold text-ink text-sm leading-snug">{ev.titulo}</div>
										<div class="flex gap-3 text-[13px] text-ink-3 mt-0.5 min-w-0">
											<span class="shrink-0 {diasAte(ev.inicio) <= 1 ? 'font-semibold text-accent' : ''}">{quandoFalta(ev.inicio)}</span>
											{#if diasAte(ev.fim) !== diasAte(ev.inicio)}<span class="shrink-0">até {dataCurta(ev.fim)}</span>{/if}
											<span class="truncate">{ev.criador_nome}</span>
										</div>
									</div>
								</li>
							{/each}
						</ul>
						{#if dados.proximos_eventos.length > 6}
							<a href="/calendario" class="block px-5 py-3 border-t border-line text-[13px] font-semibold text-ink-3 hover:text-accent">
								Mais {dados.proximos_eventos.length - 6} no calendário
							</a>
						{/if}
					{/if}
				</section>
			{/if}

			<!-- Minhas tarefas -->
			{#if auth.temModulo('tarefas')}
				<section class="panel">
					{@render cabecalho('Minhas tarefas', dados.tarefas_pendentes.length, '/tarefas', 'Ver todas')}
					{#if dados.tarefas_pendentes.length === 0}
						{@render vazio('Nenhuma tarefa pendente com você.')}
					{:else}
						<ul class="divide-y divide-line">
							{#each dados.tarefas_pendentes as t (t.id)}
								<li class="flex items-start gap-3 px-5 py-3">
									<span
										class="mt-1.5 size-2 shrink-0 rounded-full {t.atrasada ? 'bg-danger' : t.prioridade === 'alta' ? 'bg-warn' : 'bg-line-strong'}"
										aria-hidden="true"
									></span>
									<div class="min-w-0 flex-1">
										<div class="font-semibold text-ink text-sm leading-snug">{t.titulo}</div>
										<div class="text-[13px] mt-0.5 {t.atrasada ? 'text-danger font-semibold' : 'text-ink-3'}">
											{t.atrasada ? 'Atrasada, prazo' : t.prazo ? 'Prazo' : 'Sem prazo'}
											{#if t.prazo}{new Date(t.prazo).toLocaleDateString('pt-BR')}{/if}
										</div>
									</div>
									{#if t.prioridade === 'alta'}
										<Flag class="size-3.5 mt-1 text-warn shrink-0" aria-label="Prioridade alta" />
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			{/if}
		</div>

		<!-- Atalhos -->
		{#if atalhos.length}
		<nav class="grid grid-cols-2 gap-3" aria-label="Atalhos">
			{#each atalhos as atalho}
				{@const Icon = atalho.icon}
				<a href={atalho.href} class="group flex items-center gap-3 px-4 py-3.5 rounded-xl border border-line hover:bg-surface hover:border-line-strong transition-colors">
					<Icon class="size-5 text-ink-3 group-hover:text-accent transition-colors" />
					<div class="min-w-0">
						<div class="text-sm font-semibold text-ink">{atalho.label}</div>
						<div class="text-[13px] text-ink-3 truncate">{atalho.desc}</div>
					</div>
				</a>
			{/each}
		</nav>
		{/if}
	{:else}
		<div class="alert bg-danger-soft text-danger">Não foi possível carregar o painel. Recarregue a página para tentar de novo.</div>
	{/if}

	{#if avisoModalAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{avisoId ? 'Editar aviso' : 'Novo aviso'}</h3>
					<button onclick={() => (avisoModalAberto = false)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div>
						<label class="label" for="av-titulo">Título</label>
						<input
							id="av-titulo"
							type="text"
							bind:value={avisoTitulo}
							maxlength="200"
							placeholder="Ex.: Link da operadora X instável"
							class="field"
							autofocus
						/>
					</div>

					<div>
						<label class="label" for="av-msg">Mensagem <span class="font-normal text-ink-3">(opcional)</span></label>
						<textarea id="av-msg" bind:value={avisoMensagem} maxlength="2000" rows="3" class="field"></textarea>
					</div>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div>
							<label class="label" for="av-nivel">Nível</label>
							<select id="av-nivel" bind:value={avisoNivel} class="field">
								<option value="info">Informativo</option>
								<option value="atencao">Atenção</option>
								<option value="critico">Crítico</option>
							</select>
						</div>
						<div>
							<label class="label" for="av-expira">Mostrar até <span class="font-normal text-ink-3">(opcional)</span></label>
							<input id="av-expira" type="date" bind:value={avisoExpira} min={dataLocalISO(new Date())} class="field" />
						</div>
					</div>
					<p class="hint">Some do mural sozinho no fim do dia escolhido. Deixe a data em branco para ficar até alguém excluir.</p>
				</div>

				<div class="modal-foot">
					<button onclick={() => (avisoModalAberto = false)} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarAviso} class="btn btn-primary" disabled={avisoSalvando}>
						{avisoId ? 'Salvar alterações' : 'Publicar aviso'}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
