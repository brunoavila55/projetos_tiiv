<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { toast } from '$lib/toast.svelte';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { 
		Package, 
		Plus, 
		Search, 
		ArrowDownRight, 
		ArrowUpRight, 
		SlidersHorizontal, 
		AlertTriangle, 
		History, 
		Boxes, 
		Edit3, 
		X, 
		Check, 
		Filter, 
		User, 
		Clock, 
		Calendar,
		Download,
		Trash2
	} from 'lucide-svelte';

	interface ItemEstoque {
		id: string;
		nome: string;
		unidade: string;
		categoria: string;
		estoque_minimo: number;
		saldo: number;
		ativo: boolean;
		criado_em: string;
		abaixo_do_minimo: boolean;
	}

	interface MovimentacaoItem {
		id: string;
		item_id: string;
		item_nome: string;
		item_unidade: string;
		tipo: 'entrada' | 'saida' | 'ajuste';
		quantidade: number;
		saldo_resultante: number;
		motivo: string;
		usuario_id: string;
		usuario_nome: string;
		criado_em: string;
	}

	let abaAtiva = $state<'itens' | 'historico'>('itens');
	let itens = $state<ItemEstoque[]>([]);
	let movimentacoes = $state<MovimentacaoItem[]>([]);
	let categorias = $state<string[]>([]);
	let usuarios = $state<UsuarioPublico[]>([]);
	let loading = $state(true);

	// Filtros da aba Itens
	let buscaItens = $state('');
	let categoriaSelecionada = $state('');
	let mostrarDesativados = $state(false);

	// Filtros da aba Histórico
	let histItemId = $state('');
	let histTipo = $state('');
	let histUsuarioId = $state('');
	let histInicio = $state('');
	let histFim = $state('');

	// Modais
	let modalItemAberto = $state(false);
	let modalMovimentoAberto = $state(false);
	let itemSelecionado = $state<ItemEstoque | null>(null);

	// Form Item (Admin)
	let formItemId = $state<string | null>(null);
	let formItemNome = $state('');
	let formItemUnidade = $state('un');
	let formItemCategoria = $state('Geral');
	let formItemMinimo = $state<number>(5);
	let formItemAtivo = $state(true);

	// Form Movimentação
	let movTipo = $state<'entrada' | 'saida' | 'ajuste'>('entrada');
	let movQtd = $state<number>(1);
	let movMotivo = $state('');

	async function carregarItens() {
		loading = true;
		try {
			const params = new URLSearchParams();
			if (buscaItens.trim()) params.set('busca', buscaItens.trim());
			if (categoriaSelecionada) params.set('categoria', categoriaSelecionada);
			if (!mostrarDesativados) params.set('ativo', 'true');
			itens = await apiFetch<ItemEstoque[]>(`/api/estoque/itens?${params.toString()}`);
		} catch (err) {
			console.error('Erro ao listar itens:', err);
		} finally {
			loading = false;
		}
	}

	async function carregarHistorico() {
		loading = true;
		try {
			const params = new URLSearchParams();
			if (histItemId) params.set('item_id', histItemId);
			if (histTipo) params.set('tipo', histTipo);
			if (histUsuarioId) params.set('usuario_id', histUsuarioId);
			if (histInicio) params.set('inicio', histInicio);
			if (histFim) params.set('fim', histFim);
			movimentacoes = await apiFetch<MovimentacaoItem[]>(`/api/estoque/movimentacoes?${params.toString()}`);
		} catch (err) {
			console.error('Erro ao listar movimentações:', err);
		} finally {
			loading = false;
		}
	}

	function exportarCSV() {
		const params = new URLSearchParams();
		if (histItemId) params.set('item_id', histItemId);
		if (histUsuarioId) params.set('usuario_id', histUsuarioId);
		if (histTipo) params.set('tipo', histTipo);
		if (histInicio) params.set('inicio', histInicio);
		if (histFim) params.set('fim', histFim);

		const url = `/api/estoque/movimentacoes/exportar.csv?${params.toString()}`;
		window.open(url, '_blank');
	}

	async function carregarAuxiliares() {
		try {
			const [cats, users] = await Promise.all([
				apiFetch<string[]>('/api/estoque/categorias'),
				apiFetch<UsuarioPublico[]>('/api/auth/usuarios')
			]);
			categorias = cats;
			usuarios = users;
		} catch (err) {
			console.error('Erro ao carregar dados auxiliares:', err);
		}
	}

	function abrirCriarItem() {
		formItemId = null;
		formItemNome = '';
		formItemUnidade = 'un';
		formItemCategoria = 'Geral';
		formItemMinimo = 5;
		formItemAtivo = true;
		modalItemAberto = true;
	}

	function abrirEditarItem(it: ItemEstoque) {
		formItemId = it.id;
		formItemNome = it.nome;
		formItemUnidade = it.unidade;
		formItemCategoria = it.categoria;
		formItemMinimo = it.estoque_minimo;
		formItemAtivo = it.ativo;
		modalItemAberto = true;
	}

	async function salvarItem() {
		if (!formItemNome.trim() || !formItemUnidade.trim()) {
			alert('Nome e unidade do material são obrigatórios.');
			return;
		}

		const payload = {
			nome: formItemNome.trim(),
			unidade: formItemUnidade.trim(),
			categoria: formItemCategoria.trim() || 'Geral',
			estoque_minimo: Number(formItemMinimo) || 0,
			ativo: formItemAtivo
		};

		try {
			if (formItemId) {
				await apiFetch(`/api/estoque/itens/${formItemId}`, {
					method: 'PUT',
					body: JSON.stringify(payload)
				});
			} else {
				await apiFetch('/api/estoque/itens', {
					method: 'POST',
					body: JSON.stringify(payload)
				});
			}
			modalItemAberto = false;
			await carregarItens();
			await carregarAuxiliares();
		} catch (err: any) {
			alert(err.message || 'Erro ao salvar item');
		}
	}

	async function excluirItem(it: ItemEstoque) {
		if (!confirm(`Excluir "${it.nome}"?\n\nSe o item já teve movimentações, ele será desativado para preservar o histórico.`)) return;
		try {
			const res = await apiFetch<{ excluido: boolean; message: string }>(`/api/estoque/itens/${it.id}`, { method: 'DELETE' });
			if (res.excluido) toast.success(`"${it.nome}" excluído`);
			else toast.success(`"${it.nome}": ${res.message}`, 6000);
			await Promise.all([carregarItens(), carregarAuxiliares()]);
		} catch {
			// toast de erro já exibido por apiFetch
		}
	}

	function abrirMovimentar(it: ItemEstoque) {
		itemSelecionado = it;
		movTipo = 'entrada';
		movQtd = 1;
		movMotivo = '';
		modalMovimentoAberto = true;
	}

	async function salvarMovimentacao() {
		if (!itemSelecionado) return;

		if ((movTipo === 'saida' || movTipo === 'ajuste') && !movMotivo.trim()) {
			alert(`O motivo é obrigatório para ${movTipo}.`);
			return;
		}

		if (movTipo === 'saida' && movQtd > itemSelecionado.saldo) {
			alert(`Saldo insuficiente! Disponível: ${itemSelecionado.saldo} ${itemSelecionado.unidade}.`);
			return;
		}

		try {
			await apiFetch('/api/estoque/movimentacoes', {
				method: 'POST',
				body: JSON.stringify({
					item_id: itemSelecionado.id,
					tipo: movTipo,
					quantidade: Number(movQtd),
					motivo: movMotivo.trim()
				})
			});
			modalMovimentoAberto = false;
			await carregarItens();
		} catch (err: any) {
			alert(err.message || 'Erro ao registrar movimentação');
		}
	}

	function verHistoricoItem(it: ItemEstoque) {
		histItemId = it.id;
		abaAtiva = 'historico';
		carregarHistorico();
	}

	onMount(() => {
		carregarAuxiliares();
		carregarItens();
	});

	$effect(() => {
		if (abaAtiva === 'itens') {
			carregarItens();
		} else {
			carregarHistorico();
		}
	});
</script>

{#snippet tipoMov(valor: 'entrada' | 'saida' | 'ajuste', rotulo: string, detalhe: string, Icon: any, cor: string)}
	<button
		type="button"
		onclick={() => movTipo = valor}
		aria-pressed={movTipo === valor}
		class="flex flex-col items-start gap-1 p-3 rounded-lg border text-left cursor-pointer transition-colors {movTipo === valor
			? 'border-current bg-surface ring-1 ring-current ' + cor
			: 'border-line-strong text-ink-2 hover:bg-sunken'}"
	>
		<Icon class="size-4" />
		<span class="text-sm font-bold {movTipo === valor ? '' : 'text-ink'}">{rotulo}</span>
		<span class="text-xs text-ink-3">{detalhe}</span>
	</button>
{/snippet}

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Estoque</h1>
			<p class="page-sub">Saldos calculados a partir de cada entrada, saída e ajuste registrados.</p>
		</div>
		{#if auth.user?.papel === 'admin'}
			<button onclick={abrirCriarItem} class="btn btn-primary">
				<Plus class="size-4" />
				<span>Cadastrar item</span>
			</button>
		{/if}
	</div>

	<div class="segmented" role="group" aria-label="Seção">
		<button aria-pressed={abaAtiva === 'itens'} onclick={() => abaAtiva = 'itens'}>
			<Boxes class="size-4" />
			<span>Itens e saldos</span>
		</button>
		<button aria-pressed={abaAtiva === 'historico'} onclick={() => abaAtiva = 'historico'}>
			<History class="size-4" />
			<span>Movimentações</span>
		</button>
	</div>

	{#if abaAtiva === 'itens'}
		<div class="flex flex-col sm:flex-row gap-3">
			<div class="relative flex-1">
				<Search class="size-4 text-ink-3 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
				<input
					type="search"
					bind:value={buscaItens}
					oninput={carregarItens}
					placeholder="Buscar por nome ou categoria"
					class="field pl-9"
					aria-label="Buscar itens"
				/>
			</div>
			<select bind:value={categoriaSelecionada} onchange={carregarItens} class="field sm:w-56" aria-label="Categoria">
				<option value="">Todas as categorias</option>
				{#each categorias as cat}
					<option value={cat}>{cat}</option>
				{/each}
			</select>
			<label class="flex items-center gap-2 text-sm text-ink-2 cursor-pointer whitespace-nowrap">
				<input type="checkbox" bind:checked={mostrarDesativados} onchange={carregarItens} class="check" />
				Mostrar desativados
			</label>
		</div>

		{#if loading}
			<div class="flex justify-center py-20"><div class="spinner"></div></div>
		{:else if itens.length === 0}
			<div class="panel empty">
				<Package class="size-9 text-ink-3" strokeWidth={1.5} />
				<h3 class="empty-title">Nenhum item encontrado</h3>
				<p class="empty-text">Ajuste a busca ou cadastre um novo material.</p>
			</div>
		{:else}
			<div class="panel overflow-hidden">
				<div class="overflow-x-auto">
					<table class="data-table">
						<thead>
							<tr>
								<th>Material</th>
								<th>Categoria</th>
								<th class="text-right">Saldo</th>
								<th class="text-right">Mínimo</th>
								<th class="text-right"><span class="sr-only">Ações</span></th>
							</tr>
						</thead>
						<tbody>
							{#each itens as it (it.id)}
								<tr class={it.ativo ? '' : 'opacity-60'}>
									<td>
										<div class="flex items-center gap-2">
											<span class="font-semibold text-ink">{it.nome}</span>
											{#if !it.ativo}
												<span class="tag">Desativado</span>
											{/if}
											{#if it.abaixo_do_minimo}
												<span class="tag tag-danger" title="Saldo abaixo do mínimo">
													<AlertTriangle class="size-3" />
													Baixo
												</span>
											{/if}
										</div>
									</td>
									<td class="text-ink-2">{it.categoria || '—'}</td>
									<td class="text-right whitespace-nowrap">
										<span class="text-base font-bold {it.abaixo_do_minimo ? 'text-danger' : 'text-ink'}">{it.saldo}</span>
										<span class="text-[13px] text-ink-3 ml-0.5">{it.unidade}</span>
									</td>
									<td class="text-right text-ink-3 whitespace-nowrap">{it.estoque_minimo} {it.unidade}</td>
									<td class="text-right whitespace-nowrap">
										<div class="inline-flex items-center gap-0.5">
											{#if it.ativo}
												<button onclick={() => abrirMovimentar(it)} class="btn btn-sm btn-soft mr-1" title="Registrar entrada, saída ou ajuste">
													Movimentar
												</button>
											{/if}
											<button onclick={() => verHistoricoItem(it)} class="icon-btn" title="Histórico do item" aria-label="Histórico do item">
												<History class="size-4" />
											</button>
											{#if auth.user?.papel === 'admin'}
												<button onclick={() => abrirEditarItem(it)} class="icon-btn" title="Editar item" aria-label="Editar item">
													<Edit3 class="size-4" />
												</button>
												{#if it.ativo}
													<button onclick={() => excluirItem(it)} class="icon-btn icon-btn-danger" title="Excluir item" aria-label="Excluir item">
														<Trash2 class="size-4" />
													</button>
												{/if}
											{/if}
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	{:else}
		<!-- Filtros do histórico -->
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_1fr_auto] gap-3 items-end">
			<div>
				<label class="label" for="h-item">Item</label>
				<select id="h-item" bind:value={histItemId} onchange={carregarHistorico} class="field">
					<option value="">Todos os itens</option>
					{#each itens as it}
						<option value={it.id}>{it.nome}</option>
					{/each}
				</select>
			</div>
			<div>
				<label class="label" for="h-tipo">Operação</label>
				<select id="h-tipo" bind:value={histTipo} onchange={carregarHistorico} class="field">
					<option value="">Todas</option>
					<option value="entrada">Entradas</option>
					<option value="saida">Saídas</option>
					<option value="ajuste">Ajustes de inventário</option>
				</select>
			</div>
			<div>
				<label class="label" for="h-op">Operador</label>
				<select id="h-op" bind:value={histUsuarioId} onchange={carregarHistorico} class="field">
					<option value="">Todos os operadores</option>
					{#each usuarios as u}
						<option value={u.id}>{u.nome}</option>
					{/each}
				</select>
			</div>
			<div class="flex gap-2">
				<button
					onclick={() => {
						histItemId = '';
						histTipo = '';
						histUsuarioId = '';
						histInicio = '';
						histFim = '';
						carregarHistorico();
					}}
					class="btn btn-ghost h-10"
				>
					Limpar
				</button>
				<button onclick={exportarCSV} class="btn btn-secondary h-10" title="Exporta as movimentações filtradas">
					<Download class="size-4" />
					<span>CSV</span>
				</button>
			</div>
		</div>

		{#if loading}
			<div class="flex justify-center py-20"><div class="spinner"></div></div>
		{:else if movimentacoes.length === 0}
			<div class="panel empty">
				<History class="size-9 text-ink-3" strokeWidth={1.5} />
				<h3 class="empty-title">Nenhuma movimentação</h3>
				<p class="empty-text">Entradas, saídas e ajustes aparecem aqui assim que forem registrados. O histórico não pode ser alterado.</p>
			</div>
		{:else}
			<div class="panel overflow-hidden">
				<div class="overflow-x-auto">
					<table class="data-table">
						<thead>
							<tr>
								<th>Quando</th>
								<th>Material</th>
								<th>Operação</th>
								<th class="text-right">Qtd.</th>
								<th class="text-right">Saldo após</th>
								<th>Motivo</th>
								<th>Operador</th>
							</tr>
						</thead>
						<tbody>
							{#each movimentacoes as m (m.id)}
								<tr>
									<td class="text-[13px] text-ink-3 whitespace-nowrap">
										{new Date(m.criado_em).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })}
									</td>
									<td class="font-semibold text-ink">{m.item_nome}</td>
									<td>
										{#if m.tipo === 'entrada'}
											<span class="tag tag-ok"><ArrowDownRight class="size-3.5" /> Entrada</span>
										{:else if m.tipo === 'saida'}
											<span class="tag tag-danger"><ArrowUpRight class="size-3.5" /> Saída</span>
										{:else}
											<span class="tag tag-warn"><SlidersHorizontal class="size-3.5" /> Ajuste</span>
										{/if}
									</td>
									<td class="text-right font-semibold whitespace-nowrap {m.tipo === 'entrada' ? 'text-ok' : m.tipo === 'saida' ? 'text-danger' : 'text-ink'}">
										{m.quantidade > 0 && m.tipo === 'entrada' ? `+${m.quantidade}` : m.quantidade}
									</td>
									<td class="text-right whitespace-nowrap">
										<span class="font-bold text-ink">{m.saldo_resultante}</span>
										<span class="text-[13px] text-ink-3">{m.item_unidade}</span>
									</td>
									<td class="text-ink-2 max-w-xs truncate" title={m.motivo}>{m.motivo || '—'}</td>
									<td class="text-ink-2 whitespace-nowrap">{m.usuario_nome}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	{/if}

	<!-- Modal movimentação -->
	{#if modalMovimentoAberto && itemSelecionado}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div>
						<h3 class="modal-title">{itemSelecionado.nome}</h3>
						<p class="mt-0.5 text-sm text-ink-3">
							Saldo atual: <strong class="text-ink tabular">{itemSelecionado.saldo} {itemSelecionado.unidade}</strong>
						</p>
					</div>
					<button onclick={() => modalMovimentoAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div class="grid grid-cols-3 gap-2" role="group" aria-label="Tipo de movimentação">
						{@render tipoMov('entrada', 'Entrada', 'Soma ao saldo', ArrowDownRight, 'text-ok')}
						{@render tipoMov('saida', 'Saída', 'Tira do saldo', ArrowUpRight, 'text-danger')}
						{@render tipoMov('ajuste', 'Ajuste', 'Define a contagem', SlidersHorizontal, 'text-warn')}
					</div>

					<div>
						<label class="label" for="mov-qtd">
							{movTipo === 'ajuste' ? 'Quantidade contada' : 'Quantidade'}
							<span class="font-normal text-ink-3">({itemSelecionado.unidade})</span>
						</label>
						<input
							id="mov-qtd"
							type="number"
							bind:value={movQtd}
							min="0"
							class="field h-12 text-xl font-bold tabular"
						/>
						{#if movTipo === 'ajuste'}
							<p class="hint tabular">
								Diferença: <strong class="text-ink">{movQtd - itemSelecionado.saldo > 0 ? '+' : ''}{movQtd - itemSelecionado.saldo}</strong> {itemSelecionado.unidade}
							</p>
						{/if}
					</div>

					<div>
						<label class="label" for="mov-motivo">
							Motivo {#if movTipo === 'entrada'}<span class="font-normal text-ink-3">(opcional)</span>{/if}
						</label>
						<input
							id="mov-motivo"
							type="text"
							bind:value={movMotivo}
							placeholder={movTipo === 'saida' ? 'Ex.: Usado na manutenção da recepção' : movTipo === 'ajuste' ? 'Ex.: Inventário mensal' : 'Ex.: Reposição de compra'}
							class="field"
						/>
					</div>
				</div>

				<div class="modal-foot">
					<button onclick={() => modalMovimentoAberto = false} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarMovimentacao} class="btn btn-primary">
						Registrar {movTipo === 'entrada' ? 'entrada' : movTipo === 'saida' ? 'saída' : 'ajuste'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal item (admin) -->
	{#if modalItemAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{formItemId ? 'Editar item' : 'Cadastrar item'}</h3>
					<button onclick={() => modalItemAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div>
						<label class="label" for="it-nome">Nome do material</label>
						<input id="it-nome" type="text" bind:value={formItemNome} placeholder="Ex.: Cabo de rede Cat6" class="field" />
					</div>

					<div class="grid grid-cols-2 gap-4">
						<div>
							<label class="label" for="it-un">Unidade</label>
							<input id="it-un" type="text" bind:value={formItemUnidade} placeholder="un, cx, m, kg" class="field" />
						</div>
						<div>
							<label class="label" for="it-min">Estoque mínimo</label>
							<input id="it-min" type="number" bind:value={formItemMinimo} min="0" class="field tabular" />
						</div>
					</div>

					<div>
						<label class="label" for="it-cat">Categoria</label>
						<input id="it-cat" type="text" bind:value={formItemCategoria} placeholder="Ex.: Rede, Elétrica, Papelaria" class="field" />
					</div>

					{#if formItemId}
						<label class="flex items-center gap-2.5 text-sm text-ink cursor-pointer">
							<input type="checkbox" bind:checked={formItemAtivo} class="check" />
							Item ativo para movimentações
						</label>
					{/if}
				</div>

				<div class="modal-foot">
					<button onclick={() => modalItemAberto = false} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarItem} class="btn btn-primary">
						{formItemId ? 'Salvar alterações' : 'Cadastrar item'}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
