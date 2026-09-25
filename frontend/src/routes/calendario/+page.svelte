<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
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
		Repeat
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

	let ecRef = $state<any>(null);
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
		select: (info: any) => {
			// No dayGrid o fim da seleção é exclusivo (dia seguinte ao último marcado)
			abrirCriarComDatas(toDateInput(info.start), toDateInput(somarDias(info.end, -1)));
		},
		eventClick: (info: any) => {
			const ev = eventos.find(e => e.id === info.event.id);
			if (ev) {
				abrirDetalhes(ev);
			}
		},
		eventDrop: async (info: any) => {
			await persistirMudancaDias(info.event);
		},
		eventResize: async (info: any) => {
			await persistirMudancaDias(info.event);
		}
	});

	async function carregarEventos() {
		loading = true;
		try {
			const res = await apiFetch<EventoItem[]>(`/api/eventos?meus=${filtroMeus}`);
			eventos = res;

			options.events = res.map(e => ({
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
		} catch (err) {
			console.error('Erro ao carregar eventos:', err);
		} finally {
			loading = false;
		}
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
		carregarEventos();
	});

	$effect(() => {
		if (filtroMeus !== undefined) {
			carregarEventos();
		}
	});
</script>

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Calendário</h1>
			<p class="page-sub">Marcações da equipe por dia. Arraste um evento para mudar o dia.</p>
		</div>

		<div class="flex items-center gap-2">
			<div class="segmented" role="group" aria-label="Filtro">
				<button aria-pressed={!filtroMeus} onclick={() => filtroMeus = false}>Todos</button>
				<button aria-pressed={filtroMeus} onclick={() => filtroMeus = true}>Os meus</button>
			</div>
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
</div>
