<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
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
		Calendar
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

<div class="space-y-6">
	<!-- Topo com Abas e Botão Novo Item -->
	<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Controle de Estoque</h1>
			<p class="text-sm text-slate-500">Gestão transacional imutável baseada em movimentações e bloqueio pessimista</p>
		</div>

		<div class="flex items-center gap-3">
			{#if auth.user?.papel === 'admin'}
				<button
					onclick={abrirCriarItem}
					class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-semibold text-sm shadow-sm transition active:scale-95 cursor-pointer"
				>
					<Plus class="w-4 h-4" />
					<span>Novo Item</span>
				</button>
			{/if}
		</div>
	</div>

	<!-- Barra de Navegação de Abas -->
	<div class="flex items-center bg-white p-1.5 rounded-2xl border border-slate-200 shadow-xs max-w-sm">
		<button
			onclick={() => abaAtiva = 'itens'}
			class="flex-1 flex items-center justify-center gap-2 py-2 rounded-xl text-xs font-semibold transition cursor-pointer {abaAtiva === 'itens' ? 'bg-blue-600 text-white shadow-xs' : 'text-slate-600 hover:text-slate-800'}"
		>
			<Boxes class="w-4 h-4" />
			<span>Itens e Saldos</span>
		</button>
		<button
			onclick={() => abaAtiva = 'historico'}
			class="flex-1 flex items-center justify-center gap-2 py-2 rounded-xl text-xs font-semibold transition cursor-pointer {abaAtiva === 'historico' ? 'bg-blue-600 text-white shadow-xs' : 'text-slate-600 hover:text-slate-800'}"
		>
			<History class="w-4 h-4" />
			<span>Movimentações</span>
		</button>
	</div>

	{#if abaAtiva === 'itens'}
		<!-- Filtros de Itens -->
		<div class="flex flex-col sm:flex-row items-center gap-3 bg-white p-4 rounded-2xl border border-slate-200 shadow-xs">
			<div class="relative flex-1 w-full">
				<Search class="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
				<input
					type="text"
					bind:value={buscaItens}
					oninput={carregarItens}
					placeholder="Buscar item de estoque por nome ou categoria..."
					class="w-full pl-10 pr-4 py-2 rounded-xl border border-slate-200 text-sm focus:outline-blue-500 bg-slate-50/50"
				/>
			</div>

			<div class="w-full sm:w-auto">
				<select
					bind:value={categoriaSelecionada}
					onchange={carregarItens}
					class="w-full sm:w-48 px-3 py-2 border border-slate-200 rounded-xl text-xs sm:text-sm font-medium focus:outline-blue-500 bg-white"
				>
					<option value="">Todas as categorias</option>
					{#each categorias as cat}
						<option value={cat}>{cat}</option>
					{/each}
				</select>
			</div>
		</div>

		<!-- Tabela de Itens de Estoque -->
		{#if loading}
			<div class="flex justify-center py-20">
				<div class="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
			</div>
		{:else if itens.length === 0}
			<div class="bg-white rounded-2xl border border-slate-200 p-12 text-center max-w-md mx-auto shadow-xs">
				<Package class="w-12 h-12 text-slate-300 mx-auto mb-3" />
				<h3 class="text-base font-bold text-slate-800">Nenhum item encontrado</h3>
				<p class="text-xs text-slate-500 mt-1">Cadastre novos materiais ou redefina os filtros de busca.</p>
			</div>
		{:else}
			<div class="bg-white rounded-2xl border border-slate-200 overflow-hidden shadow-xs">
				<div class="overflow-x-auto">
					<table class="w-full text-left border-collapse text-sm">
						<thead>
							<tr class="bg-slate-50/75 border-b border-slate-200 text-slate-600 font-semibold">
								<th class="py-3 px-4">Material</th>
								<th class="py-3 px-4">Categoria</th>
								<th class="py-3 px-4">Estoque Mínimo</th>
								<th class="py-3 px-4">Saldo Atual</th>
								<th class="py-3 px-4 text-right">Ações</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-slate-100">
							{#each itens as it (it.id)}
								<tr class="hover:bg-slate-50/50 transition {it.abaixo_do_minimo ? 'bg-amber-50/20' : ''}">
									<td class="py-3.5 px-4 font-semibold text-slate-800">
										<div class="flex items-center gap-2">
											<span>{it.nome}</span>
											{#if it.abaixo_do_minimo}
												<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-red-100 text-red-700 animate-pulse" title="Saldo abaixo do estoque mínimo!">
													<AlertTriangle class="w-3 h-3" />
													<span>Crítico</span>
												</span>
											{/if}
										</div>
									</td>
									<td class="py-3.5 px-4 text-slate-600">
										<span class="inline-flex px-2.5 py-0.5 rounded-md text-xs font-medium bg-slate-100 text-slate-700">
											{it.categoria}
										</span>
									</td>
									<td class="py-3.5 px-4 text-slate-500 text-xs">
										{it.estoque_minimo} {it.unidade}
									</td>
									<td class="py-3.5 px-4">
										<span class="font-extrabold text-base {it.abaixo_do_minimo ? 'text-red-600' : 'text-slate-800'}">
											{it.saldo}
										</span>
										<span class="text-xs text-slate-500 ml-1">{it.unidade}</span>
									</td>
									<td class="py-3.5 px-4 text-right space-x-1">
										<!-- Botão Movimentar (qualquer operador) -->
										<button
											onclick={() => abrirMovimentar(it)}
											class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg bg-blue-50 hover:bg-blue-100 text-blue-700 text-xs font-bold transition"
											title="Registrar entrada, saída ou ajuste"
										>
											<span>Movimentar</span>
										</button>

										<!-- Botão Histórico do Item -->
										<button
											onclick={() => verHistoricoItem(it)}
											class="p-2 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-lg transition"
											title="Ver histórico de movimentações"
										>
											<History class="w-4 h-4" />
										</button>

										<!-- Botão Editar Item (Admin) -->
										{#if auth.user?.papel === 'admin'}
											<button
												onclick={() => abrirEditarItem(it)}
												class="p-2 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition"
												title="Editar item de estoque"
											>
												<Edit3 class="w-4 h-4" />
											</button>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	{:else}
		<!-- Aba Histórico de Movimentações -->
		<div class="bg-white p-4 rounded-2xl border border-slate-200 shadow-xs space-y-3">
			<div class="grid grid-cols-1 sm:grid-cols-4 gap-3">
				<div>
					<label class="block text-xs font-semibold text-slate-700 mb-1">Filtrar por Item</label>
					<select
						bind:value={histItemId}
						onchange={carregarHistorico}
						class="w-full px-3 py-2 border border-slate-200 rounded-xl text-xs font-medium focus:outline-blue-500 bg-white"
					>
						<option value="">Todos os itens</option>
						{#each itens as it}
							<option value={it.id}>{it.nome}</option>
						{/each}
					</select>
				</div>

				<div>
					<label class="block text-xs font-semibold text-slate-700 mb-1">Tipo de Operação</label>
					<select
						bind:value={histTipo}
						onchange={carregarHistorico}
						class="w-full px-3 py-2 border border-slate-200 rounded-xl text-xs font-medium focus:outline-blue-500 bg-white"
					>
						<option value="">Todos os tipos</option>
						<option value="entrada">Entradas (+)</option>
						<option value="saida">Saídas (-)</option>
						<option value="ajuste">Ajustes de inventário</option>
					</select>
				</div>

				<div>
					<label class="block text-xs font-semibold text-slate-700 mb-1">Operador</label>
					<select
						bind:value={histUsuarioId}
						onchange={carregarHistorico}
						class="w-full px-3 py-2 border border-slate-200 rounded-xl text-xs font-medium focus:outline-blue-500 bg-white"
					>
						<option value="">Todos os operadores</option>
						{#each usuarios as u}
							<option value={u.id}>{u.nome}</option>
						{/each}
					</select>
				</div>

				<div>
					<label class="block text-xs font-semibold text-slate-700 mb-1">Ações</label>
					<button
						onclick={() => {
							histItemId = '';
							histTipo = '';
							histUsuarioId = '';
							histInicio = '';
							histFim = '';
							carregarHistorico();
						}}
						class="w-full py-2 px-3 border border-slate-200 hover:bg-slate-100 rounded-xl text-xs font-semibold text-slate-600 transition"
					>
						Limpar Filtros
					</button>
				</div>
			</div>
		</div>

		<!-- Tabela do Histórico Imutável -->
		{#if loading}
			<div class="flex justify-center py-20">
				<div class="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
			</div>
		{:else if movimentacoes.length === 0}
			<div class="bg-white rounded-2xl border border-slate-200 p-12 text-center max-w-md mx-auto shadow-xs">
				<History class="w-12 h-12 text-slate-300 mx-auto mb-3" />
				<h3 class="text-base font-bold text-slate-800">Nenhuma movimentação registrada</h3>
				<p class="text-xs text-slate-500 mt-1">As entradas, saídas e ajustes realizados ficarão registrados de forma imutável aqui.</p>
			</div>
		{:else}
			<div class="bg-white rounded-2xl border border-slate-200 overflow-hidden shadow-xs">
				<div class="overflow-x-auto">
					<table class="w-full text-left border-collapse text-sm">
						<thead>
							<tr class="bg-slate-50/75 border-b border-slate-200 text-slate-600 font-semibold">
								<th class="py-3 px-4">Data/Hora</th>
								<th class="py-3 px-4">Material</th>
								<th class="py-3 px-4">Operação</th>
								<th class="py-3 px-4">Qtd.</th>
								<th class="py-3 px-4">Saldo Resultante</th>
								<th class="py-3 px-4">Motivo</th>
								<th class="py-3 px-4">Operador</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-slate-100">
							{#each movimentacoes as m (m.id)}
								<tr class="hover:bg-slate-50/50 transition">
									<td class="py-3.5 px-4 text-xs text-slate-500 whitespace-nowrap">
										{new Date(m.criado_em).toLocaleString('pt-BR')}
									</td>
									<td class="py-3.5 px-4 font-semibold text-slate-800">
										{m.item_nome}
									</td>
									<td class="py-3.5 px-4">
										{#if m.tipo === 'entrada'}
											<span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-bold bg-emerald-100 text-emerald-700">
												<ArrowDownRight class="w-3.5 h-3.5" /> Entrada
											</span>
										{:else if m.tipo === 'saida'}
											<span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-bold bg-red-100 text-red-700">
												<ArrowUpRight class="w-3.5 h-3.5" /> Saída
											</span>
										{:else}
											<span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-bold bg-amber-100 text-amber-700">
												<SlidersHorizontal class="w-3.5 h-3.5" /> Ajuste
											</span>
										{/if}
									</td>
									<td class="py-3.5 px-4 font-bold text-slate-800">
										{m.quantidade > 0 && m.tipo === 'entrada' ? `+${m.quantidade}` : m.quantidade} {m.item_unidade}
									</td>
									<td class="py-3.5 px-4 font-extrabold text-blue-700">
										{m.saldo_resultante} {m.item_unidade}
									</td>
									<td class="py-3.5 px-4 text-xs text-slate-600 max-w-xs truncate" title={m.motivo}>
										{m.motivo || '—'}
									</td>
									<td class="py-3.5 px-4 text-xs font-medium text-slate-700">
										{m.usuario_nome}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	{/if}

	<!-- Modal Registrar Movimentação (Entrada, Saída, Ajuste) -->
	{#if modalMovimentoAberto && itemSelecionado}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-3xl max-w-md w-full p-6 shadow-2xl space-y-4 animate-in fade-in zoom-in-95 duration-150">
				<div class="flex items-center justify-between border-b border-slate-100 pb-3">
					<div>
						<span class="text-xs font-bold uppercase text-slate-400">Movimentação</span>
						<h3 class="font-bold text-slate-800 text-lg leading-tight">{itemSelecionado.nome}</h3>
					</div>
					<button onclick={() => modalMovimentoAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="p-3 bg-slate-50 rounded-xl text-xs flex items-center justify-between">
					<span class="text-slate-500 font-medium">Saldo Atual no Estoque:</span>
					<span class="font-bold text-sm text-slate-800">{itemSelecionado.saldo} {itemSelecionado.unidade}</span>
				</div>

				<!-- Seletor do Tipo de Movimento -->
				<div class="grid grid-cols-3 gap-2">
					<button
						type="button"
						onclick={() => movTipo = 'entrada'}
						class="py-2.5 rounded-xl border text-xs font-bold flex flex-col items-center gap-1 transition cursor-pointer {movTipo === 'entrada' ? 'bg-emerald-50 border-emerald-500 text-emerald-700 shadow-xs' : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50'}"
					>
						<ArrowDownRight class="w-4 h-4 text-emerald-600" />
						<span>Entrada (+)</span>
					</button>
					<button
						type="button"
						onclick={() => movTipo = 'saida'}
						class="py-2.5 rounded-xl border text-xs font-bold flex flex-col items-center gap-1 transition cursor-pointer {movTipo === 'saida' ? 'bg-red-50 border-red-500 text-red-700 shadow-xs' : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50'}"
					>
						<ArrowUpRight class="w-4 h-4 text-red-600" />
						<span>Saída (-)</span>
					</button>
					<button
						type="button"
						onclick={() => movTipo = 'ajuste'}
						class="py-2.5 rounded-xl border text-xs font-bold flex flex-col items-center gap-1 transition cursor-pointer {movTipo === 'ajuste' ? 'bg-amber-50 border-amber-500 text-amber-700 shadow-xs' : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50'}"
					>
						<SlidersHorizontal class="w-4 h-4 text-amber-600" />
						<span>Ajuste (=)</span>
					</button>
				</div>

				<div class="space-y-3">
					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">
							{#if movTipo === 'ajuste'}
								Novo Saldo Contado ({itemSelecionado.unidade}) *
							{:else}
								Quantidade ({itemSelecionado.unidade}) *
							{/if}
						</label>
						<input 
							type="number" 
							bind:value={movQtd} 
							min="0"
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-lg font-bold text-center focus:outline-blue-500" 
						/>
						{#if movTipo === 'ajuste'}
							<p class="text-[11px] text-slate-400 mt-1 text-center">
								Diferença calculada: <strong>{movQtd - itemSelecionado.saldo}</strong> {itemSelecionado.unidade}
							</p>
						{/if}
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">
							Motivo {movTipo !== 'entrada' ? '*' : '(opcional)'}
						</label>
						<input 
							type="text" 
							bind:value={movMotivo} 
							placeholder={movTipo === 'saida' ? 'Ex: Uso na sala de reuniões / atendimento #12' : movTipo === 'ajuste' ? 'Ex: Contagem de inventário mensal' : 'Ex: Compra ou reposição de material'}
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
						/>
					</div>
				</div>

				<div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
					<button 
						onclick={() => modalMovimentoAberto = false}
						class="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-xl font-medium"
					>
						Cancelar
					</button>
					<button 
						onclick={salvarMovimentacao}
						class="px-5 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs"
					>
						Confirmar Movimentação
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal Criar / Editar Item de Estoque (Admin) -->
	{#if modalItemAberto}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-3xl max-w-md w-full p-6 shadow-2xl space-y-4 animate-in fade-in zoom-in-95 duration-150">
				<div class="flex items-center justify-between border-b border-slate-100 pb-3">
					<h3 class="font-bold text-slate-800 text-lg">
						{formItemId ? 'Editar Item de Estoque' : 'Novo Item de Estoque'}
					</h3>
					<button onclick={() => modalItemAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="space-y-3">
					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Nome do Material *</label>
						<input 
							type="text" 
							bind:value={formItemNome} 
							placeholder="Ex: Cabo de Rede RJ45 Cat6"
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
						/>
					</div>

					<div class="grid grid-cols-2 gap-3">
						<div>
							<label class="block text-xs font-semibold text-slate-700 mb-1">Unidade *</label>
							<input 
								type="text" 
								bind:value={formItemUnidade} 
								placeholder="un, cx, m, kg"
								class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
							/>
						</div>
						<div>
							<label class="block text-xs font-semibold text-slate-700 mb-1">Estoque Mínimo</label>
							<input 
								type="number" 
								bind:value={formItemMinimo} 
								min="0"
								class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
							/>
						</div>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Categoria</label>
						<input 
							type="text" 
							bind:value={formItemCategoria} 
							placeholder="Ex: Rede, Elétrica, Papelaria..."
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
						/>
					</div>

					{#if formItemId}
						<div class="flex items-center gap-2 pt-1">
							<input 
								type="checkbox" 
								id="itemAtivo" 
								bind:checked={formItemAtivo}
								class="w-4 h-4 rounded text-blue-600"
							/>
							<label for="itemAtivo" class="text-xs font-semibold text-slate-700 cursor-pointer">
								Item Ativo para movimentações
							</label>
						</div>
					{/if}
				</div>

				<div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
					<button 
						onclick={() => modalItemAberto = false}
						class="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-xl font-medium"
					>
						Cancelar
					</button>
					<button 
						onclick={salvarItem}
						class="px-5 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs"
					>
						{formItemId ? 'Atualizar Item' : 'Cadastrar Item'}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
