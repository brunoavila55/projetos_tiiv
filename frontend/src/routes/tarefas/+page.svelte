<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { 
		CheckSquare, 
		Plus, 
		Calendar, 
		Clock, 
		AlertTriangle, 
		CheckCircle2, 
		Circle, 
		PlayCircle, 
		User, 
		Edit3, 
		Trash2, 
		X, 
		ArrowDown, 
		ArrowUp,
		Flag
	} from 'lucide-svelte';

	interface TarefaItem {
		id: string;
		titulo: string;
		descricao: string;
		status: 'pendente' | 'em_andamento' | 'concluida';
		prioridade: 'baixa' | 'media' | 'alta';
		prazo: string | null;
		criado_por: string;
		criador_nome: string;
		criador_cor: string;
		responsavel_id: string | null;
		responsavel_nome: string | null;
		responsavel_cor: string | null;
		concluida_em: string | null;
		criado_em: string;
		atualizado_em: string;
		atrasada: boolean;
		pode_editar: boolean;
		pode_excluir: boolean;
	}

	let tarefas = $state<TarefaItem[]>([]);
	let usuarios = $state<UsuarioPublico[]>([]);
	let loading = $state(true);

	// Filtros de visão
	let visao = $state<'minhas' | 'criadas' | 'todas'>('minhas');
	let filtroStatus = $state<string>('');

	// Modal Criar / Editar
	let modalAberto = $state(false);
	let formId = $state<string | null>(null);
	let formTitulo = $state('');
	let formDescricao = $state('');
	let formPrioridade = $state<'baixa' | 'media' | 'alta'>('media');
	let formPrazo = $state('');
	let formResponsavel = $state('');

	async function carregar() {
		loading = true;
		try {
			const params = new URLSearchParams({ visao });
			if (filtroStatus) params.set('status', filtroStatus);
			tarefas = await apiFetch<TarefaItem[]>(`/api/tarefas?${params.toString()}`);
		} catch (err) {
			console.error('Erro ao carregar tarefas:', err);
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

	function abrirCriar() {
		formId = null;
		formTitulo = '';
		formDescricao = '';
		formPrioridade = 'media';
		formPrazo = '';
		formResponsavel = auth.user ? auth.user.id : '';
		modalAberto = true;
	}

	function abrirEditar(t: TarefaItem) {
		formId = t.id;
		formTitulo = t.titulo;
		formDescricao = t.descricao;
		formPrioridade = t.prioridade;
		formPrazo = t.prazo ? t.prazo.split('T')[0] : '';
		formResponsavel = t.responsavel_id || '';
		modalAberto = true;
	}

	async function salvarTarefa() {
		if (!formTitulo.trim()) {
			alert('O título da tarefa é obrigatório.');
			return;
		}

		const payload = {
			titulo: formTitulo.trim(),
			descricao: formDescricao.trim(),
			prioridade: formPrioridade,
			prazo: formPrazo ? new Date(formPrazo + 'T23:59:59').toISOString() : null,
			responsavel_id: formResponsavel || null
		};

		try {
			if (formId) {
				await apiFetch(`/api/tarefas/${formId}`, {
					method: 'PUT',
					body: JSON.stringify(payload)
				});
			} else {
				await apiFetch('/api/tarefas', {
					method: 'POST',
					body: JSON.stringify(payload)
				});
			}
			modalAberto = false;
			await carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao salvar tarefa');
		}
	}

	async function alternarStatus(t: TarefaItem) {
		// Clique com 1 toque: se concluída, reabre como pendente (limpa concluida_em). Se pendente/em_andamento, conclui (grava concluida_em).
		const novoStatus = t.status === 'concluida' ? 'pendente' : 'concluida';
		try {
			await apiFetch(`/api/tarefas/${t.id}/status`, {
				method: 'PATCH',
				body: JSON.stringify({ status: novoStatus })
			});
			await carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao atualizar status da tarefa');
		}
	}

	async function definirEmAndamento(t: TarefaItem, e: MouseEvent) {
		e.stopPropagation();
		try {
			await apiFetch(`/api/tarefas/${t.id}/status`, {
				method: 'PATCH',
				body: JSON.stringify({ status: 'em_andamento' })
			});
			await carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao atualizar status');
		}
	}

	async function excluirTarefa(id: string, e: MouseEvent) {
		e.stopPropagation();
		if (!confirm('Deseja realmente excluir esta tarefa?')) return;
		try {
			await apiFetch(`/api/tarefas/${id}`, { method: 'DELETE' });
			await carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao excluir tarefa');
		}
	}

	onMount(() => {
		carregarUsuarios();
		carregar();
	});

	$effect(() => {
		if (visao || filtroStatus !== undefined) {
			carregar();
		}
	});
</script>

<div class="space-y-6">
	<!-- Topo com Abas de Visão e Botão Nova Tarefa -->
	<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Quadro de Tarefas</h1>
			<p class="text-sm text-slate-500">Acompanhamento com prioridades, prazos e alertas visuais de atraso</p>
		</div>

		<button
			onclick={abrirCriar}
			class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-semibold text-sm shadow-sm transition active:scale-95 cursor-pointer"
		>
			<Plus class="w-4 h-4" />
			<span>Nova Tarefa</span>
		</button>
	</div>

	<!-- Barra de Abas e Filtro de Status -->
	<div class="flex flex-col sm:flex-row items-center justify-between gap-3 bg-white p-3 rounded-2xl border border-slate-200 shadow-xs">
		<!-- Abas -->
		<div class="flex items-center bg-slate-100 p-1 rounded-xl w-full sm:w-auto text-xs font-semibold">
			<button
				onclick={() => visao = 'minhas'}
				class="flex-1 sm:flex-none px-3.5 py-2 rounded-lg transition {visao === 'minhas' ? 'bg-white text-slate-800 shadow-xs' : 'text-slate-500 hover:text-slate-700'}"
			>
				Minhas tarefas
			</button>
			<button
				onclick={() => visao = 'criadas'}
				class="flex-1 sm:flex-none px-3.5 py-2 rounded-lg transition {visao === 'criadas' ? 'bg-white text-slate-800 shadow-xs' : 'text-slate-500 hover:text-slate-700'}"
			>
				Criadas por mim
			</button>
			<button
				onclick={() => visao = 'todas'}
				class="flex-1 sm:flex-none px-3.5 py-2 rounded-lg transition {visao === 'todas' ? 'bg-white text-slate-800 shadow-xs' : 'text-slate-500 hover:text-slate-700'}"
			>
				Todas as tarefas
			</button>
		</div>

		<!-- Filtro Status -->
		<div class="flex items-center gap-2 w-full sm:w-auto">
			<select
				bind:value={filtroStatus}
				class="w-full sm:w-40 px-3 py-2 border border-slate-200 rounded-xl text-xs font-medium bg-slate-50/50 focus:outline-blue-500"
			>
				<option value="">Todos os status</option>
				<option value="pendente">Apenas Pendentes</option>
				<option value="em_andamento">Em Andamento</option>
				<option value="concluida">Concluídas</option>
			</select>
		</div>
	</div>

	<!-- Lista de Tarefas -->
	{#if loading}
		<div class="flex justify-center py-20">
			<div class="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
		</div>
	{:else if tarefas.length === 0}
		<div class="bg-white rounded-2xl border border-slate-200 p-12 text-center max-w-md mx-auto shadow-xs">
			<CheckSquare class="w-12 h-12 text-slate-300 mx-auto mb-3" />
			<h3 class="text-base font-bold text-slate-800">Nenhuma tarefa nesta visualização</h3>
			<p class="text-xs text-slate-500 mt-1">Crie uma nova tarefa ou alterne entre as abas acima.</p>
		</div>
	{:else}
		<div class="space-y-3">
			{#each tarefas as t (t.id)}
				<div 
					class="bg-white rounded-2xl p-4 sm:p-5 border transition-all duration-150 shadow-xs flex items-start gap-3.5 {t.atrasada ? 'border-red-300 bg-red-50/30' : 'border-slate-200 hover:border-slate-300'} {t.status === 'concluida' ? 'opacity-60 bg-slate-50/70' : ''}"
				>
					<!-- Botão de Conclusão com 1 Clique -->
					<button
						type="button"
						onclick={() => alternarStatus(t)}
						class="mt-1 flex-shrink-0 text-slate-400 hover:text-blue-600 transition cursor-pointer"
						title={t.status === 'concluida' ? 'Reabrir tarefa' : 'Marcar como concluída'}
					>
						{#if t.status === 'concluida'}
							<CheckCircle2 class="w-6 h-6 text-emerald-600" />
						{:else if t.status === 'em_andamento'}
							<PlayCircle class="w-6 h-6 text-amber-500" />
						{:else}
							<Circle class="w-6 h-6 text-slate-300 hover:text-blue-500" />
						{/if}
					</button>

					<!-- Conteúdo da Tarefa -->
					<div class="flex-1 min-w-0 space-y-1.5">
						<div class="flex flex-wrap items-center gap-2">
							<span class="font-bold text-slate-800 text-base {t.status === 'concluida' ? 'line-through text-slate-500' : ''}">
								{t.titulo}
							</span>

							<!-- Badge Atrasada Destacada -->
							{#if t.atrasada}
								<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-bold bg-red-100 text-red-700 animate-pulse">
									<AlertTriangle class="w-3 h-3" />
									<span>Atrasada</span>
								</span>
							{/if}

							<!-- Prioridade -->
							{#if t.prioridade === 'alta'}
								<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold bg-red-100 text-red-700">
									<Flag class="w-3 h-3" /> Alta
								</span>
							{:else if t.prioridade === 'media'}
								<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-semibold bg-amber-100 text-amber-700">
									Média
								</span>
							{:else}
								<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-medium bg-slate-100 text-slate-600">
									Baixa
								</span>
							{/if}

							<!-- Status tag se em andamento -->
							{#if t.status === 'em_andamento'}
								<span class="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-semibold bg-amber-50 text-amber-700 border border-amber-200">
									Em andamento
								</span>
							{/if}
						</div>

						{#if t.descricao}
							<p class="text-sm text-slate-600 line-clamp-2">{t.descricao}</p>
						{/if}

						<!-- Rodapé do Card: Responsável, Prazo e Criador -->
						<div class="flex flex-wrap items-center gap-3 pt-1 text-xs text-slate-400">
							{#if t.responsavel_nome}
								<div class="flex items-center gap-1.5 text-slate-600 font-medium">
									<span class="w-2.5 h-2.5 rounded-full" style="background-color: {t.responsavel_cor};"></span>
									<span>Responsável: {t.responsavel_nome}</span>
								</div>
							{:else}
								<span class="italic text-slate-400">Sem responsável</span>
							{/if}

							{#if t.prazo}
								<div class="flex items-center gap-1 {t.atrasada ? 'text-red-600 font-bold' : 'text-slate-500'}">
									<Clock class="w-3.5 h-3.5" />
									<span>Prazo: {new Date(t.prazo).toLocaleDateString('pt-BR')}</span>
								</div>
							{/if}

							{#if t.concluida_em}
								<div class="text-emerald-600">
									Concluída em {new Date(t.concluida_em).toLocaleDateString('pt-BR')}
								</div>
							{/if}
						</div>
					</div>

					<!-- Ações Rápidas -->
					<div class="flex items-center gap-1 self-center">
						{#if t.status === 'pendente'}
							<button
								type="button"
								onclick={(e) => definirEmAndamento(t, e)}
								class="px-2.5 py-1 text-xs font-semibold rounded-lg bg-amber-50 hover:bg-amber-100 text-amber-700 transition"
								title="Iniciar tarefa"
							>
								Iniciar
							</button>
						{/if}

						{#if t.pode_editar}
							<button
								type="button"
								onclick={() => abrirEditar(t)}
								class="p-2 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition"
								title="Editar tarefa"
							>
								<Edit3 class="w-4 h-4" />
							</button>
						{/if}

						{#if t.pode_excluir}
							<button
								type="button"
								onclick={(e) => excluirTarefa(t.id, e)}
								class="p-2 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition"
								title="Excluir tarefa"
							>
								<Trash2 class="w-4 h-4" />
							</button>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}

	<!-- Modal Criar / Editar Tarefa -->
	{#if modalAberto}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-3xl max-w-lg w-full p-6 shadow-2xl space-y-4 animate-in fade-in zoom-in-95 duration-150">
				<div class="flex items-center justify-between border-b border-slate-100 pb-3">
					<h3 class="font-bold text-slate-800 text-lg">
						{formId ? 'Editar Tarefa' : 'Nova Tarefa'}
					</h3>
					<button onclick={() => modalAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="space-y-3.5">
					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Título da Tarefa *</label>
						<input 
							type="text" 
							bind:value={formTitulo} 
							placeholder="Ex: Verificar suprimentos de informática"
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
							autofocus
						/>
					</div>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
						<div>
							<label class="block text-xs font-semibold text-slate-700 mb-1">Prioridade</label>
							<select 
								bind:value={formPrioridade}
								class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500"
							>
								<option value="baixa">Baixa</option>
								<option value="media">Média</option>
								<option value="alta">Alta</option>
							</select>
						</div>

						<div>
							<label class="block text-xs font-semibold text-slate-700 mb-1">Prazo (opcional)</label>
							<input 
								type="date" 
								bind:value={formPrazo}
								class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
							/>
						</div>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Responsável (opcional)</label>
						<select 
							bind:value={formResponsavel}
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500"
						>
							<option value="">Sem responsável definido</option>
							{#each usuarios as u}
								<option value={u.id}>{u.nome}</option>
							{/each}
						</select>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Descrição</label>
						<textarea 
							bind:value={formDescricao}
							rows="3"
							placeholder="Instruções e detalhes adicionais da tarefa..."
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500"
						></textarea>
					</div>
				</div>

				<div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
					<button 
						onclick={() => modalAberto = false}
						class="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-xl font-medium"
					>
						Cancelar
					</button>
					<button 
						onclick={salvarTarefa}
						class="px-5 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs"
					>
						{formId ? 'Salvar Alterações' : 'Criar Tarefa'}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
