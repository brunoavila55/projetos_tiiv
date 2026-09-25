<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { toast } from '$lib/toast.svelte';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { Calendar, DayGrid, Interaction, List } from '@event-calendar/core';
	import { 
		Plus, 
		CalendarDays,
		Users, 
		X, 
		Trash2, 
		Edit3, 
		Check,
		Repeat,
		ShieldCheck,
		ArrowRight
	} from 'lucide-svelte';

	interface EventoItem {
		id: string;
		original_id?: string;
		titulo: string;
		descricao: string;
		inicio: string;
		fim: string;
		dia_inteiro: boolean;
		recorrencia?: 'nenhuma' | 'semanal' | 'mensal';
		recorrencia_fim?: string | null;
		criado_por: string;
		criador_nome: string;
		criador_cor: string;
		participantes: { id: string; nome: string; cor: string }[];
		pode_editar: boolean;
	}

	type TipoPlantao = 'plantao' | 'sobreaviso';

	interface Plantao {
		id: string;
		usuario_id: string;
		usuario_nome: string;
		usuario_cor: string;
		tipo: TipoPlantao;
		inicio: string; // AAAA-MM-DD
		fim: string; // AAAA-MM-DD, inclusivo
		observacao: string;
	}

	const TIPO_PLANTAO: Record<TipoPlantao, string> = { plantao: 'Plantão', sobreaviso: 'Sobreaviso' };
	const PREFIXO_PLANTAO = 'plantao:';

	let plantoes = $state<Plantao[]>([]);
	let mostrarEscala = $state(true);
	// Faixa visível no calendário (atualizada pelo datesSet)
	let faixa = $state<{ inicio: Date; fim: Date } | null>(null);
	const ehAdmin = $derived(auth.user?.papel === 'admin');
	let eventos = $state<EventoItem[]>([]);
	let usuarios = $state<UsuarioPublico[]>([]);
	let filtroMeus = $state<boolean>(false);
	let loading = $state<boolean>(true);

	// Modal de criação / edição de evento
	let modalAberto = $state<boolean>(false);
	let modalDetalhesAberto = $state<boolean>(false);
	let eventoSelecionado = $state<EventoItem | null>(null);

	// Campos do formulário
	let formId = $state<string | null>(null);
	let formTitulo = $state<string>('');
	let formDescricao = $state<string>('');
	let formInicio = $state<string>('');
	let formFim = $state<string>('');
	let formRecorrencia = $state<'nenhuma' | 'semanal' | 'mensal'>('nenhuma');
	let formRecorrenciaFim = $state<string>('');
	let formParticipantes = $state<string[]>([]);

	let plugins = [DayGrid, Interaction, List];
	let options = $state({
		view: 'dayGridMonth',
		headerToolbar: {
			start: 'prev,next today',
			center: 'title',
			end: 'dayGridMonth,listMonth'
		},
		buttonText: {
			today: 'Hoje',
			month: 'Mês',
			week: 'Semana',
			day: 'Dia',
			list: 'Lista',
			dayGridMonth: 'Mês',
			timeGridWeek: 'Semana',
			timeGridDay: 'Dia',
			listMonth: 'Lista'
		},
		locale: 'pt-br',
		firstDay: 0 as const,
		selectable: true,
		editable: true,
		events: [] as any[],
		// Plantões aparecem antes das marcações em cada dia
		eventOrder: (a: any, b: any) =>
			(b.extendedProps?.plantao ? 1 : 0) - (a.extendedProps?.plantao ? 1 : 0) || a.start - b.start,
		datesSet: (info: any) => {
			faixa = { inicio: info.start, fim: info.end };
		},
		select: (info: any) => {
			// No dayGrid o fim da seleção é exclusivo (dia seguinte ao último marcado)
			abrirCriarComDatas(toDateInput(info.start), toDateInput(somarDias(info.end, -1)));
		},
		eventClick: (info: any) => {
			if (String(info.event.id).startsWith(PREFIXO_PLANTAO)) {
				const p = plantoes.find((x) => PREFIXO_PLANTAO + x.id === info.event.id);
				if (p) plantaoSelecionado = p;
				return;
			}
			const ev = eventos.find(e => e.id === info.event.id);
			if (ev) {
				abrirDetalhes(ev);
			}
		},
		eventDrop: async (info: any) => {
			// A escala só muda pelo formulário de plantão
			if (String(info.event.id).startsWith(PREFIXO_PLANTAO)) return info.revert();
			await persistirMudancaDias(info.event);
		},
		eventResize: async (info: any) => {
			if (String(info.event.id).startsWith(PREFIXO_PLANTAO)) return info.revert();
			await persistirMudancaDias(info.event);
		}
	});

	async function carregarEventos() {
		if (!faixa) return;
		loading = true;
		const params = new URLSearchParams({
			meus: String(filtroMeus),
			inicio: faixa.inicio.toISOString(),
			fim: faixa.fim.toISOString()
		});
		// O fim da faixa é exclusivo; a escala usa dias inclusivos
		const escalaParams = new URLSearchParams({
			inicio: toDateInput(faixa.inicio),
			fim: toDateInput(somarDias(faixa.fim, -1))
		});
		try {
			const [res, escala] = await Promise.all([
				apiFetch<EventoItem[]>(`/api/eventos?${params}`),
				apiFetch<Plantao[]>(`/api/plantoes?${escalaParams}`)
			]);
			eventos = res;
			plantoes = escala;
			montarEventosCalendario();
		} catch (err) {
			console.error('Erro ao carregar eventos:', err);
		} finally {
			loading = false;
		}
	}

	function montarEventosCalendario() {
		const marcacoes = eventos.map(e => ({
			id: e.id,
			title: e.recorrencia && e.recorrencia !== 'nenhuma' ? `↻ ${e.titulo}` : e.titulo,
			// Marcações são por dia: o fim exibido é exclusivo (dia seguinte ao último)
			start: toDateInput(new Date(e.inicio)),
			end: toDateInput(somarDias(new Date(e.fim), 1)),
			allDay: true,
			backgroundColor: e.criador_cor || '#1d5bbf',
			borderColor: e.criador_cor || '#1d5bbf',
			// Arrastar uma ocorrência moveria a série inteira; recorrentes só pelo formulário
			editable: e.pode_editar && (!e.recorrencia || e.recorrencia === 'nenhuma'),
			extendedProps: {
				descricao: e.descricao,
				criadorNome: e.criador_nome,
				participantes: e.participantes,
				recorrencia: e.recorrencia
			}
		}));

		const escala = !mostrarEscala
			? []
			: plantoes
					.filter((p) => !filtroMeus || p.usuario_id === auth.user?.id)
					.map((p) => ({
						id: PREFIXO_PLANTAO + p.id,
						title: `${TIPO_PLANTAO[p.tipo]} · ${p.usuario_nome}`,
						start: p.inicio,
						end: toDateInput(somarDias(diaLocal(p.fim), 1)),
						allDay: true,
						editable: false,
						// Plantão: faixa listrada na cor da pessoa; sobreaviso: só contorno tracejado
						classNames: [p.tipo === 'plantao' ? 'ec-plantao' : 'ec-sobreaviso'],
						backgroundColor: p.tipo === 'plantao' ? p.usuario_cor || '#1d5bbf' : 'transparent',
						// Cor da pessoa no contorno tracejado do sobreaviso
						styles: [`--cor-escala: ${p.usuario_cor || '#1d5bbf'}`],
						extendedProps: { plantao: true }
					}));

		options.events = [...escala, ...marcacoes];
	}

	// "AAAA-MM-DD" como meia-noite local (new Date("AAAA-MM-DD") seria UTC)
	function diaLocal(dia: string): Date {
		return new Date(dia + 'T00:00:00');
	}

	async function carregarUsuarios() {
		try {
			usuarios = await apiFetch<UsuarioPublico[]>('/api/auth/usuarios');
		} catch (err) {
			console.error('Erro ao carregar usuários:', err);
		}
	}

	function abrirCriarComDatas(inicioDia: string, fimDia: string) {
		formId = null;
		formTitulo = '';
		formDescricao = '';
		formRecorrencia = 'nenhuma';
		formRecorrenciaFim = '';
		formInicio = inicioDia;
		formFim = fimDia < inicioDia ? inicioDia : fimDia;

		formParticipantes = auth.user ? [auth.user.id] : [];
		modalAberto = true;
	}

	function abrirCriarManual() {
		const hoje = toDateInput(new Date());
		abrirCriarComDatas(hoje, hoje);
	}

	function abrirDetalhes(e: EventoItem) {
		eventoSelecionado = e;
		modalDetalhesAberto = true;
	}

	function editarEventoSelecionado() {
		if (!eventoSelecionado) return;
		formId = eventoSelecionado.original_id || eventoSelecionado.id;
		formTitulo = eventoSelecionado.titulo;
		formDescricao = eventoSelecionado.descricao;
		formInicio = toDateInput(new Date(eventoSelecionado.inicio));
		formFim = toDateInput(new Date(eventoSelecionado.fim));
		formRecorrencia = eventoSelecionado.recorrencia || 'nenhuma';
		formRecorrenciaFim = eventoSelecionado.recorrencia_fim ? eventoSelecionado.recorrencia_fim.split('T')[0] : '';
		formParticipantes = eventoSelecionado.participantes.map(p => p.id);

		modalDetalhesAberto = false;
		modalAberto = true;
	}

	async function salvarEvento() {
		if (!formTitulo || !formInicio || !formFim) {
			alert('Título, dia de início e dia de término são obrigatórios.');
			return;
		}

		if (formFim < formInicio) {
			alert('O dia de término não pode ser anterior ao de início.');
			return;
		}

		const payload: any = {
			titulo: formTitulo,
			descricao: formDescricao,
			...intervaloDias(formInicio, formFim),
			recorrencia: formRecorrencia,
			participantes: formParticipantes
		};

		if (formRecorrencia !== 'nenhuma' && formRecorrenciaFim) {
			payload.recorrencia_fim = new Date(formRecorrenciaFim + 'T23:59:59').toISOString();
		}

		try {
			if (formId) {
				await apiFetch(`/api/eventos/${formId}`, {
					method: 'PUT',
					body: JSON.stringify(payload)
				});
			} else {
				await apiFetch('/api/eventos', {
					method: 'POST',
					body: JSON.stringify(payload)
				});
			}
			modalAberto = false;
			await carregarEventos();
		} catch (err: any) {
			alert(err.message || 'Erro ao salvar evento');
		}
	}

	async function excluirEvento() {
		if (!eventoSelecionado || !confirm('Deseja realmente excluir este evento?')) return;
		const targetId = eventoSelecionado.original_id || eventoSelecionado.id;
		try {
			await apiFetch(`/api/eventos/${targetId}`, { method: 'DELETE' });
			modalDetalhesAberto = false;
			await carregarEventos();
		} catch (err: any) {
			alert(err.message || 'Erro ao excluir evento');
		}
	}

	async function persistirMudancaDias(calEvent: any) {
		const ev = eventos.find(e => e.id === calEvent.id);
		const inicioDia = toDateInput(new Date(calEvent.start));
		// O fim do calendário é exclusivo: o último dia marcado é o anterior
		const fimDia = calEvent.end ? toDateInput(somarDias(new Date(calEvent.end), -1)) : inicioDia;
		try {
			await apiFetch(`/api/eventos/${ev?.original_id || calEvent.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					titulo: ev?.titulo ?? calEvent.title,
					descricao: ev?.descricao ?? '',
					...intervaloDias(inicioDia, fimDia < inicioDia ? inicioDia : fimDia),
					recorrencia: ev?.recorrencia ?? 'nenhuma',
					recorrencia_fim: ev?.recorrencia_fim ?? null
				})
			});
			await carregarEventos();
		} catch (err: any) {
			alert(err.message || 'Erro ao mudar o dia do evento');
			await carregarEventos(); // reverte visualmente em caso de erro
		}
	}

	// Marcação por dia: do início do primeiro dia ao fim do último, no fuso local
	function intervaloDias(inicioDia: string, fimDia: string) {
		return {
			inicio: new Date(inicioDia + 'T00:00:00').toISOString(),
			fim: new Date(fimDia + 'T23:59:59').toISOString(),
			dia_inteiro: true
		};
	}

	function toDateInput(d: Date): string {
		const pad = (n: number) => n.toString().padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
	}

	function somarDias(d: Date, dias: number): Date {
		const r = new Date(d);
		r.setDate(r.getDate() + dias);
		return r;
	}

	function mesmoDia(a: string, b: string): boolean {
		return new Date(a).toDateString() === new Date(b).toDateString();
	}

	function alternarParticipante(uId: string) {
		if (formParticipantes.includes(uId)) {
			// Não permite remover o criador
			if (uId === auth.user?.id) return;
			formParticipantes = formParticipantes.filter(id => id !== uId);
		} else {
			formParticipantes = [...formParticipantes, uId];
		}
	}

	onMount(() => {
		carregarUsuarios();
	});

	// Recarrega ao navegar entre meses (faixa) ou trocar o filtro
	$effect(() => {
		if (faixa && filtroMeus !== undefined) {
			carregarEventos();
		}
	});

	// ---------- Escala de plantão ----------
	let plantaoSelecionado = $state<Plantao | null>(null);
	let escalaModalAberto = $state(false);
	let escalaModo = $state<'avulso' | 'rodizio'>('avulso');
	let escalaSalvando = $state(false);
	let escalaEditId = $state<string | null>(null);
	let eUsuario = $state('');
	let eTipo = $state<TipoPlantao>('plantao');
	let eInicio = $state('');
	let eFim = $state('');
	let eObs = $state('');
	// Rodízio
	let rPessoas = $state<string[]>([]);
	let rDias = $state(7);
	let rTurnos = $state(8);

	const nomeUsuario = (id: string) => usuarios.find((u) => u.id === id)?.nome ?? '—';

	// Prévia dos primeiros turnos do rodízio, igual ao que o servidor gera
	const previaRodizio = $derived.by(() => {
		if (!eInicio || rPessoas.length === 0 || rDias < 1 || rTurnos < 1) return [];
		return Array.from({ length: Math.min(rTurnos, 6) }, (_, i) => {
			const ini = somarDias(diaLocal(eInicio), i * rDias);
			return { pessoa: nomeUsuario(rPessoas[i % rPessoas.length]), inicio: ini, fim: somarDias(ini, rDias - 1) };
		});
	});

	function abrirEscala(modo: 'avulso' | 'rodizio' = 'avulso') {
		escalaEditId = null;
		escalaModo = modo;
		eUsuario = '';
		eTipo = 'plantao';
		eInicio = toDateInput(new Date());
		eFim = eInicio;
		eObs = '';
		rPessoas = [];
		rDias = 7;
		rTurnos = 8;
		escalaModalAberto = true;
	}

	function editarPlantao(p: Plantao) {
		escalaEditId = p.id;
		escalaModo = 'avulso';
		eUsuario = p.usuario_id;
		eTipo = p.tipo;
		eInicio = p.inicio;
		eFim = p.fim;
		eObs = p.observacao;
		plantaoSelecionado = null;
		escalaModalAberto = true;
	}

	function alternarPessoaRodizio(id: string) {
		rPessoas = rPessoas.includes(id) ? rPessoas.filter((x) => x !== id) : [...rPessoas, id];
	}

	async function salvarEscala() {
		escalaSalvando = true;
		try {
			if (escalaModo === 'rodizio') {
				if (rPessoas.length === 0) {
					alert('Escolha quem entra no rodízio.');
					return;
				}
				const res = await apiFetch<{ criados: number }>('/api/plantoes/rodizio', {
					method: 'POST',
					body: JSON.stringify({
						usuarios: rPessoas,
						tipo: eTipo,
						inicio: eInicio,
						dias_por_turno: Number(rDias),
						turnos: Number(rTurnos),
						observacao: eObs.trim()
					})
				});
				toast.success(`${res.criados} turnos adicionados à escala`);
			} else {
				if (!eUsuario) {
					alert('Escolha quem fica de plantão.');
					return;
				}
				await apiFetch(escalaEditId ? `/api/plantoes/${escalaEditId}` : '/api/plantoes', {
					method: escalaEditId ? 'PUT' : 'POST',
					body: JSON.stringify({
						usuario_id: eUsuario,
						tipo: eTipo,
						inicio: eInicio,
						fim: eFim < eInicio ? eInicio : eFim,
						observacao: eObs.trim()
					})
				});
			}
			escalaModalAberto = false;
			await carregarEventos();
		} catch {
			// apiFetch já mostrou o erro (ex.: conflito na escala)
		} finally {
			escalaSalvando = false;
		}
	}

	async function excluirPlantao(p: Plantao) {
		if (!confirm(`Tirar ${p.usuario_nome} da escala de ${periodo(p)}?`)) return;
		try {
			await apiFetch(`/api/plantoes/${p.id}`, { method: 'DELETE' });
			plantaoSelecionado = null;
			await carregarEventos();
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	function diaMes(d: Date): string {
		return d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' });
	}

	function periodo(p: { inicio: string; fim: string }): string {
		const ini = diaLocal(p.inicio);
		const fim = diaLocal(p.fim);
		const longo = (d: Date) => d.toLocaleDateString('pt-BR', { weekday: 'short', day: '2-digit', month: '2-digit' }).replace('.', '');
		return p.inicio === p.fim ? longo(ini) : `${longo(ini)} a ${longo(fim)}`;
	}
</script>

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Calendário</h1>
			<p class="page-sub">Marcações e escala de plantão da equipe. Arraste um evento para mudar o dia.</p>
		</div>

		<div class="flex flex-wrap lg:flex-nowrap lg:shrink-0 items-center gap-2">
			<div class="segmented shrink-0" role="group" aria-label="Filtro">
				<button aria-pressed={!filtroMeus} onclick={() => filtroMeus = false}>Todos</button>
				<button aria-pressed={filtroMeus} onclick={() => filtroMeus = true}>Os meus</button>
			</div>
			<button
				onclick={() => {
					mostrarEscala = !mostrarEscala;
					montarEventosCalendario();
				}}
				aria-pressed={mostrarEscala}
				class="btn {mostrarEscala ? 'btn-soft' : 'btn-ghost'}"
				title={mostrarEscala ? 'Esconder a escala de plantão' : 'Mostrar a escala de plantão'}
			>
				<ShieldCheck class="size-4" />
				<span class="hidden sm:inline">Escala</span>
			</button>
			{#if ehAdmin}
				<button onclick={() => abrirEscala()} class="btn btn-secondary">
					<Plus class="size-4" />
					<span class="hidden sm:inline">Plantão</span>
				</button>
			{/if}
			<button onclick={abrirCriarManual} class="btn btn-primary">
				<Plus class="size-4" />
				<span class="hidden sm:inline">Novo evento</span>
			</button>
		</div>
	</div>

	<div class="panel relative p-3 sm:p-5">
		{#if loading}
			<div class="absolute top-5 right-5 z-10"><div class="spinner size-5"></div></div>
		{/if}
		<div class="ec-theme-custom overflow-x-auto min-h-[600px]">
			<Calendar {plugins} {options} />
		</div>
	</div>

	<!-- Modal criar / editar -->
	{#if modalAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{formId ? 'Editar evento' : 'Novo evento'}</h3>
					<button onclick={() => modalAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div>
						<label class="label" for="ev-titulo">Título</label>
						<input id="ev-titulo" type="text" bind:value={formTitulo} placeholder="Ex.: Reunião de planejamento" class="field" autofocus />
					</div>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div>
							<label class="label" for="ev-inicio">Dia</label>
							<input id="ev-inicio" type="date" bind:value={formInicio} class="field" />
						</div>
						<div>
							<label class="label" for="ev-fim">Até <span class="font-normal text-ink-3">(se durar mais de um dia)</span></label>
							<input id="ev-fim" type="date" bind:value={formFim} min={formInicio} class="field" />
						</div>
					</div>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div>
							<label class="label" for="ev-rep">Repetição</label>
							<select id="ev-rep" bind:value={formRecorrencia} class="field">
								<option value="nenhuma">Não se repete</option>
								<option value="semanal">Toda semana</option>
								<option value="mensal">Todo mês, no mesmo dia</option>
							</select>
						</div>
						{#if formRecorrencia !== 'nenhuma'}
							<div>
								<label class="label" for="ev-ate">Repetir até <span class="font-normal text-ink-3">(opcional)</span></label>
								<input id="ev-ate" type="date" bind:value={formRecorrenciaFim} class="field" />
							</div>
						{/if}
					</div>

					<div>
						<label class="label" for="ev-desc">Descrição <span class="font-normal text-ink-3">(opcional)</span></label>
						<textarea id="ev-desc" bind:value={formDescricao} rows="2" placeholder="Pauta, local, link" class="field"></textarea>
					</div>

					<div>
						<span class="label">Participantes</span>
						<div class="flex flex-wrap gap-2 max-h-40 overflow-y-auto">
							{#each usuarios as u (u.id)}
								{@const selecionado = formParticipantes.includes(u.id)}
								{@const ehCriador = u.id === auth.user?.id}
								<button
									type="button"
									onclick={() => alternarParticipante(u.id)}
									aria-pressed={selecionado}
									class="inline-flex items-center gap-2 h-8 pl-2.5 pr-3 rounded-full text-[13px] font-semibold border cursor-pointer transition-colors {selecionado
										? 'bg-accent-soft border-accent/40 text-ink'
										: 'bg-surface border-line-strong text-ink-2 hover:bg-sunken'}"
								>
									{#if selecionado}
										<Check class="size-3.5 text-accent" strokeWidth={3} />
									{:else}
										<span class="dot" style="background-color: {u.cor || '#1d5bbf'};"></span>
									{/if}
									<span>{u.nome}</span>
									{#if ehCriador}
										<span class="font-normal text-ink-3">você</span>
									{/if}
								</button>
							{/each}
						</div>
					</div>
				</div>

				<div class="modal-foot">
					<button onclick={() => modalAberto = false} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarEvento} class="btn btn-primary">
						{formId ? 'Salvar alterações' : 'Criar evento'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal detalhes -->
	{#if modalDetalhesAberto && eventoSelecionado}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div class="flex gap-3 min-w-0">
						<span class="w-1 self-stretch rounded-full shrink-0" style="background-color: {eventoSelecionado.criador_cor};"></span>
						<div class="min-w-0">
							<h3 class="modal-title">{eventoSelecionado.titulo}</h3>
							<p class="mt-1 flex items-center gap-1.5 text-sm text-ink-2 tabular">
								<CalendarDays class="size-4 text-ink-3" />
								{new Date(eventoSelecionado.inicio).toLocaleDateString('pt-BR', { weekday: 'long', day: 'numeric', month: 'long' })}
								{#if !mesmoDia(eventoSelecionado.inicio, eventoSelecionado.fim)}
									até {new Date(eventoSelecionado.fim).toLocaleDateString('pt-BR', { weekday: 'long', day: 'numeric', month: 'long' })}
								{/if}
							</p>
							{#if eventoSelecionado.recorrencia && eventoSelecionado.recorrencia !== 'nenhuma'}
								<p class="mt-1 flex items-center gap-1.5 text-sm text-ink-2">
									<Repeat class="size-4 text-ink-3" />
									{eventoSelecionado.recorrencia === 'semanal' ? 'Repete toda semana' : 'Repete todo mês'}
								</p>
							{/if}
						</div>
					</div>
					<button onclick={() => modalDetalhesAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					{#if eventoSelecionado.descricao}
						<p class="text-[15px] text-ink whitespace-pre-wrap leading-relaxed">{eventoSelecionado.descricao}</p>
					{/if}

					<div>
						<h4 class="flex items-center gap-1.5 text-[13px] font-semibold text-ink-3 mb-2">
							<Users class="size-4" />
							{eventoSelecionado.participantes.length} {eventoSelecionado.participantes.length === 1 ? 'participante' : 'participantes'}
						</h4>
						<div class="flex flex-wrap gap-1.5">
							{#each eventoSelecionado.participantes as p}
								<span class="tag h-7 px-2.5 text-[13px]">
									<span class="dot size-2" style="background-color: {p.cor};"></span>
									{p.nome}
								</span>
							{/each}
						</div>
					</div>

					<p class="text-[13px] text-ink-3">Criado por {eventoSelecionado.criador_nome}</p>
				</div>

				<div class="modal-foot">
					{#if eventoSelecionado.pode_editar}
						<button onclick={excluirEvento} class="btn btn-danger mr-auto">
							<Trash2 class="size-4" />
							<span>Excluir</span>
						</button>
						<button onclick={editarEventoSelecionado} class="btn btn-primary">
							<Edit3 class="size-4" />
							<span>Editar</span>
						</button>
					{:else}
						<span class="mr-auto text-[13px] text-ink-3">Só o criador ou um administrador pode editar.</span>
						<button onclick={() => modalDetalhesAberto = false} class="btn btn-secondary">Fechar</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}
	<!-- Detalhes do plantão -->
	{#if plantaoSelecionado}
		{@const p = plantaoSelecionado}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div class="flex gap-3 min-w-0">
						<span class="w-1 self-stretch rounded-full shrink-0" style="background-color: {p.usuario_cor};"></span>
						<div class="min-w-0">
							<p class="text-[13px] font-semibold text-ink-3">{TIPO_PLANTAO[p.tipo]}</p>
							<h3 class="modal-title">{p.usuario_nome}</h3>
							<p class="mt-1 flex items-center gap-1.5 text-sm text-ink-2 tabular">
								<CalendarDays class="size-4 text-ink-3" />
								{periodo(p)}
							</p>
						</div>
					</div>
					<button onclick={() => (plantaoSelecionado = null)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>
				{#if p.observacao}
					<div class="modal-body">
						<p class="text-[15px] text-ink whitespace-pre-wrap">{p.observacao}</p>
					</div>
				{/if}
				<div class="modal-foot">
					{#if ehAdmin}
						<button onclick={() => excluirPlantao(p)} class="btn btn-danger mr-auto">
							<Trash2 class="size-4" />
							<span>Excluir</span>
						</button>
						<button onclick={() => editarPlantao(p)} class="btn btn-primary">
							<Edit3 class="size-4" />
							<span>Editar</span>
						</button>
					{:else}
						<span class="mr-auto text-[13px] text-ink-3">A escala é montada pelos administradores.</span>
						<button onclick={() => (plantaoSelecionado = null)} class="btn btn-secondary">Fechar</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}

	<!-- Montar escala (admin) -->
	{#if escalaModalAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{escalaEditId ? 'Editar turno' : 'Escala de plantão'}</h3>
					<button onclick={() => (escalaModalAberto = false)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					{#if !escalaEditId}
						<div class="segmented" role="group" aria-label="Modo">
							<button aria-pressed={escalaModo === 'avulso'} onclick={() => (escalaModo = 'avulso')}>Turno avulso</button>
							<button aria-pressed={escalaModo === 'rodizio'} onclick={() => (escalaModo = 'rodizio')}>
								<Repeat class="size-3.5" />
								Rodízio
							</button>
						</div>
					{/if}

					<div>
						<label class="label" for="es-tipo">Tipo</label>
						<select id="es-tipo" bind:value={eTipo} class="field">
							<option value="plantao">Plantão</option>
							<option value="sobreaviso">Sobreaviso</option>
						</select>
					</div>

					{#if escalaModo === 'avulso'}
						<div>
							<label class="label" for="es-pessoa">Quem</label>
							<select id="es-pessoa" bind:value={eUsuario} class="field">
								<option value="" disabled>Escolha o operador</option>
								{#each usuarios as u (u.id)}
									<option value={u.id}>{u.nome}</option>
								{/each}
							</select>
						</div>
						<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
							<div>
								<label class="label" for="es-ini">De</label>
								<input id="es-ini" type="date" bind:value={eInicio} class="field" />
							</div>
							<div>
								<label class="label" for="es-fim">Até <span class="font-normal text-ink-3">(inclusive)</span></label>
								<input id="es-fim" type="date" bind:value={eFim} min={eInicio} class="field" />
							</div>
						</div>
					{:else}
						<div>
							<span class="label">Quem entra, na ordem do rodízio</span>
							<div class="flex flex-wrap gap-2 max-h-40 overflow-y-auto">
								{#each usuarios as u (u.id)}
									{@const pos = rPessoas.indexOf(u.id)}
									<button
										type="button"
										onclick={() => alternarPessoaRodizio(u.id)}
										aria-pressed={pos >= 0}
										class="inline-flex items-center gap-2 h-8 pl-1.5 pr-3 rounded-full text-[13px] font-semibold border cursor-pointer transition-colors {pos >= 0
											? 'bg-accent-soft border-accent/40 text-ink'
											: 'bg-surface border-line-strong text-ink-2 hover:bg-sunken'}"
									>
										{#if pos >= 0}
											<span class="grid place-items-center size-5 rounded-full bg-accent text-on-accent text-[11px] font-bold tabular">{pos + 1}</span>
										{:else}
											<span class="dot ml-1" style="background-color: {u.cor || '#1d5bbf'};"></span>
										{/if}
										<span>{u.nome}</span>
									</button>
								{/each}
							</div>
						</div>
						<div class="grid grid-cols-3 gap-3">
							<div>
								<label class="label" for="es-rini">Começa em</label>
								<input id="es-rini" type="date" bind:value={eInicio} class="field" />
							</div>
							<div>
								<label class="label" for="es-rdias">Cada turno</label>
								<select id="es-rdias" bind:value={rDias} class="field">
									<option value={1}>1 dia</option>
									<option value={2}>2 dias</option>
									<option value={7}>1 semana</option>
									<option value={14}>2 semanas</option>
								</select>
							</div>
							<div>
								<label class="label" for="es-rturnos">Turnos</label>
								<input id="es-rturnos" type="number" min="1" max="104" bind:value={rTurnos} class="field tabular" />
							</div>
						</div>
						{#if previaRodizio.length}
							<div class="rounded-lg border border-line bg-sunken px-3.5 py-2.5 text-[13px]">
								<ul class="space-y-0.5">
									{#each previaRodizio as t}
										<li class="flex items-center gap-2 tabular">
											<span class="text-ink-3 w-28 shrink-0">{rDias === 1 ? diaMes(t.inicio) : `${diaMes(t.inicio)} a ${diaMes(t.fim)}`}</span>
											<ArrowRight class="size-3 text-ink-3" />
											<span class="font-semibold text-ink">{t.pessoa}</span>
										</li>
									{/each}
								</ul>
								{#if rTurnos > previaRodizio.length}
									<p class="mt-1 text-ink-3">e mais {rTurnos - previaRodizio.length} turnos, até {diaMes(somarDias(diaLocal(eInicio), rTurnos * rDias - 1))}</p>
								{/if}
							</div>
						{/if}
					{/if}

					<div>
						<label class="label" for="es-obs">Observação <span class="font-normal text-ink-3">(opcional)</span></label>
						<input id="es-obs" type="text" bind:value={eObs} maxlength="300" placeholder="Ex.: contato pelo celular da equipe" class="field" />
					</div>
				</div>

				<div class="modal-foot">
					<button onclick={() => (escalaModalAberto = false)} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarEscala} class="btn btn-primary" disabled={escalaSalvando}>
						{escalaEditId ? 'Salvar alterações' : escalaModo === 'rodizio' ? `Gerar ${rTurnos || 0} turnos` : 'Adicionar à escala'}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
