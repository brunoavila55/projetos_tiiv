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
		Flag,
		MessageSquare,
		Send
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

	interface ComentarioItem {
		id: string;
		tarefa_id: string;
		usuario_id: string;
		usuario_nome: string;
		usuario_cor: string;
		conteudo: string;
		criado_em: string;
		pode_excluir: boolean;
	}

	// Modal Criar / Editar
	let modalAberto = $state(false);
	let formId = $state<string | null>(null);
	let formTitulo = $state('');
	let formDescricao = $state('');
	let formPrioridade = $state<'baixa' | 'media' | 'alta'>('media');
	let formPrazo = $state('');
	let formResponsavel = $state('');

	// Comentários da Tarefa selecionada
	let comentarios = $state<ComentarioItem[]>([]);
	let novoComentario = $state('');
	let enviandoComentario = $state(false);
	let carregandoComentarios = $state(false);

	async function carregarComentarios(tarefaId: string) {
		carregandoComentarios = true;
		try {
			comentarios = await apiFetch<ComentarioItem[]>(`/api/tarefas/${tarefaId}/comentarios`);
		} catch (err) {
			console.error('Erro ao carregar comentários:', err);
			comentarios = [];
		} finally {
			carregandoComentarios = false;
		}
	}

	async function adicionarComentario() {
		if (!formId || !novoComentario.trim()) return;
		enviandoComentario = true;
		try {
			const res = await apiFetch<ComentarioItem>(`/api/tarefas/${formId}/comentarios`, {
				method: 'POST',
				body: JSON.stringify({ conteudo: novoComentario.trim() })
			});
			comentarios = [...comentarios, res];
			novoComentario = '';
		} catch (err) {
			console.error('Erro ao adicionar comentário:', err);
		} finally {
			enviandoComentario = false;
		}
	}

	async function excluirComentario(cid: string) {
		if (!formId || !confirm('Excluir este comentário?')) return;
		try {
			await apiFetch(`/api/tarefas/${formId}/comentarios/${cid}`, {
				method: 'DELETE'
			});
			comentarios = comentarios.filter(c => c.id !== cid);
		} catch (err) {
			console.error('Erro ao excluir comentário:', err);
		}
	}

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
		comentarios = [];
		novoComentario = '';
		modalAberto = true;
	}

	function abrirEditar(t: TarefaItem) {
		formId = t.id;
		formTitulo = t.titulo;
		formDescricao = t.descricao;
		formPrioridade = t.prioridade;
		formPrazo = t.prazo ? t.prazo.split('T')[0] : '';
		formResponsavel = t.responsavel_id || '';
		comentarios = [];
		novoComentario = '';
		carregarComentarios(t.id);
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
	<div class="page-head">
		<div>
			<h1 class="page-title">Tarefas</h1>
			<p class="page-sub">
				{auth.user?.papel === 'admin'
					? 'Pendências da equipe, com prioridade, prazo e responsável.'
					: 'Tarefas que você criou ou pelas quais é responsável.'}
			</p>
		</div>
		<button onclick={abrirCriar} class="btn btn-primary">
			<Plus class="size-4" />
			<span>Nova tarefa</span>
		</button>
	</div>

	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
		<div class="segmented w-full sm:w-auto" role="group" aria-label="Visão">
			<button class="flex-1 sm:flex-none" aria-pressed={visao === 'minhas'} onclick={() => visao = 'minhas'}>Comigo</button>
			<button class="flex-1 sm:flex-none" aria-pressed={visao === 'criadas'} onclick={() => visao = 'criadas'}>Criadas por mim</button>
			<button class="flex-1 sm:flex-none" aria-pressed={visao === 'todas'} onclick={() => visao = 'todas'}>Todas</button>
		</div>

		<select bind:value={filtroStatus} class="field w-full sm:w-48" aria-label="Status">
			<option value="">Qualquer status</option>
			<option value="pendente">Pendentes</option>
			<option value="em_andamento">Em andamento</option>
			<option value="concluida">Concluídas</option>
		</select>
	</div>

	{#if loading}
		<div class="flex justify-center py-20"><div class="spinner"></div></div>
	{:else if tarefas.length === 0}
		<div class="panel empty">
			<CheckSquare class="size-9 text-ink-3" strokeWidth={1.5} />
			<h3 class="empty-title">Nenhuma tarefa aqui</h3>
			<p class="empty-text">Troque a visão acima ou crie uma nova tarefa.</p>
			<button onclick={abrirCriar} class="btn btn-secondary mt-5">
				<Plus class="size-4" />
				<span>Nova tarefa</span>
			</button>
		</div>
	{:else}
		<ul class="panel divide-y divide-line overflow-hidden">
			{#each tarefas as t (t.id)}
				<li
					class="relative flex items-start gap-3 px-4 sm:px-5 py-4 transition-colors hover:bg-sunken {t.status === 'concluida' ? 'bg-sunken/60' : ''}"
				>
					{#if t.atrasada && t.status !== 'concluida'}
						<span class="absolute left-0 inset-y-0 w-[3px] bg-danger" aria-hidden="true"></span>
					{/if}

					<!-- Concluir com um clique -->
					<button
						type="button"
						onclick={() => alternarStatus(t)}
						class="shrink-0 -m-1 p-1 rounded-full cursor-pointer text-ink-3 hover:text-accent transition-colors"
						title={t.status === 'concluida' ? 'Reabrir tarefa' : 'Concluir tarefa'}
						aria-label={t.status === 'concluida' ? 'Reabrir tarefa' : 'Concluir tarefa'}
					>
						{#if t.status === 'concluida'}
							<CheckCircle2 class="size-[22px] text-ok" />
						{:else if t.status === 'em_andamento'}
							<PlayCircle class="size-[22px] text-warn" />
						{:else}
							<Circle class="size-[22px]" />
						{/if}
					</button>

					<div class="flex-1 min-w-0">
						<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
							<span class="font-semibold text-[15px] {t.status === 'concluida' ? 'line-through decoration-ink-3/60 text-ink-3' : 'text-ink'}">
								{t.titulo}
							</span>
							{#if t.atrasada}
								<span class="tag tag-danger">Atrasada</span>
							{/if}
							{#if t.status === 'em_andamento'}
								<span class="tag tag-warn">Em andamento</span>
							{/if}
							{#if t.prioridade === 'alta'}
								<span class="tag tag-danger bg-transparent px-0"><Flag class="size-3" /> Alta</span>
							{/if}
						</div>

						{#if t.descricao}
							<p class="mt-1 text-sm text-ink-2 line-clamp-2 max-w-[75ch]">{t.descricao}</p>
						{/if}

						<div class="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-[13px] text-ink-3">
							{#if t.responsavel_nome}
								<span class="inline-flex items-center gap-1.5">
									<span class="dot size-2" style="background-color: {t.responsavel_cor};"></span>
									{t.responsavel_nome}
								</span>
							{:else}
								<span>Sem responsável</span>
							{/if}

							{#if t.prazo}
								<span class="inline-flex items-center gap-1 tabular {t.atrasada ? 'text-danger font-semibold' : ''}">
									<Clock class="size-3.5" />
									{new Date(t.prazo).toLocaleDateString('pt-BR')}
								</span>
							{/if}

							{#if t.prioridade === 'media'}
								<span>Prioridade média</span>
							{:else if t.prioridade === 'baixa'}
								<span>Prioridade baixa</span>
							{/if}

							{#if t.concluida_em}
								<span class="text-ok">Concluída em {new Date(t.concluida_em).toLocaleDateString('pt-BR')}</span>
							{/if}
						</div>
					</div>

					<div class="flex items-center gap-0.5 shrink-0">
						{#if t.status === 'pendente'}
							<button type="button" onclick={(e) => definirEmAndamento(t, e)} class="btn btn-sm btn-soft mr-1">
								Iniciar
							</button>
						{/if}
						{#if t.pode_editar}
							<button type="button" onclick={() => abrirEditar(t)} class="icon-btn" title="Editar" aria-label="Editar tarefa">
								<Edit3 class="size-4" />
							</button>
						{/if}
						{#if t.pode_excluir}
							<button type="button" onclick={(e) => excluirTarefa(t.id, e)} class="icon-btn icon-btn-danger" title="Excluir" aria-label="Excluir tarefa">
								<Trash2 class="size-4" />
							</button>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	{/if}

	<!-- Modal criar / editar -->
	{#if modalAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{formId ? 'Editar tarefa' : 'Nova tarefa'}</h3>
					<button onclick={() => modalAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div>
						<label class="label" for="tf-titulo">Título</label>
						<input
							id="tf-titulo"
							type="text"
							bind:value={formTitulo}
							placeholder="Ex.: Repor toner da impressora do 2º andar"
							class="field"
							autofocus
						/>
					</div>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div>
							<label class="label" for="tf-prioridade">Prioridade</label>
							<select id="tf-prioridade" bind:value={formPrioridade} class="field">
								<option value="baixa">Baixa</option>
								<option value="media">Média</option>
								<option value="alta">Alta</option>
							</select>
						</div>
						<div>
							<label class="label" for="tf-prazo">Prazo <span class="font-normal text-ink-3">(opcional)</span></label>
							<input id="tf-prazo" type="date" bind:value={formPrazo} class="field" />
						</div>
					</div>

					<div>
						<label class="label" for="tf-resp">Responsável <span class="font-normal text-ink-3">(opcional)</span></label>
						<select id="tf-resp" bind:value={formResponsavel} class="field">
							<option value="">Ninguém ainda</option>
							{#each usuarios as u}
								<option value={u.id}>{u.nome}</option>
							{/each}
						</select>
					</div>

					<div>
						<label class="label" for="tf-desc">Descrição</label>
						<textarea
							id="tf-desc"
							bind:value={formDescricao}
							rows="3"
							placeholder="Detalhes e instruções"
							class="field"
						></textarea>
					</div>

					{#if formId}
						<!-- Comentários -->
						<div class="pt-4 border-t border-line space-y-3">
							<h4 class="flex items-center gap-2 text-sm font-bold text-ink">
								<MessageSquare class="size-4 text-ink-3" />
								Comentários
								<span class="font-semibold text-ink-3 tabular">{comentarios.length}</span>
							</h4>

							<div class="max-h-52 overflow-y-auto space-y-3 pr-1">
								{#if carregandoComentarios}
									<p class="text-sm text-ink-3">Carregando comentários…</p>
								{:else if comentarios.length === 0}
									<p class="text-sm text-ink-3">Nenhum comentário ainda.</p>
								{:else}
									{#each comentarios as c (c.id)}
										<div class="group flex gap-2.5">
											<div class="avatar size-7 text-[11px] mt-0.5" style="background-color: {c.usuario_cor || '#1d5bbf'};">
												{c.usuario_nome.slice(0, 1).toUpperCase()}
											</div>
											<div class="flex-1 min-w-0">
												<div class="flex items-baseline gap-2">
													<span class="text-sm font-semibold text-ink">{c.usuario_nome}</span>
													<time class="text-xs text-ink-3">
														{new Date(c.criado_em).toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })}
													</time>
													{#if c.pode_excluir}
														<button
															onclick={() => excluirComentario(c.id)}
															class="ml-auto icon-btn icon-btn-danger size-6 opacity-0 group-hover:opacity-100 focus:opacity-100"
															title="Excluir comentário"
															aria-label="Excluir comentário"
														>
															<Trash2 class="size-3.5" />
														</button>
													{/if}
												</div>
												<p class="text-sm text-ink-2 whitespace-pre-wrap">{c.conteudo}</p>
											</div>
										</div>
									{/each}
								{/if}
							</div>

							<div class="flex items-center gap-2">
								<input
									type="text"
									bind:value={novoComentario}
									onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); adicionarComentario(); }}}
									placeholder="Escreva um comentário"
									class="field"
									aria-label="Novo comentário"
								/>
								<button
									onclick={adicionarComentario}
									disabled={!novoComentario.trim() || enviandoComentario}
									class="btn btn-secondary h-10"
								>
									<Send class="size-4" />
									<span>Enviar</span>
								</button>
							</div>
						</div>
					{/if}
				</div>

				<div class="modal-foot">
					<button onclick={() => modalAberto = false} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarTarefa} class="btn btn-primary">
						{formId ? 'Salvar alterações' : 'Criar tarefa'}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
