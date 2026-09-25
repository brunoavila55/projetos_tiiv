<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { toast } from '$lib/toast.svelte';
	import {
		HardHat,
		Plus,
		LogIn,
		LogOut,
		Clock,
		Edit3,
		Trash2,
		X,
		History,
		BarChart3,
		UserCheck,
		Download,
		ChevronLeft,
		ChevronRight,
		NotebookPen
	} from 'lucide-svelte';

	interface Tecnico {
		id: string;
		nome: string;
		empresa: string;
		ativo: boolean;
		criado_em: string;
		registro_aberto_id: string | null;
		entrada_aberta: string | null;
		atividades_abertas: string;
	}

	interface Registro {
		id: string;
		tecnico_id: string;
		tecnico_nome: string;
		tecnico_empresa: string;
		entrada: string;
		saida: string | null;
		duracao_segundos: number | null;
		observacao: string;
		atividades: string;
		entrada_registrada_por_nome: string;
		saida_registrada_por_nome: string | null;
	}

	interface ListaRegistros {
		itens: Registro[];
		total: number;
		pagina: number;
		limite: number;
		total_pages: number;
	}

	interface LinhaRelatorio {
		tecnico_id: string;
		tecnico_nome: string;
		tecnico_empresa: string;
		total_registros: number;
		registros_abertos: number;
		dias_presentes: number;
		segundos_totais: number;
		primeira_entrada: string | null;
		ultima_marcacao: string | null;
	}

	const FUSO = 'America/Sao_Paulo';

	let abaAtiva = $state<'presenca' | 'registros' | 'relatorio'>('presenca');
	let tecnicos = $state<Tecnico[]>([]);
	let mostrarInativos = $state(false);
	let loading = $state(true);
	let agora = $state(Date.now());

	// Filtros compartilhados por Registros e Relatório (primeiro dia do mês até hoje)
	let filtroTecnico = $state('');
	let filtroInicio = $state(dataISO(new Date(new Date().getFullYear(), new Date().getMonth(), 1)));
	let filtroFim = $state(dataISO(new Date()));

	let registros = $state<Registro[]>([]);
	let paginaAtual = $state(1);
	let totalPaginas = $state(1);
	let totalRegistros = $state(0);

	let relatorio = $state<LinhaRelatorio[]>([]);

	// Modal cadastro de técnico
	let modalTecnicoAberto = $state(false);
	let formTecId = $state<string | null>(null);
	let formTecNome = $state('');
	let formTecEmpresa = $state('');
	let formTecAtivo = $state(true);

	// Modal marcação com horário/observação
	let modalMarcacaoAberto = $state(false);
	let marcacaoTecnico = $state<Tecnico | null>(null);
	let marcacaoHorario = $state('');
	let marcacaoObs = $state('');
	let marcacaoAtividades = $state('');

	// Modal "o que foi feito" (qualquer operador)
	let modalAtividadesAberto = $state(false);
	let atividadesRegistroId = $state('');
	let atividadesTitulo = $state('');
	let atividadesTexto = $state('');
	let atividadesSalvando = $state(false);

	// Modal correção de registro (admin)
	let modalRegistroAberto = $state(false);
	let formRegId = $state('');
	let formRegNome = $state('');
	let formRegEntrada = $state('');
	let formRegSaida = $state('');
	let formRegObs = $state('');

	const tecnicosVisiveis = $derived(tecnicos.filter((t) => mostrarInativos || t.ativo));
	const presentes = $derived(tecnicos.filter((t) => t.entrada_aberta).length);
	const totalSegundosRelatorio = $derived(relatorio.reduce((s, l) => s + l.segundos_totais, 0));

	function dataISO(d: Date): string {
		const p = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
	}

	// Valor para <input type="datetime-local"> no horário do navegador
	function paraInputLocal(iso: string | Date): string {
		const d = new Date(iso);
		const p = (n: number) => String(n).padStart(2, '0');
		return `${dataISO(d)}T${p(d.getHours())}:${p(d.getMinutes())}`;
	}

	function hora(iso: string): string {
		return new Date(iso).toLocaleTimeString('pt-BR', { timeZone: FUSO, hour: '2-digit', minute: '2-digit' });
	}

	function dataHora(iso: string): string {
		return new Date(iso).toLocaleString('pt-BR', { timeZone: FUSO, dateStyle: 'short', timeStyle: 'short' });
	}

	function mesmoDia(a: string, b: string): boolean {
		const f = (iso: string) => new Date(iso).toLocaleDateString('pt-BR', { timeZone: FUSO });
		return f(a) === f(b);
	}

	function duracao(segundos: number): string {
		const h = Math.floor(segundos / 3600);
		const m = Math.floor((segundos % 3600) / 60);
		if (h === 0) return `${m} min`;
		return `${h}h ${String(m).padStart(2, '0')}min`;
	}

	function parametrosFiltro(): URLSearchParams {
		const params = new URLSearchParams();
		if (filtroTecnico) params.set('tecnico_id', filtroTecnico);
		if (filtroInicio) params.set('inicio', filtroInicio);
		if (filtroFim) params.set('fim', filtroFim);
		return params;
	}

	async function carregarTecnicos() {
		try {
			tecnicos = await apiFetch<Tecnico[]>('/api/tecnicos');
		} catch (err) {
			console.error('Erro ao listar técnicos:', err);
		} finally {
			loading = false;
		}
	}

	async function carregarRegistros(resetPagina = false) {
		if (resetPagina) paginaAtual = 1;
		loading = true;
		try {
			const params = parametrosFiltro();
			params.set('pagina', paginaAtual.toString());
			const res = await apiFetch<ListaRegistros>(`/api/tecnicos/registros?${params.toString()}`);
			registros = res.itens;
			totalPaginas = Math.max(1, res.total_pages);
			totalRegistros = res.total;
		} catch (err) {
			console.error('Erro ao listar registros:', err);
		} finally {
			loading = false;
		}
	}

	async function carregarRelatorio() {
		loading = true;
		try {
			relatorio = await apiFetch<LinhaRelatorio[]>(`/api/tecnicos/relatorio?${parametrosFiltro().toString()}`);
		} catch (err) {
			console.error('Erro ao gerar relatório:', err);
		} finally {
			loading = false;
		}
	}

	function recarregarAba() {
		if (abaAtiva === 'registros') carregarRegistros(true);
		else if (abaAtiva === 'relatorio') carregarRelatorio();
	}

	function exportarCSV() {
		const rota = abaAtiva === 'relatorio' ? 'relatorio' : 'registros';
		window.open(`/api/tecnicos/${rota}/exportar.csv?${parametrosFiltro().toString()}`, '_blank');
	}

	async function marcar(t: Tecnico, horario?: string, observacao?: string, atividades?: string) {
		const tipo = t.entrada_aberta ? 'saida' : 'entrada';
		const body: Record<string, string> = {};
		if (horario) body.horario = new Date(horario).toISOString();
		if (observacao?.trim()) body.observacao = observacao.trim();
		if (tipo === 'saida' && atividades?.trim()) body.atividades = atividades.trim();
		try {
			await apiFetch(`/api/tecnicos/${t.id}/${tipo}`, { method: 'POST', body: JSON.stringify(body) });
			toast.success(`${tipo === 'entrada' ? 'Entrada' : 'Saída'} de ${t.nome} registrada`);
			modalMarcacaoAberto = false;
			await carregarTecnicos();
		} catch {
			// toast de erro já exibido por apiFetch
		}
	}

	function abrirMarcacao(t: Tecnico) {
		marcacaoTecnico = t;
		marcacaoHorario = paraInputLocal(new Date());
		marcacaoObs = '';
		marcacaoAtividades = t.atividades_abertas ?? '';
		modalMarcacaoAberto = true;
	}

	function abrirAtividades(registroId: string, titulo: string, texto: string) {
		atividadesRegistroId = registroId;
		atividadesTitulo = titulo;
		atividadesTexto = texto;
		modalAtividadesAberto = true;
	}

	async function salvarAtividades() {
		atividadesSalvando = true;
		try {
			await apiFetch(`/api/tecnicos/registros/${atividadesRegistroId}/atividades`, {
				method: 'PATCH',
				body: JSON.stringify({ atividades: atividadesTexto.trim() })
			});
			modalAtividadesAberto = false;
			toast.success('Anotação salva');
			if (abaAtiva === 'registros') await carregarRegistros();
			else await carregarTecnicos();
		} catch {
			// toast de erro já exibido por apiFetch
		} finally {
			atividadesSalvando = false;
		}
	}

	function abrirCriarTecnico() {
		formTecId = null;
		formTecNome = '';
		formTecEmpresa = '';
		formTecAtivo = true;
		modalTecnicoAberto = true;
	}

	function abrirEditarTecnico(t: Tecnico) {
		formTecId = t.id;
		formTecNome = t.nome;
		formTecEmpresa = t.empresa;
		formTecAtivo = t.ativo;
		modalTecnicoAberto = true;
	}

	async function salvarTecnico() {
		if (!formTecNome.trim()) {
			toast.error('Informe o nome do técnico.');
			return;
		}
		const payload = { nome: formTecNome.trim(), empresa: formTecEmpresa.trim(), ativo: formTecAtivo };
		try {
			if (formTecId) {
				await apiFetch(`/api/tecnicos/${formTecId}`, { method: 'PUT', body: JSON.stringify(payload) });
			} else {
				await apiFetch('/api/tecnicos', { method: 'POST', body: JSON.stringify(payload) });
			}
			modalTecnicoAberto = false;
			await carregarTecnicos();
		} catch {
			// toast de erro já exibido por apiFetch
		}
	}

	function abrirEditarRegistro(r: Registro) {
		formRegId = r.id;
		formRegNome = r.tecnico_nome;
		formRegEntrada = paraInputLocal(r.entrada);
		formRegSaida = r.saida ? paraInputLocal(r.saida) : '';
		formRegObs = r.observacao;
		modalRegistroAberto = true;
	}

	async function salvarRegistro() {
		if (!formRegEntrada) {
			toast.error('Informe o horário de entrada.');
			return;
		}
		try {
			await apiFetch(`/api/tecnicos/registros/${formRegId}`, {
				method: 'PUT',
				body: JSON.stringify({
					entrada: new Date(formRegEntrada).toISOString(),
					saida: formRegSaida ? new Date(formRegSaida).toISOString() : null,
					observacao: formRegObs
				})
			});
			modalRegistroAberto = false;
			await Promise.all([carregarRegistros(), carregarTecnicos()]);
		} catch {
			// toast de erro já exibido por apiFetch
		}
	}

	async function excluirRegistro(r: Registro) {
		if (!confirm(`Excluir o registro de ${r.tecnico_nome} de ${dataHora(r.entrada)}?`)) return;
		try {
			await apiFetch(`/api/tecnicos/registros/${r.id}`, { method: 'DELETE' });
			await Promise.all([carregarRegistros(), carregarTecnicos()]);
		} catch {
			// toast de erro já exibido por apiFetch
		}
	}

	onMount(() => {
		const timer = setInterval(() => (agora = Date.now()), 30_000);
		return () => clearInterval(timer);
	});

	// Recarrega só ao trocar de aba; filtros e paginação disparam suas próprias cargas
	$effect(() => {
		const aba = abaAtiva;
		untrack(() => {
			if (aba === 'presenca') carregarTecnicos();
			else if (aba === 'registros') carregarRegistros(true);
			else carregarRelatorio();
		});
	});
</script>

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Técnicos</h1>
			<p class="page-sub">Marque a entrada e a saída dos técnicos e acompanhe as horas no período.</p>
		</div>
		<button onclick={abrirCriarTecnico} class="btn btn-primary">
			<Plus class="size-4" />
			<span>Cadastrar técnico</span>
		</button>
	</div>

	<div class="segmented" role="group" aria-label="Seção">
		<button aria-pressed={abaAtiva === 'presenca'} onclick={() => (abaAtiva = 'presenca')}>
			<UserCheck class="size-4" />
			<span>Presença</span>
		</button>
		<button aria-pressed={abaAtiva === 'registros'} onclick={() => (abaAtiva = 'registros')}>
			<History class="size-4" />
			<span>Registros</span>
		</button>
		<button aria-pressed={abaAtiva === 'relatorio'} onclick={() => (abaAtiva = 'relatorio')}>
			<BarChart3 class="size-4" />
			<span>Relatório</span>
		</button>
	</div>

	{#if abaAtiva === 'presenca'}
		<div class="flex flex-wrap items-center justify-between gap-3">
			<p class="text-sm text-ink-2">
				<strong class="text-ink tabular">{presentes}</strong>
				{presentes === 1 ? 'técnico no local agora' : 'técnicos no local agora'}
			</p>
			<label class="flex items-center gap-2 text-sm text-ink-2 cursor-pointer">
				<input type="checkbox" bind:checked={mostrarInativos} class="check" />
				Mostrar desativados
			</label>
		</div>

		{#if loading}
			<div class="flex justify-center py-20"><div class="spinner"></div></div>
		{:else if tecnicosVisiveis.length === 0}
			<div class="panel empty">
				<HardHat class="size-9 text-ink-3" strokeWidth={1.5} />
				<h3 class="empty-title">Nenhum técnico cadastrado</h3>
				<p class="empty-text">Cadastre os técnicos pelo nome para começar a marcar entradas e saídas.</p>
			</div>
		{:else}
			<ul class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
				{#each tecnicosVisiveis as t (t.id)}
					{@const noLocal = !!t.entrada_aberta}
					<li class="panel p-4 flex flex-col gap-3 {t.ativo ? '' : 'opacity-60'}">
						<div class="flex items-start justify-between gap-2">
							<div class="min-w-0">
								<div class="font-semibold text-ink truncate">{t.nome}</div>
								<div class="text-[13px] text-ink-3 truncate">{t.empresa || 'Sem empresa informada'}</div>
							</div>
							<button onclick={() => abrirEditarTecnico(t)} class="icon-btn -mr-1.5 -mt-1" title="Editar técnico" aria-label="Editar técnico">
								<Edit3 class="size-4" />
							</button>
						</div>

						<div class="text-sm">
							{#if !t.ativo}
								<span class="tag">Desativado</span>
							{:else if noLocal && t.entrada_aberta}
								<span class="tag tag-ok"><Clock class="size-3.5" /> No local desde {hora(t.entrada_aberta)}</span>
								<span class="ml-1 text-[13px] text-ink-3 tabular">
									há {duracao(Math.max(0, (agora - new Date(t.entrada_aberta).getTime()) / 1000))}
								</span>
							{:else}
								<span class="tag">Fora do local</span>
							{/if}
						</div>

						{#if noLocal && t.registro_aberto_id}
							{@const regId = t.registro_aberto_id}
							<button
								onclick={() => abrirAtividades(regId, t.nome, t.atividades_abertas)}
								class="group text-left rounded-lg border border-dashed border-line-strong hover:border-accent/50 hover:bg-accent-soft/40 px-3 py-2 transition-colors cursor-pointer"
								title="Anotar o que o técnico fez hoje"
							>
								{#if t.atividades_abertas}
									<span class="flex items-center gap-1.5 text-xs font-semibold text-ink-3 mb-0.5">
										<NotebookPen class="size-3.5" /> O que fez hoje
									</span>
									<span class="block text-[13px] text-ink-2 whitespace-pre-line line-clamp-3 break-words">{t.atividades_abertas}</span>
								{:else}
									<span class="flex items-center gap-1.5 text-[13px] text-ink-3 group-hover:text-accent">
										<NotebookPen class="size-4" /> Anotar o que ele fez hoje
									</span>
								{/if}
							</button>
						{/if}

						{#if t.ativo || noLocal}
							<div class="flex gap-2 mt-auto">
								<button
									onclick={() => (noLocal ? abrirMarcacao(t) : marcar(t))}
									class="btn flex-1 {noLocal ? 'btn-secondary' : 'btn-primary'}"
								>
									{#if noLocal}
										<LogOut class="size-4" /><span>Marcar saída</span>
									{:else}
										<LogIn class="size-4" /><span>Marcar entrada</span>
									{/if}
								</button>
								<button
									onclick={() => abrirMarcacao(t)}
									class="btn btn-ghost"
									title="Informar outro horário ou observação"
									aria-label="Informar outro horário ou observação"
								>
									<Clock class="size-4" />
								</button>
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	{:else}
		<!-- Filtros de Registros e Relatório -->
		<div class="flex flex-wrap items-center gap-3">
			<select bind:value={filtroTecnico} onchange={recarregarAba} class="field sm:w-60" aria-label="Técnico">
				<option value="">Todos os técnicos</option>
				{#each tecnicos as t}
					<option value={t.id}>{t.nome}</option>
				{/each}
			</select>
			<input type="date" bind:value={filtroInicio} onchange={recarregarAba} class="field w-auto" aria-label="De" />
			<span class="text-sm text-ink-3">até</span>
			<input type="date" bind:value={filtroFim} onchange={recarregarAba} class="field w-auto" aria-label="Até" />
			<button onclick={exportarCSV} class="btn btn-secondary sm:ml-auto" title="Exporta os dados filtrados">
				<Download class="size-4" />
				<span>CSV</span>
			</button>
		</div>

		{#if loading}
			<div class="flex justify-center py-20"><div class="spinner"></div></div>
		{:else if abaAtiva === 'registros'}
			{#if registros.length === 0}
				<div class="panel empty">
					<History class="size-9 text-ink-3" strokeWidth={1.5} />
					<h3 class="empty-title">Nenhum registro no período</h3>
					<p class="empty-text">As entradas e saídas marcadas aparecem aqui.</p>
				</div>
			{:else}
				<div class="panel overflow-hidden">
					<div class="overflow-x-auto">
						<table class="data-table">
							<thead>
								<tr>
									<th>Técnico</th>
									<th>Entrada</th>
									<th>Saída</th>
									<th class="text-right">Duração</th>
									<th>O que fez</th>
									<th>Observação</th>
									<th>Marcado por</th>
									{#if auth.ehAdmin}
										<th class="text-right"><span class="sr-only">Ações</span></th>
									{/if}
								</tr>
							</thead>
							<tbody>
								{#each registros as r (r.id)}
									<tr>
										<td>
											<div class="font-semibold text-ink">{r.tecnico_nome}</div>
											{#if r.tecnico_empresa}<div class="text-[13px] text-ink-3">{r.tecnico_empresa}</div>{/if}
										</td>
										<td class="whitespace-nowrap tabular">{dataHora(r.entrada)}</td>
										<td class="whitespace-nowrap tabular">
											{#if r.saida}
												{mesmoDia(r.entrada, r.saida) ? hora(r.saida) : dataHora(r.saida)}
											{:else}
												<span class="tag tag-ok">No local</span>
											{/if}
										</td>
										<td class="text-right whitespace-nowrap font-semibold text-ink tabular">
											{r.duracao_segundos !== null ? duracao(r.duracao_segundos) : '—'}
										</td>
										<td class="max-w-xs">
											<button
												onclick={() => abrirAtividades(r.id, `${r.tecnico_nome} · ${dataHora(r.entrada)}`, r.atividades)}
												class="block w-full text-left truncate cursor-pointer hover:text-accent {r.atividades ? 'text-ink' : 'text-ink-3'}"
												title={r.atividades || 'Anotar o que foi feito'}
											>
												{#if r.atividades}{r.atividades}{:else}<span class="inline-flex items-center gap-1"><NotebookPen class="size-3.5" /> anotar</span>{/if}
											</button>
										</td>
										<td class="text-ink-2 max-w-40 truncate" title={r.observacao}>{r.observacao || '—'}</td>
										<td class="text-[13px] text-ink-2 whitespace-nowrap">
											{r.entrada_registrada_por_nome}{#if r.saida_registrada_por_nome && r.saida_registrada_por_nome !== r.entrada_registrada_por_nome}
												/ {r.saida_registrada_por_nome}{/if}
										</td>
										{#if auth.ehAdmin}
											<td class="text-right whitespace-nowrap">
												<button onclick={() => abrirEditarRegistro(r)} class="icon-btn" title="Corrigir horários" aria-label="Corrigir horários">
													<Edit3 class="size-4" />
												</button>
												<button onclick={() => excluirRegistro(r)} class="icon-btn icon-btn-danger" title="Excluir registro" aria-label="Excluir registro">
													<Trash2 class="size-4" />
												</button>
											</td>
										{/if}
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</div>

				<div class="flex items-center justify-between text-[13px] text-ink-3">
					<span class="tabular">{totalRegistros} {totalRegistros === 1 ? 'registro' : 'registros'}</span>
					<div class="flex items-center gap-1">
						<button
							disabled={paginaAtual <= 1}
							onclick={() => { paginaAtual--; carregarRegistros(); }}
							class="icon-btn disabled:opacity-30 disabled:pointer-events-none"
							aria-label="Página anterior"
						>
							<ChevronLeft class="size-4" />
						</button>
						<span class="px-2 font-semibold text-ink-2 tabular">{paginaAtual} de {totalPaginas}</span>
						<button
							disabled={paginaAtual >= totalPaginas}
							onclick={() => { paginaAtual++; carregarRegistros(); }}
							class="icon-btn disabled:opacity-30 disabled:pointer-events-none"
							aria-label="Próxima página"
						>
							<ChevronRight class="size-4" />
						</button>
					</div>
				</div>
			{/if}
		{:else if relatorio.length === 0}
			<div class="panel empty">
				<BarChart3 class="size-9 text-ink-3" strokeWidth={1.5} />
				<h3 class="empty-title">Sem dados no período</h3>
				<p class="empty-text">Ajuste o período ou o técnico para ver o consolidado de horas.</p>
			</div>
		{:else}
			<div class="panel overflow-hidden">
				<div class="overflow-x-auto">
					<table class="data-table">
						<thead>
							<tr>
								<th>Técnico</th>
								<th class="text-right">Dias</th>
								<th class="text-right">Registros</th>
								<th class="text-right">Horas totais</th>
								<th>Primeira entrada</th>
								<th>Última marcação</th>
							</tr>
						</thead>
						<tbody>
							{#each relatorio as l (l.tecnico_id)}
								<tr>
									<td>
										<div class="font-semibold text-ink">{l.tecnico_nome}</div>
										{#if l.tecnico_empresa}<div class="text-[13px] text-ink-3">{l.tecnico_empresa}</div>{/if}
									</td>
									<td class="text-right tabular">{l.dias_presentes}</td>
									<td class="text-right tabular whitespace-nowrap">
										{l.total_registros}
										{#if l.registros_abertos > 0}
											<span class="tag tag-warn ml-1" title="Registros sem saída não somam horas">{l.registros_abertos} em aberto</span>
										{/if}
									</td>
									<td class="text-right font-bold text-ink tabular whitespace-nowrap">{duracao(l.segundos_totais)}</td>
									<td class="text-ink-2 whitespace-nowrap tabular">{l.primeira_entrada ? dataHora(l.primeira_entrada) : '—'}</td>
									<td class="text-ink-2 whitespace-nowrap tabular">{l.ultima_marcacao ? dataHora(l.ultima_marcacao) : '—'}</td>
								</tr>
							{/each}
						</tbody>
						{#if relatorio.length > 1}
							<tfoot>
								<tr>
									<td class="font-semibold text-ink">Total</td>
									<td></td>
									<td class="text-right tabular">{relatorio.reduce((s, l) => s + l.total_registros, 0)}</td>
									<td class="text-right font-bold text-ink tabular whitespace-nowrap">{duracao(totalSegundosRelatorio)}</td>
									<td></td>
									<td></td>
								</tr>
							</tfoot>
						{/if}
					</table>
				</div>
			</div>
			<p class="hint">O período considera a data de entrada. Registros ainda sem saída contam como visita, mas não somam horas.</p>
		{/if}
	{/if}

	<!-- Modal marcação com horário -->
	{#if modalMarcacaoAberto && marcacaoTecnico}
		{@const saida = !!marcacaoTecnico.entrada_aberta}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div>
						<h3 class="modal-title">{saida ? 'Marcar saída' : 'Marcar entrada'}</h3>
						<p class="mt-0.5 text-sm text-ink-3">{marcacaoTecnico.nome}</p>
					</div>
					<button onclick={() => (modalMarcacaoAberto = false)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>
				<div class="modal-body">
					{#if saida && marcacaoTecnico.entrada_aberta}
						<p class="text-sm text-ink-2">Entrada registrada em <strong class="text-ink">{dataHora(marcacaoTecnico.entrada_aberta)}</strong>.</p>
					{/if}
					<div>
						<label class="label" for="marc-horario">Horário da {saida ? 'saída' : 'entrada'}</label>
						<input id="marc-horario" type="datetime-local" bind:value={marcacaoHorario} max={paraInputLocal(new Date())} class="field" />
					</div>
					{#if saida}
						<div>
							<label class="label" for="marc-atividades">O que ele fez hoje? <span class="font-normal text-ink-3">(opcional)</span></label>
							<textarea
								id="marc-atividades"
								bind:value={marcacaoAtividades}
								rows="5"
								maxlength="4000"
								placeholder={'Ex.: Troca do switch do rack 2\nOrganização do cabeamento do 3º andar'}
								class="field"
								autofocus
							></textarea>
						</div>
					{/if}
					<div>
						<label class="label" for="marc-obs">Observação <span class="font-normal text-ink-3">(opcional)</span></label>
						<input id="marc-obs" type="text" bind:value={marcacaoObs} placeholder={saida ? 'Ex.: Volta amanhã para terminar' : 'Ex.: Manutenção do ar-condicionado'} class="field" />
					</div>
				</div>
				<div class="modal-foot">
					<button onclick={() => (modalMarcacaoAberto = false)} class="btn btn-ghost">Cancelar</button>
					<button
						onclick={() => marcacaoTecnico && marcar(marcacaoTecnico, marcacaoHorario, marcacaoObs, marcacaoAtividades)}
						class="btn btn-primary"
					>
						Registrar {saida ? 'saída' : 'entrada'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal "o que foi feito" -->
	{#if modalAtividadesAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div>
						<h3 class="modal-title">O que foi feito</h3>
						<p class="mt-0.5 text-sm text-ink-3">{atividadesTitulo}</p>
					</div>
					<button onclick={() => (modalAtividadesAberto = false)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>
				<div class="modal-body">
					<textarea
						bind:value={atividadesTexto}
						rows="8"
						maxlength="4000"
						placeholder="Serviços feitos, equipamentos trocados, pendências…"
						class="field"
						aria-label="O que foi feito"
						autofocus
					></textarea>
					<p class="hint -mt-2 text-right tabular">{atividadesTexto.length}/4000</p>
				</div>
				<div class="modal-foot">
					<button onclick={() => (modalAtividadesAberto = false)} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarAtividades} class="btn btn-primary" disabled={atividadesSalvando}>Salvar</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal cadastro de técnico -->
	{#if modalTecnicoAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{formTecId ? 'Editar técnico' : 'Cadastrar técnico'}</h3>
					<button onclick={() => (modalTecnicoAberto = false)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>
				<div class="modal-body">
					<div>
						<label class="label" for="tec-nome">Nome</label>
						<input id="tec-nome" type="text" bind:value={formTecNome} placeholder="Nome completo do técnico" class="field" />
					</div>
					<div>
						<label class="label" for="tec-empresa">Empresa <span class="font-normal text-ink-3">(opcional)</span></label>
						<input id="tec-empresa" type="text" bind:value={formTecEmpresa} placeholder="Ex.: Prestadora de serviços" class="field" />
					</div>
					{#if formTecId}
						<label class="flex items-center gap-2.5 text-sm text-ink cursor-pointer">
							<input type="checkbox" bind:checked={formTecAtivo} class="check" />
							Técnico ativo
						</label>
					{/if}
				</div>
				<div class="modal-foot">
					<button onclick={() => (modalTecnicoAberto = false)} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarTecnico} class="btn btn-primary">{formTecId ? 'Salvar alterações' : 'Cadastrar técnico'}</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal correção de registro (admin) -->
	{#if modalRegistroAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div>
						<h3 class="modal-title">Corrigir registro</h3>
						<p class="mt-0.5 text-sm text-ink-3">{formRegNome}</p>
					</div>
					<button onclick={() => (modalRegistroAberto = false)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>
				<div class="modal-body">
					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div>
							<label class="label" for="reg-entrada">Entrada</label>
							<input id="reg-entrada" type="datetime-local" bind:value={formRegEntrada} class="field" />
						</div>
						<div>
							<label class="label" for="reg-saida">Saída</label>
							<input id="reg-saida" type="datetime-local" bind:value={formRegSaida} class="field" />
							<p class="hint">Deixe vazio se o técnico ainda está no local.</p>
						</div>
					</div>
					<div>
						<label class="label" for="reg-obs">Observação</label>
						<input id="reg-obs" type="text" bind:value={formRegObs} class="field" />
					</div>
				</div>
				<div class="modal-foot">
					<button onclick={() => (modalRegistroAberto = false)} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarRegistro} class="btn btn-primary">Salvar correção</button>
				</div>
			</div>
		</div>
	{/if}
</div>
