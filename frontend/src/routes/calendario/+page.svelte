<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { Calendar, TimeGrid, DayGrid, Interaction, List } from '@event-calendar/core';
	import { 
		Plus, 
		Calendar as CalendarIcon, 
		Clock, 
		Users, 
		X, 
		Trash2, 
		Edit3, 
		Check,
		Filter,
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
	let formDiaInteiro = $state<boolean>(false);
	let formRecorrencia = $state<'nenhuma' | 'semanal' | 'mensal'>('nenhuma');
	let formRecorrenciaFim = $state<string>('');
	let formParticipantes = $state<string[]>([]);

	let plugins = [TimeGrid, DayGrid, Interaction, List];
	let options = $state({
		view: 'timeGridWeek',
		headerToolbar: {
			start: 'prev,next today',
			center: 'title',
			end: 'dayGridMonth,timeGridWeek,timeGridDay'
		},
		buttonText: {
			today: 'Hoje',
			month: 'Mês',
			week: 'Semana',
			day: 'Dia',
			list: 'Lista'
		},
		locale: 'pt-br',
		firstDay: 0 as const,
		slotMinTime: '06:00:00',
		slotMaxTime: '22:00:00',
		allDaySlot: true,
		selectable: true,
		editable: true,
		events: [] as any[],
		select: (info: any) => {
			abrirCriarComDatas(info.startStr, info.endStr, info.allDay);
		},
		eventClick: (info: any) => {
			const ev = eventos.find(e => e.id === info.event.id);
			if (ev) {
				abrirDetalhes(ev);
			}
		},
		eventDrop: async (info: any) => {
			await persistirMudancaHorario(info.event);
		},
		eventResize: async (info: any) => {
			await persistirMudancaHorario(info.event);
		}
	});

	async function carregarEventos() {
		loading = true;
		try {
			const res = await apiFetch<EventoItem[]>(`/api/eventos?meus=${filtroMeus}`);
			eventos = res;

			options.events = res.map(e => ({
				id: e.id,
				title: e.recorrencia && e.recorrencia !== 'nenhuma' ? `🔄 ${e.titulo}` : e.titulo,
				start: e.inicio,
				end: e.fim,
				allDay: e.dia_inteiro,
				backgroundColor: e.criador_cor || '#2563EB',
				borderColor: e.criador_cor || '#2563EB',
				editable: e.pode_editar,
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

	function abrirCriarComDatas(inicioISO: string, fimISO: string, diaInteiro: boolean) {
		formId = null;
		formTitulo = '';
		formDescricao = '';
		formDiaInteiro = diaInteiro;
		formRecorrencia = 'nenhuma';
		formRecorrenciaFim = '';

		// Converte para formato local datetime-local "YYYY-MM-DDTHH:mm"
		formInicio = toDateTimeLocal(new Date(inicioISO));
		formFim = toDateTimeLocal(new Date(fimISO));

		formParticipantes = auth.user ? [auth.user.id] : [];
		modalAberto = true;
	}

	function abrirCriarManual() {
		const agora = new Date();
		const maisUmaHora = new Date(agora.getTime() + 60 * 60 * 1000);
		abrirCriarComDatas(agora.toISOString(), maisUmaHora.toISOString(), false);
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
		formInicio = toDateTimeLocal(new Date(eventoSelecionado.inicio));
		formFim = toDateTimeLocal(new Date(eventoSelecionado.fim));
		formDiaInteiro = eventoSelecionado.dia_inteiro;
		formRecorrencia = eventoSelecionado.recorrencia || 'nenhuma';
		formRecorrenciaFim = eventoSelecionado.recorrencia_fim ? eventoSelecionado.recorrencia_fim.split('T')[0] : '';
		formParticipantes = eventoSelecionado.participantes.map(p => p.id);

		modalDetalhesAberto = false;
		modalAberto = true;
	}

	async function salvarEvento() {
		if (!formTitulo || !formInicio || !formFim) {
			alert('Título, início e fim são obrigatórios.');
			return;
		}

		const inicioDate = new Date(formInicio);
		const fimDate = new Date(formFim);

		if (fimDate < inicioDate) {
			alert('O horário de término não pode ser anterior ao início.');
			return;
		}

		const payload: any = {
			titulo: formTitulo,
			descricao: formDescricao,
			inicio: inicioDate.toISOString(),
			fim: fimDate.toISOString(),
			dia_inteiro: formDiaInteiro,
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

	async function persistirMudancaHorario(calEvent: any) {
		try {
			await apiFetch(`/api/eventos/${calEvent.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					titulo: calEvent.title,
					descricao: calEvent.extendedProps?.descricao || '',
					inicio: new Date(calEvent.start).toISOString(),
					fim: new Date(calEvent.end || calEvent.start).toISOString(),
					dia_inteiro: calEvent.allDay
				})
			});
			await carregarEventos();
		} catch (err: any) {
			alert(err.message || 'Erro ao atualizar horário do evento');
			await carregarEventos(); // reverte visualmente em caso de erro
		}
	}

	function toDateTimeLocal(d: Date): string {
		const pad = (n: number) => n.toString().padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
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
	<!-- Topo com Filtros e Novo Evento -->
	<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Calendário de Reuniões</h1>
			<p class="text-sm text-slate-500">Agende compromissos entre operadores com sincronização em tempo real</p>
		</div>

		<div class="flex items-center gap-3">
			<!-- Filtro Meus Eventos / Todos -->
			<div class="flex items-center bg-slate-200/70 p-1 rounded-xl text-xs font-semibold">
				<button
					onclick={() => filtroMeus = false}
					class="px-3 py-1.5 rounded-lg transition {!filtroMeus ? 'bg-white text-slate-800 shadow-xs' : 'text-slate-500 hover:text-slate-700'}"
				>
					Todos os eventos
				</button>
				<button
					onclick={() => filtroMeus = true}
					class="px-3 py-1.5 rounded-lg transition {filtroMeus ? 'bg-white text-slate-800 shadow-xs' : 'text-slate-500 hover:text-slate-700'}"
				>
					Meus eventos
				</button>
			</div>

			<button
				onclick={abrirCriarManual}
				class="flex items-center gap-2 px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-semibold text-sm shadow-sm transition active:scale-95 cursor-pointer"
			>
				<Plus class="w-4 h-4" />
				<span class="hidden sm:inline">Novo Evento</span>
			</button>
		</div>
	</div>

	<!-- Container do Calendário -->
	<div class="bg-white rounded-2xl p-4 sm:p-6 border border-slate-200 shadow-xs relative">
		{#if loading}
			<div class="absolute inset-0 bg-white/70 backdrop-blur-xs flex items-center justify-center z-10 rounded-2xl">
				<div class="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
			</div>
		{/if}

		<div class="ec-theme-custom overflow-x-auto min-h-[600px]">
			<Calendar {plugins} {options} />
		</div>
	</div>

	<!-- Modal Criar / Editar Evento -->
	{#if modalAberto}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-3xl max-w-lg w-full p-6 shadow-2xl space-y-4 animate-in fade-in zoom-in-95 duration-150">
				<div class="flex items-center justify-between border-b border-slate-100 pb-3">
					<h3 class="font-bold text-slate-800 text-lg">
						{formId ? 'Editar Evento' : 'Novo Evento'}
					</h3>
					<button onclick={() => modalAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="space-y-3">
					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Título do Evento</label>
						<input 
							type="text" 
							bind:value={formTitulo} 
							placeholder="Ex: Alinhamento de estoque ou treinamento"
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
						/>
					</div>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
						<div>
							<label class="block text-xs font-semibold text-slate-700 mb-1">Início</label>
							<input 
								type="datetime-local" 
								bind:value={formInicio}
								class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
							/>
						</div>
						<div>
							<label class="block text-xs font-semibold text-slate-700 mb-1">Término</label>
							<input 
								type="datetime-local" 
								bind:value={formFim}
								class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
							/>
						</div>
					</div>

					<div class="flex items-center gap-2 pt-1">
						<input 
							type="checkbox" 
							id="diaInteiro" 
							bind:checked={formDiaInteiro}
							class="w-4 h-4 rounded text-blue-600"
						/>
						<label for="diaInteiro" class="text-xs font-medium text-slate-700 dark:text-slate-300 cursor-pointer">
							Evento de dia inteiro
						</label>
					</div>

					<!-- Recorrência -->
					<div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
						<div>
							<label class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1">Repetição</label>
							<select
								bind:value={formRecorrencia}
								class="w-full px-3 py-2 border border-slate-200 dark:border-slate-700 rounded-xl text-sm focus:outline-blue-500 bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-100"
							>
								<option value="nenhuma">Não se repete</option>
								<option value="semanal">Semanalmente (a cada 7 dias)</option>
								<option value="mensal">Mensalmente (mesmo dia do mês)</option>
							</select>
						</div>

						{#if formRecorrencia !== 'nenhuma'}
							<div>
								<label class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1">Repetir até (opcional)</label>
								<input
									type="date"
									bind:value={formRecorrenciaFim}
									class="w-full px-3 py-2 border border-slate-200 dark:border-slate-700 rounded-xl text-sm focus:outline-blue-500 bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-100"
								/>
							</div>
						{/if}
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1">Descrição (opcional)</label>
						<textarea 
							bind:value={formDescricao}
							rows="2"
							placeholder="Pauta da reunião ou observações adicionais..."
							class="w-full px-3 py-2 border border-slate-200 dark:border-slate-700 rounded-xl text-sm focus:outline-blue-500 bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-100"
						></textarea>
					</div>

					<!-- Seleção de Participantes -->
					<div>
						<label class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
							Participantes (Operadores do setor)
						</label>
						<div class="flex flex-wrap gap-2 max-h-36 overflow-y-auto p-1">
							{#each usuarios as u (u.id)}
								{@const selecionado = formParticipantes.includes(u.id)}
								{@const ehCriador = u.id === auth.user?.id}
								<button
									type="button"
									onclick={() => alternarParticipante(u.id)}
									class="flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium border transition cursor-pointer {selecionado ? 'bg-blue-50 dark:bg-blue-950/40 border-blue-400 text-blue-800 dark:text-blue-300' : 'bg-slate-50 dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700'}"
								>
									<span 
										class="w-2.5 h-2.5 rounded-full" 
										style="background-color: {u.cor || '#2563EB'};"
									></span>
									<span>{u.nome}</span>
									{#if ehCriador}
										<span class="text-[10px] text-blue-600 dark:text-blue-400 font-bold">(Você)</span>
									{/if}
									{#if selecionado}
										<Check class="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
									{/if}
								</button>
							{/each}
						</div>
					</div>
				</div>

				<div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100 dark:border-slate-800">
					<button 
						onclick={() => modalAberto = false}
						class="px-4 py-2 text-sm text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-xl font-medium cursor-pointer"
					>
						Cancelar
					</button>
					<button 
						onclick={salvarEvento}
						class="px-5 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs cursor-pointer"
					>
						{formId ? 'Atualizar Evento' : 'Salvar Evento'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal Detalhes do Evento -->
	{#if modalDetalhesAberto && eventoSelecionado}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white dark:bg-slate-900 border border-slate-100 dark:border-slate-800 rounded-3xl max-w-md w-full p-6 shadow-2xl space-y-4 text-slate-800 dark:text-slate-100">
				<div class="flex items-start justify-between">
					<div class="flex items-center gap-2">
						<span 
							class="w-4 h-4 rounded-full mt-0.5" 
							style="background-color: {eventoSelecionado.criador_cor};"
						></span>
						<h3 class="font-bold text-slate-800 dark:text-slate-100 text-lg leading-snug">{eventoSelecionado.titulo}</h3>
					</div>
					<button onclick={() => modalDetalhesAberto = false} class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="space-y-3 text-sm text-slate-600 dark:text-slate-300">
					<div class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
						<Clock class="w-4 h-4" />
						<span>
							{new Date(eventoSelecionado.inicio).toLocaleString('pt-BR')} até {new Date(eventoSelecionado.fim).toLocaleTimeString('pt-BR')}
						</span>
					</div>

					{#if eventoSelecionado.recorrencia && eventoSelecionado.recorrencia !== 'nenhuma'}
						<div class="flex items-center gap-1.5 text-xs text-blue-600 dark:text-blue-400 font-semibold bg-blue-50 dark:bg-blue-950/40 px-2.5 py-1.5 rounded-lg border border-blue-100 dark:border-blue-900/50">
							<Repeat class="w-3.5 h-3.5" />
							<span>Evento recorrente ({eventoSelecionado.recorrencia === 'semanal' ? 'Semanal' : 'Mensal'})</span>
						</div>
					{/if}

					{#if eventoSelecionado.descricao}
						<div class="p-3 bg-slate-50 rounded-xl text-slate-700 text-xs border border-slate-100 whitespace-pre-wrap">
							{eventoSelecionado.descricao}
						</div>
					{/if}

					<div>
						<div class="text-xs font-semibold text-slate-500 mb-1 flex items-center gap-1.5">
							<Users class="w-3.5 h-3.5" />
							<span>Participantes ({eventoSelecionado.participantes.length}):</span>
						</div>
						<div class="flex flex-wrap gap-1.5">
							{#each eventoSelecionado.participantes as p}
								<span 
									class="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-medium bg-slate-100 text-slate-700"
								>
									<span class="w-2 h-2 rounded-full" style="background-color: {p.cor};"></span>
									<span>{p.nome}</span>
								</span>
							{/each}
						</div>
					</div>

					<div class="text-[11px] text-slate-400 pt-2 border-t border-slate-100">
						Criado por: <strong>{eventoSelecionado.criador_nome}</strong>
					</div>
				</div>

				<div class="flex items-center justify-between pt-3 border-t border-slate-100">
					{#if eventoSelecionado.pode_editar}
						<button 
							onclick={excluirEvento}
							class="flex items-center gap-1.5 px-3 py-2 text-xs text-red-600 hover:bg-red-50 rounded-xl font-medium"
						>
							<Trash2 class="w-4 h-4" />
							<span>Excluir</span>
						</button>
						<button 
							onclick={editarEventoSelecionado}
							class="flex items-center gap-1.5 px-4 py-2 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs"
						>
							<Edit3 class="w-4 h-4" />
							<span>Editar</span>
						</button>
					{:else}
						<div class="text-xs text-slate-400 italic">Visualização apenas (você não é o criador nem admin)</div>
						<button 
							onclick={() => modalDetalhesAberto = false}
							class="px-4 py-2 text-xs bg-slate-100 hover:bg-slate-200 text-slate-700 rounded-xl font-semibold"
						>
							Fechar
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</div>
