<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { 
		Headphones, 
		Plus, 
		Search, 
		Filter, 
		Calendar, 
		User, 
		Clock, 
		Edit3, 
		Trash2, 
		X, 
		ChevronLeft, 
		ChevronRight,
		CheckCircle2,
		AlertCircle
	} from 'lucide-svelte';

	interface AtendimentoItem {
		id: string;
		cliente_nome: string;
		descricao: string;
		data_atendimento: string;
		usuario_id: string;
		usuario_nome: string;
		usuario_cor: string;
		criado_em: string;
		atualizado_em: string;
		pode_editar: boolean;
	}

	interface RespostaListagem {
		itens: AtendimentoItem[];
		total: number;
		pagina: number;
		limite: number;
		total_pages: number;
	}

	let atendimentos = $state<AtendimentoItem[]>([]);
	let usuarios = $state<UsuarioPublico[]>([]);
	let loading = $state(true);
	let totalRegistros = $state(0);
	let paginaAtual = $state(1);
	let totalPaginas = $state(1);

	// Filtros
	let filtroBusca = $state('');
	let filtroUsuario = $state('');
	let filtroInicio = $state('');
	let filtroFim = $state('');
	let debounceTimeout: any = null;

	// Modais
	let modalAberto = $state(false);
	let modalDetalhesAberto = $state(false);
	let atendimentoSelecionado = $state<AtendimentoItem | null>(null);

	// Formulário
	let formId = $state<string | null>(null);
	let formCliente = $state('');
	let formDescricao = $state('');
	let formData = $state('');

	async function carregar(resetPagina = false) {
		if (resetPagina) paginaAtual = 1;
		loading = true;
		try {
			const params = new URLSearchParams({
				pagina: paginaAtual.toString(),
				limite: '15'
			});
			if (filtroBusca.trim()) params.set('busca', filtroBusca.trim());
			if (filtroUsuario) params.set('usuario_id', filtroUsuario);
			if (filtroInicio) params.set('inicio', filtroInicio);
			if (filtroFim) params.set('fim', filtroFim);

			const res = await apiFetch<RespostaListagem>(`/api/atendimentos?${params.toString()}`);
			atendimentos = res.itens;
			totalRegistros = res.total;
			totalPaginas = res.total_pages;
		} catch (err) {
			console.error('Erro ao carregar atendimentos:', err);
		} finally {
			loading = false;
		}
	}

	async function carregarUsuarios() {
		try {
			usuarios = await apiFetch<UsuarioPublico[]>('/api/auth/usuarios');
		} catch (err) {
			console.error('Erro ao carregar lista de usuários:', err);
		}
	}

	function handleBuscaInput() {
		clearTimeout(debounceTimeout);
		debounceTimeout = setTimeout(() => {
			carregar(true);
		}, 300);
	}

	function abrirCriar() {
		formId = null;
		formCliente = '';
		formDescricao = '';
		formData = toDateTimeLocal(new Date());
		modalAberto = true;
	}

	function abrirEditar(a: AtendimentoItem) {
		formId = a.id;
		formCliente = a.cliente_nome;
		formDescricao = a.descricao;
		formData = toDateTimeLocal(new Date(a.data_atendimento));
		modalDetalhesAberto = false;
		modalAberto = true;
	}

	function abrirDetalhes(a: AtendimentoItem) {
		atendimentoSelecionado = a;
		modalDetalhesAberto = true;
	}

	async function salvarAtendimento() {
		if (!formCliente.trim() || !formDescricao.trim()) {
			alert('Preencha o nome do cliente e a descrição do atendimento.');
			return;
		}

		const payload = {
			cliente_nome: formCliente.trim(),
			descricao: formDescricao.trim(),
			data_atendimento: formData ? new Date(formData).toISOString() : new Date().toISOString()
		};

		try {
			if (formId) {
				await apiFetch(`/api/atendimentos/${formId}`, {
					method: 'PUT',
					body: JSON.stringify(payload)
				});
			} else {
				await apiFetch('/api/atendimentos', {
					method: 'POST',
					body: JSON.stringify(payload)
				});
			}
			modalAberto = false;
			await carregar(formId === null);
		} catch (err: any) {
			alert(err.message || 'Erro ao salvar atendimento');
		}
	}

	async function excluirAtendimento(id: string) {
		if (!confirm('Deseja realmente excluir este atendimento?')) return;
		try {
			await apiFetch(`/api/atendimentos/${id}`, { method: 'DELETE' });
			modalDetalhesAberto = false;
			await carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao excluir atendimento');
		}
	}

	function toDateTimeLocal(d: Date): string {
		const pad = (n: number) => n.toString().padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
	}

	onMount(() => {
		carregarUsuarios();
		carregar();
	});
</script>

<div class="space-y-6">
	<!-- Topo com Título e Botão de Ação -->
	<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Registro de Atendimentos</h1>
			<p class="text-sm text-slate-500">Histórico de atendimentos do setor com busca rápida sem diferenciação de acentos</p>
		</div>

		<button
			onclick={abrirCriar}
			class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-semibold text-sm shadow-sm transition active:scale-95 cursor-pointer"
		>
			<Plus class="w-4 h-4" />
			<span>Novo Atendimento</span>
		</button>
	</div>

	<!-- Barra de Busca e Filtros -->
	<div class="bg-white rounded-2xl p-4 border border-slate-200 shadow-xs space-y-3">
		<div class="flex flex-col sm:flex-row items-center gap-3">
			<div class="relative flex-1 w-full">
				<Search class="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
				<input
					type="text"
					bind:value={filtroBusca}
					oninput={handleBuscaInput}
					placeholder="Buscar por cliente ou conteúdo da descrição..."
					class="w-full pl-10 pr-4 py-2 rounded-xl border border-slate-200 text-sm focus:outline-blue-500 bg-slate-50/50"
				/>
			</div>

			<div class="flex items-center gap-2 w-full sm:w-auto">
				<!-- Filtro por Usuário -->
				<select
					bind:value={filtroUsuario}
					onchange={() => carregar(true)}
					class="w-full sm:w-44 px-3 py-2 rounded-xl border border-slate-200 text-xs sm:text-sm font-medium focus:outline-blue-500 bg-white"
				>
					<option value="">Todos operadores</option>
					{#each usuarios as u}
						<option value={u.id}>{u.nome}</option>
					{/each}
				</select>
			</div>
		</div>

		<div class="flex flex-wrap items-center gap-3 pt-2 border-t border-slate-100 text-xs">
			<span class="text-slate-500 font-semibold flex items-center gap-1">
				<Filter class="w-3.5 h-3.5" /> Período:
			</span>
			<div class="flex items-center gap-2">
				<input
					type="date"
					bind:value={filtroInicio}
					onchange={() => carregar(true)}
					class="px-2.5 py-1.5 rounded-lg border border-slate-200 text-xs bg-white text-slate-700 focus:outline-blue-500"
				/>
				<span class="text-slate-400">até</span>
				<input
					type="date"
					bind:value={filtroFim}
					onchange={() => carregar(true)}
					class="px-2.5 py-1.5 rounded-lg border border-slate-200 text-xs bg-white text-slate-700 focus:outline-blue-500"
				/>
			</div>
			{#if filtroBusca || filtroUsuario || filtroInicio || filtroFim}
				<button
					onclick={() => {
						filtroBusca = '';
						filtroUsuario = '';
						filtroInicio = '';
						filtroFim = '';
						carregar(true);
					}}
					class="text-blue-600 hover:text-blue-800 font-semibold ml-auto"
				>
					Limpar filtros
				</button>
			{/if}
		</div>
	</div>

	<!-- Lista de Atendimentos -->
	{#if loading}
		<div class="flex justify-center py-20">
			<div class="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
		</div>
	{:else if atendimentos.length === 0}
		<div class="bg-white rounded-2xl border border-slate-200 p-12 text-center max-w-md mx-auto shadow-xs">
			<Headphones class="w-12 h-12 text-slate-300 mx-auto mb-3" />
			<h3 class="text-base font-bold text-slate-800">Nenhum atendimento encontrado</h3>
			<p class="text-xs text-slate-500 mt-1">Ajuste os filtros de busca ou cadastre o primeiro atendimento.</p>
		</div>
	{:else}
		<div class="bg-white rounded-2xl border border-slate-200 shadow-xs divide-y divide-slate-100 overflow-hidden">
			{#each atendimentos as a (a.id)}
				<div 
					class="p-4 sm:p-5 hover:bg-slate-50/70 transition flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 cursor-pointer"
					onclick={() => abrirDetalhes(a)}
				>
					<div class="space-y-1.5 flex-1 min-w-0">
						<div class="flex items-center gap-2.5 flex-wrap">
							<span class="font-bold text-slate-800 text-base">{a.cliente_nome}</span>
							<span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-600">
								<span class="w-2 h-2 rounded-full" style="background-color: {a.usuario_cor};"></span>
								<span>{a.usuario_nome}</span>
							</span>
							<span class="text-xs text-slate-400 flex items-center gap-1">
								<Clock class="w-3.5 h-3.5" />
								<span>{new Date(a.data_atendimento).toLocaleString('pt-BR')}</span>
							</span>
						</div>
						<p class="text-sm text-slate-600 line-clamp-2">{a.descricao}</p>
					</div>

					{#if a.pode_editar}
						<div class="flex items-center gap-1 sm:self-center" onclick={(e) => e.stopPropagation()}>
							<button
								onclick={() => abrirEditar(a)}
								class="p-2 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition"
								title="Editar atendimento"
							>
								<Edit3 class="w-4 h-4" />
							</button>
							<button
								onclick={() => excluirAtendimento(a.id)}
								class="p-2 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition"
								title="Excluir atendimento"
							>
								<Trash2 class="w-4 h-4" />
							</button>
						</div>
					{/if}
				</div>
			{/each}
		</div>

		<!-- Paginação -->
		<div class="flex items-center justify-between text-xs text-slate-500 px-2">
			<span>Total de {totalRegistros} atendimento(s)</span>
			<div class="flex items-center gap-2">
				<button
					disabled={paginaAtual <= 1}
					onclick={() => { paginaAtual--; carregar(); }}
					class="p-2 rounded-lg border border-slate-200 hover:bg-slate-100 disabled:opacity-30 disabled:cursor-not-allowed"
				>
					<ChevronLeft class="w-4 h-4" />
				</button>
				<span class="font-semibold">Página {paginaAtual} de {totalPaginas}</span>
				<button
					disabled={paginaAtual >= totalPaginas}
					onclick={() => { paginaAtual++; carregar(); }}
					class="p-2 rounded-lg border border-slate-200 hover:bg-slate-100 disabled:opacity-30 disabled:cursor-not-allowed"
				>
					<ChevronRight class="w-4 h-4" />
				</button>
			</div>
		</div>
	{/if}

	<!-- Modal Registrar / Editar Atendimento -->
	{#if modalAberto}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-3xl max-w-lg w-full p-6 shadow-2xl space-y-4 animate-in fade-in zoom-in-95 duration-150">
				<div class="flex items-center justify-between border-b border-slate-100 pb-3">
					<h3 class="font-bold text-slate-800 text-lg">
						{formId ? 'Editar Atendimento' : 'Novo Atendimento'}
					</h3>
					<button onclick={() => modalAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="space-y-3.5">
					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Nome do Cliente *</label>
						<input 
							type="text" 
							bind:value={formCliente} 
							placeholder="Ex: João da Silva / Secretaria de Saúde"
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
							autofocus
						/>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Data e Hora do Atendimento</label>
						<input 
							type="datetime-local" 
							bind:value={formData}
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
						/>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">O que foi feito? *</label>
						<textarea 
							bind:value={formDescricao}
							rows="4"
							placeholder="Descreva detalhadamente o serviço ou suporte prestado..."
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
						onclick={salvarAtendimento}
						class="px-5 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs"
					>
						{formId ? 'Salvar Alterações' : 'Registrar Atendimento'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal Detalhes do Atendimento -->
	{#if modalDetalhesAberto && atendimentoSelecionado}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-3xl max-w-lg w-full p-6 shadow-2xl space-y-4">
				<div class="flex items-start justify-between">
					<div>
						<span class="text-xs text-slate-400 font-semibold uppercase tracking-wider">Cliente</span>
						<h3 class="font-bold text-slate-800 text-xl leading-snug">{atendimentoSelecionado.cliente_nome}</h3>
					</div>
					<button onclick={() => modalDetalhesAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="space-y-3 text-sm text-slate-600 bg-slate-50 p-4 rounded-2xl border border-slate-100">
					<div>
						<span class="text-xs font-semibold text-slate-400">Atendido por:</span>
						<div class="flex items-center gap-2 mt-0.5 font-medium text-slate-800">
							<span class="w-2.5 h-2.5 rounded-full" style="background-color: {atendimentoSelecionado.usuario_cor};"></span>
							<span>{atendimentoSelecionado.usuario_nome}</span>
						</div>
					</div>

					<div>
						<span class="text-xs font-semibold text-slate-400">Data e Horário:</span>
						<div class="text-slate-800 font-medium mt-0.5">
							{new Date(atendimentoSelecionado.data_atendimento).toLocaleString('pt-BR')}
						</div>
					</div>

					<div>
						<span class="text-xs font-semibold text-slate-400">Descrição do que foi realizado:</span>
						<div class="mt-1 text-slate-800 whitespace-pre-wrap leading-relaxed">
							{atendimentoSelecionado.descricao}
						</div>
					</div>

					{#if atendimentoSelecionado.atualizado_em !== atendimentoSelecionado.criado_em}
						<div class="text-[11px] text-slate-400 pt-2 border-t border-slate-200">
							Editado em: {new Date(atendimentoSelecionado.atualizado_em).toLocaleString('pt-BR')}
						</div>
					{/if}
				</div>

				<div class="flex items-center justify-between pt-2 border-t border-slate-100">
					{#if atendimentoSelecionado.pode_editar}
						<button 
							onclick={() => excluirAtendimento(atendimentoSelecionado!.id)}
							class="flex items-center gap-1.5 px-3 py-2 text-xs text-red-600 hover:bg-red-50 rounded-xl font-medium"
						>
							<Trash2 class="w-4 h-4" />
							<span>Excluir</span>
						</button>
						<button 
							onclick={() => abrirEditar(atendimentoSelecionado!)}
							class="flex items-center gap-1.5 px-4 py-2 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs"
						>
							<Edit3 class="w-4 h-4" />
							<span>Editar</span>
						</button>
					{:else}
						<div class="text-xs text-slate-400 italic">Visualização apenas</div>
						<button 
							onclick={() => modalDetalhesAberto = false}
							class="px-4 py-2 text-xs bg-slate-100 hover:bg-slate-200 text-slate-700 rounded-xl font-semibold ml-auto"
						>
							Fechar
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</div>
