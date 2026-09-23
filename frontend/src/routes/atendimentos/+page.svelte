<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { 
		Headphones, 
		Plus, 
		Search, 
		Edit3, 
		Trash2, 
		X, 
		ChevronLeft, 
		ChevronRight,
		Download
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

	function exportarCSV() {
		const params = new URLSearchParams();
		if (filtroBusca.trim()) params.set('busca', filtroBusca.trim());
		if (filtroUsuario) params.set('usuario_id', filtroUsuario);
		if (filtroInicio) params.set('inicio', filtroInicio);
		if (filtroFim) params.set('fim', filtroFim);

		const url = `/api/atendimentos/exportar.csv?${params.toString()}`;
		window.open(url, '_blank');
	}

	onMount(() => {
		carregarUsuarios();
		carregar();
	});
</script>

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Atendimentos</h1>
			<p class="page-sub">Registro de tudo o que o setor atendeu. A busca ignora acentos.</p>
		</div>

		<div class="flex items-center gap-2">
			<button onclick={exportarCSV} class="btn btn-secondary" title="Exporta os atendimentos filtrados">
				<Download class="size-4" />
				<span>Exportar CSV</span>
			</button>
			<button onclick={abrirCriar} class="btn btn-primary">
				<Plus class="size-4" />
				<span>Registrar atendimento</span>
			</button>
		</div>
	</div>

	<!-- Busca e filtros -->
	<div class="flex flex-col lg:flex-row lg:items-center gap-3">
		<div class="relative flex-1">
			<Search class="size-4 text-ink-3 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
			<input
				type="search"
				bind:value={filtroBusca}
				oninput={handleBuscaInput}
				placeholder="Buscar por cliente ou descrição"
				class="field pl-9"
				aria-label="Buscar atendimentos"
			/>
		</div>

		<div class="flex flex-wrap items-center gap-2">
			<select bind:value={filtroUsuario} onchange={() => carregar(true)} class="field w-auto min-w-40" aria-label="Operador">
				<option value="">Todos os operadores</option>
				{#each usuarios as u}
					<option value={u.id}>{u.nome}</option>
				{/each}
			</select>
			<input type="date" bind:value={filtroInicio} onchange={() => carregar(true)} class="field w-auto" aria-label="De" />
			<span class="text-sm text-ink-3">até</span>
			<input type="date" bind:value={filtroFim} onchange={() => carregar(true)} class="field w-auto" aria-label="Até" />
			{#if filtroBusca || filtroUsuario || filtroInicio || filtroFim}
				<button
					onclick={() => {
						filtroBusca = '';
						filtroUsuario = '';
						filtroInicio = '';
						filtroFim = '';
						carregar(true);
					}}
					class="btn btn-ghost"
				>
					Limpar filtros
				</button>
			{/if}
		</div>
	</div>

	<!-- Lista -->
	{#if loading}
		<div class="flex justify-center py-20"><div class="spinner"></div></div>
	{:else if atendimentos.length === 0}
		<div class="panel empty">
			<Headphones class="size-9 text-ink-3" strokeWidth={1.5} />
			<h3 class="empty-title">Nenhum atendimento encontrado</h3>
			<p class="empty-text">Ajuste a busca ou registre um novo atendimento.</p>
			<button onclick={abrirCriar} class="btn btn-secondary mt-5">
				<Plus class="size-4" />
				<span>Registrar atendimento</span>
			</button>
		</div>
	{:else}
		<ul class="panel divide-y divide-line overflow-hidden">
			{#each atendimentos as a (a.id)}
				<li
					class="group flex items-start gap-4 px-4 sm:px-5 py-4 hover:bg-sunken transition-colors cursor-pointer"
					onclick={() => abrirDetalhes(a)}
				>
					<time class="hidden sm:block w-20 shrink-0 pt-0.5 text-[13px] leading-tight text-ink-3">
						<span class="block font-semibold text-ink-2">{new Date(a.data_atendimento).toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: '2-digit' })}</span>
						{new Date(a.data_atendimento).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
					</time>

					<div class="flex-1 min-w-0">
						<div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
							<span class="font-bold text-ink text-[15px]">{a.cliente_nome}</span>
							<span class="inline-flex items-center gap-1.5 text-[13px] text-ink-3">
								<span class="dot size-2" style="background-color: {a.usuario_cor};"></span>
								{a.usuario_nome}
							</span>
							<time class="sm:hidden text-[13px] text-ink-3">{new Date(a.data_atendimento).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })}</time>
						</div>
						<p class="mt-1 text-sm text-ink-2 line-clamp-2 max-w-[75ch]">{a.descricao}</p>
					</div>

					{#if a.pode_editar}
						<div class="flex items-center gap-0.5 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-within:opacity-100 transition-opacity" onclick={(e) => e.stopPropagation()}>
							<button onclick={() => abrirEditar(a)} class="icon-btn" title="Editar" aria-label="Editar atendimento">
								<Edit3 class="size-4" />
							</button>
							<button onclick={() => excluirAtendimento(a.id)} class="icon-btn icon-btn-danger" title="Excluir" aria-label="Excluir atendimento">
								<Trash2 class="size-4" />
							</button>
						</div>
					{/if}
				</li>
			{/each}
		</ul>

		<!-- Paginação -->
		<div class="flex items-center justify-between text-[13px] text-ink-3">
			<span class="tabular">{totalRegistros} {totalRegistros === 1 ? 'atendimento' : 'atendimentos'}</span>
			<div class="flex items-center gap-1">
				<button
					disabled={paginaAtual <= 1}
					onclick={() => { paginaAtual--; carregar(); }}
					class="icon-btn disabled:opacity-30 disabled:pointer-events-none"
					aria-label="Página anterior"
				>
					<ChevronLeft class="size-4" />
				</button>
				<span class="px-2 font-semibold text-ink-2 tabular">{paginaAtual} de {totalPaginas}</span>
				<button
					disabled={paginaAtual >= totalPaginas}
					onclick={() => { paginaAtual++; carregar(); }}
					class="icon-btn disabled:opacity-30 disabled:pointer-events-none"
					aria-label="Próxima página"
				>
					<ChevronRight class="size-4" />
				</button>
			</div>
		</div>
	{/if}

	<!-- Modal registrar / editar -->
	{#if modalAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{formId ? 'Editar atendimento' : 'Registrar atendimento'}</h3>
					<button onclick={() => modalAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div>
						<label class="label" for="at-cliente">Cliente</label>
						<input
							id="at-cliente"
							type="text"
							bind:value={formCliente}
							placeholder="Pessoa ou órgão atendido"
							class="field"
							autofocus
						/>
					</div>

					<div>
						<label class="label" for="at-data">Data e hora</label>
						<input id="at-data" type="datetime-local" bind:value={formData} class="field" />
					</div>

					<div>
						<label class="label" for="at-desc">O que foi feito</label>
						<textarea
							id="at-desc"
							bind:value={formDescricao}
							rows="5"
							placeholder="Descreva o serviço ou suporte prestado"
							class="field"
						></textarea>
					</div>
				</div>

				<div class="modal-foot">
					<button onclick={() => modalAberto = false} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarAtendimento} class="btn btn-primary">
						{formId ? 'Salvar alterações' : 'Registrar'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal detalhes -->
	{#if modalDetalhesAberto && atendimentoSelecionado}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div class="min-w-0">
						<h3 class="modal-title">{atendimentoSelecionado.cliente_nome}</h3>
						<p class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] text-ink-3">
							<span class="inline-flex items-center gap-1.5">
								<span class="dot size-2" style="background-color: {atendimentoSelecionado.usuario_cor};"></span>
								{atendimentoSelecionado.usuario_nome}
							</span>
							<time class="tabular">{new Date(atendimentoSelecionado.data_atendimento).toLocaleString('pt-BR', { dateStyle: 'long', timeStyle: 'short' })}</time>
						</p>
					</div>
					<button onclick={() => modalDetalhesAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<p class="text-[15px] text-ink whitespace-pre-wrap leading-relaxed">{atendimentoSelecionado.descricao}</p>

					{#if atendimentoSelecionado.atualizado_em !== atendimentoSelecionado.criado_em}
						<p class="text-xs text-ink-3">
							Editado em {new Date(atendimentoSelecionado.atualizado_em).toLocaleString('pt-BR')}
						</p>
					{/if}
				</div>

				<div class="modal-foot">
					{#if atendimentoSelecionado.pode_editar}
						<button onclick={() => excluirAtendimento(atendimentoSelecionado!.id)} class="btn btn-danger mr-auto">
							<Trash2 class="size-4" />
							<span>Excluir</span>
						</button>
						<button onclick={() => abrirEditar(atendimentoSelecionado!)} class="btn btn-primary">
							<Edit3 class="size-4" />
							<span>Editar</span>
						</button>
					{:else}
						<span class="mr-auto text-[13px] text-ink-3">Só quem registrou ou um administrador pode editar.</span>
						<button onclick={() => modalDetalhesAberto = false} class="btn btn-secondary">Fechar</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</div>
