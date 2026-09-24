<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { diffLines, type Change } from 'diff';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { renderMarkdown } from '$lib/markdown';
	import {
		BookOpen,
		Plus,
		Search,
		X,
		Check,
		AlertCircle,
		ShieldAlert,
		Bold,
		Heading2,
		List,
		ListOrdered,
		Link as LinkIcon,
		RotateCcw
	} from 'lucide-svelte';

	interface ProcedimentoResumo {
		id: string;
		titulo: string;
		categoria: string;
		ativo: boolean;
		atualizado_em: string;
		atualizado_por_nome: string;
	}

	interface Procedimento extends ProcedimentoResumo {
		corpo: string;
		criado_em: string;
	}

	interface Revisao {
		id: string;
		titulo: string;
		categoria: string;
		corpo: string;
		ativo: boolean;
		nota: string;
		editado_por_nome: string;
		criado_em: string;
	}

	const MODELO = `## Como as pessoas descrevem
- "..."

## O que fazer
1. ...

## Contato
**Nome** · telefone · ramal · horário
`;

	// Lista
	let procedimentos = $state<ProcedimentoResumo[]>([]);
	let categorias = $state<string[]>([]);
	let loading = $state(true);
	let errorMsg = $state<string | null>(null);
	let successMsg = $state<string | null>(null);
	let busca = $state('');
	let filtroCategoria = $state('');
	let filtroSituacao = $state<'ativos' | 'inativos' | 'todos'>('ativos');

	// Editor
	let editorAberto = $state(false);
	let editando = $state<Procedimento | null>(null);
	let aba = $state<'texto' | 'historico'>('texto');
	let visaoCelular = $state<'escrever' | 'visualizar'>('escrever');
	let fTitulo = $state('');
	let fCategoria = $state('');
	let fCorpo = $state('');
	let fAtivo = $state(true);
	let salvando = $state(false);
	let textarea = $state<HTMLTextAreaElement | null>(null);

	// Histórico
	let revisoes = $state<Revisao[]>([]);
	let carregandoRevisoes = $state(false);
	let revisaoSel = $state<Revisao | null>(null);
	let visaoRevisao = $state<'texto' | 'comparar'>('texto');
	let restaurando = $state(false);

	const previa = $derived(renderMarkdown(fCorpo));
	const alterado = $derived(
		editando
			? fTitulo !== editando.titulo || fCategoria !== editando.categoria || fCorpo !== editando.corpo || fAtivo !== editando.ativo
			: fTitulo.trim() !== '' || fCorpo !== MODELO
	);
	const diferencas = $derived<Change[]>(
		revisaoSel && editando ? diffLines(revisaoSel.corpo, editando.corpo, { ignoreNewlineAtEof: true }) : []
	);

	let buscaTimer: ReturnType<typeof setTimeout> | undefined;

	async function carregar() {
		loading = true;
		errorMsg = null;
		const params = new URLSearchParams();
		if (busca.trim()) params.set('busca', busca.trim());
		if (filtroCategoria) params.set('categoria', filtroCategoria);
		if (filtroSituacao !== 'todos') params.set('ativo', String(filtroSituacao === 'ativos'));
		try {
			[procedimentos, categorias] = await Promise.all([
				apiFetch<ProcedimentoResumo[]>(`/api/procedimentos?${params}`),
				apiFetch<string[]>('/api/procedimentos/categorias')
			]);
		} catch (err: any) {
			errorMsg = err.message || 'Erro ao carregar procedimentos';
		} finally {
			loading = false;
		}
	}

	function aoBuscar() {
		clearTimeout(buscaTimer);
		buscaTimer = setTimeout(carregar, 250);
	}

	function mostrarSucesso(msg: string) {
		successMsg = msg;
		setTimeout(() => (successMsg = null), 4000);
	}

	function preencher(p: Procedimento | null) {
		editando = p;
		fTitulo = p?.titulo ?? '';
		fCategoria = p?.categoria ?? '';
		fCorpo = p?.corpo ?? MODELO;
		fAtivo = p?.ativo ?? true;
	}

	function abrirNovo() {
		preencher(null);
		aba = 'texto';
		visaoCelular = 'escrever';
		editorAberto = true;
	}

	async function abrirEditar(id: string) {
		try {
			preencher(await apiFetch<Procedimento>(`/api/procedimentos/${id}`));
			aba = 'texto';
			visaoCelular = 'escrever';
			revisoes = [];
			revisaoSel = null;
			editorAberto = true;
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	function fecharEditor() {
		if (alterado && !confirm('Descartar as alterações que não foram salvas?')) return;
		editorAberto = false;
	}

	async function salvar() {
		if (!fTitulo.trim() || !fCorpo.trim()) {
			alert('Preencha o título e o texto do procedimento.');
			return;
		}
		salvando = true;
		try {
			const corpo = JSON.stringify({ titulo: fTitulo, categoria: fCategoria, corpo: fCorpo, ativo: fAtivo });
			const salvo = editando
				? await apiFetch<Procedimento>(`/api/procedimentos/${editando.id}`, { method: 'PUT', body: corpo })
				: await apiFetch<Procedimento>('/api/procedimentos', { method: 'POST', body: corpo });
			preencher(salvo);
			editorAberto = false;
			mostrarSucesso(`"${salvo.titulo}" salvo.`);
			carregar();
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			salvando = false;
		}
	}

	async function abrirHistorico() {
		if (!editando) return;
		aba = 'historico';
		carregandoRevisoes = true;
		try {
			revisoes = await apiFetch<Revisao[]>(`/api/procedimentos/${editando.id}/revisoes`);
			revisaoSel = revisoes[1] ?? revisoes[0] ?? null;
			visaoRevisao = 'texto';
		} catch {
			revisoes = [];
		} finally {
			carregandoRevisoes = false;
		}
	}

	async function restaurar(r: Revisao) {
		if (!editando) return;
		if (alterado && !confirm('Há alterações não salvas no texto. Restaurar mesmo assim e descartá-las?')) return;
		if (!confirm(`Restaurar a versão de ${dataHora(r.criado_em)}? O texto atual continua guardado no histórico.`)) return;
		restaurando = true;
		try {
			preencher(await apiFetch<Procedimento>(`/api/procedimentos/${editando.id}/revisoes/${r.id}/restaurar`, { method: 'POST' }));
			mostrarSucesso('Versão restaurada.');
			carregar();
			await abrirHistorico();
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			restaurando = false;
		}
	}

	// Barra de formatação: envolve a seleção ou insere no início das linhas
	async function envolver(antes: string, depois = antes, exemplo = 'texto') {
		if (!textarea) return;
		const { selectionStart: ini, selectionEnd: fim } = textarea;
		const sel = fCorpo.slice(ini, fim) || exemplo;
		fCorpo = fCorpo.slice(0, ini) + antes + sel + depois + fCorpo.slice(fim);
		await tick();
		textarea.focus();
		textarea.setSelectionRange(ini + antes.length, ini + antes.length + sel.length);
	}

	async function prefixarLinhas(prefixo: (i: number) => string) {
		if (!textarea) return;
		const { selectionStart: ini, selectionEnd: fim } = textarea;
		const inicioLinha = fCorpo.lastIndexOf('\n', ini - 1) + 1;
		const trecho = fCorpo.slice(inicioLinha, fim);
		const novo = trecho
			.split('\n')
			.map((l, i) => prefixo(i) + l)
			.join('\n');
		fCorpo = fCorpo.slice(0, inicioLinha) + novo + fCorpo.slice(fim);
		await tick();
		textarea.focus();
		textarea.setSelectionRange(inicioLinha, inicioLinha + novo.length);
	}

	function aoTeclar(e: KeyboardEvent) {
		if ((e.ctrlKey || e.metaKey) && e.key === 's') {
			e.preventDefault();
			salvar();
		} else if ((e.ctrlKey || e.metaKey) && e.key === 'b') {
			e.preventDefault();
			envolver('**');
		}
	}

	function dataHora(iso: string) {
		return new Date(iso).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' });
	}

	onMount(() => {
		if (auth.user?.papel === 'admin') carregar();
	});
</script>

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape' && editorAberto) fecharEditor();
	}}
/>

{#if auth.user?.papel !== 'admin'}
	<div class="panel empty max-w-lg mx-auto mt-12">
		<ShieldAlert class="size-10 text-danger" strokeWidth={1.5} />
		<h2 class="empty-title">Acesso restrito</h2>
		<p class="empty-text">Só administradores podem editar os procedimentos.</p>
		<a href="/" class="btn btn-secondary mt-5">Voltar ao painel</a>
	</div>
{:else}
	<div class="space-y-6">
		<div class="page-head">
			<div>
				<h1 class="page-title">Procedimentos</h1>
				<p class="page-sub">
					O que o tira-dúvidas da tela de acesso sabe responder. Ele só usa os procedimentos ativos.
				</p>
			</div>
			<button onclick={abrirNovo} class="btn btn-primary">
				<Plus class="size-4" />
				<span>Novo procedimento</span>
			</button>
		</div>

		{#if successMsg}
			<div class="alert bg-ok-soft text-ok" role="status">
				<Check class="size-4 shrink-0" />
				<span>{successMsg}</span>
			</div>
		{/if}

		{#if errorMsg}
			<div class="alert bg-danger-soft text-danger" role="alert">
				<AlertCircle class="size-4 shrink-0" />
				<span>{errorMsg}</span>
			</div>
		{/if}

		<div class="flex flex-col sm:flex-row sm:items-center gap-3">
			<div class="relative flex-1 max-w-md">
				<Search class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-ink-3 pointer-events-none" />
				<input
					type="search"
					bind:value={busca}
					oninput={aoBuscar}
					placeholder="Buscar no título ou no texto…"
					class="field pl-9"
					aria-label="Buscar procedimentos"
				/>
			</div>
			<select bind:value={filtroCategoria} onchange={carregar} class="field sm:w-48" aria-label="Categoria">
				<option value="">Todas as categorias</option>
				{#each categorias as c}
					<option value={c}>{c}</option>
				{/each}
			</select>
			<div class="segmented" role="group" aria-label="Situação">
				{#each [['ativos', 'Ativos'], ['inativos', 'Inativos'], ['todos', 'Todos']] as [valor, rotulo]}
					<button
						type="button"
						aria-pressed={filtroSituacao === valor}
						onclick={() => {
							filtroSituacao = valor as typeof filtroSituacao;
							carregar();
						}}>{rotulo}</button
					>
				{/each}
			</div>
		</div>

		{#if loading}
			<div class="flex justify-center py-16"><div class="spinner"></div></div>
		{:else if procedimentos.length === 0}
			<div class="panel empty">
				<BookOpen class="size-10 text-ink-3" strokeWidth={1.5} />
				{#if busca || filtroCategoria || filtroSituacao !== 'ativos'}
					<h2 class="empty-title">Nada encontrado</h2>
					<p class="empty-text">Nenhum procedimento com esses filtros.</p>
				{:else}
					<h2 class="empty-title">Nenhum procedimento ainda</h2>
					<p class="empty-text">
						Escreva o primeiro: o que fazer quando algo acontece e com quem falar. O tira-dúvidas passa a responder
						com base nele.
					</p>
					<button onclick={abrirNovo} class="btn btn-primary mt-5"><Plus class="size-4" /> Novo procedimento</button>
				{/if}
			</div>
		{:else}
			<div class="panel overflow-hidden">
				<div class="overflow-x-auto">
					<table class="data-table">
						<thead>
							<tr>
								<th>Procedimento</th>
								<th>Categoria</th>
								<th>Última alteração</th>
								<th>Situação</th>
							</tr>
						</thead>
						<tbody>
							{#each procedimentos as p (p.id)}
								<tr class="cursor-pointer {p.ativo ? '' : 'opacity-60'}" onclick={() => abrirEditar(p.id)}>
									<td>
										<button
											type="button"
											class="font-semibold text-ink text-left hover:text-accent cursor-pointer"
											onclick={(e) => {
												e.stopPropagation();
												abrirEditar(p.id);
											}}>{p.titulo}</button
										>
									</td>
									<td class="text-ink-2">{p.categoria || '—'}</td>
									<td class="text-ink-2 whitespace-nowrap">
										<time datetime={p.atualizado_em}>{dataHora(p.atualizado_em)}</time>
										<span class="text-ink-3">· {p.atualizado_por_nome}</span>
									</td>
									<td>
										<span class="inline-flex items-center gap-2 {p.ativo ? 'text-ink-2' : 'text-ink-3'}">
											<span class="size-2 rounded-full {p.ativo ? 'bg-ok' : 'bg-line-strong'}"></span>
											{p.ativo ? 'Ativo' : 'Inativo'}
										</span>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	</div>

	<!-- Editor -->
	{#if editorAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-6xl h-[92vh]" role="dialog" aria-modal="true" aria-labelledby="proc-editor-titulo">
				<div class="modal-head items-center">
					<div class="min-w-0">
						<h3 id="proc-editor-titulo" class="modal-title truncate">
							{editando ? editando.titulo : 'Novo procedimento'}
						</h3>
						{#if editando}
							<p class="mt-0.5 text-sm text-ink-3">
								Alterado em {dataHora(editando.atualizado_em)} por {editando.atualizado_por_nome}
							</p>
						{/if}
					</div>
					<div class="flex items-center gap-3">
						{#if editando}
							<div class="segmented" role="group" aria-label="Seção do editor">
								<button type="button" aria-pressed={aba === 'texto'} onclick={() => (aba = 'texto')}>Texto</button>
								<button type="button" aria-pressed={aba === 'historico'} onclick={abrirHistorico}>Histórico</button>
							</div>
						{/if}
						<button onclick={fecharEditor} class="icon-btn -mr-1.5" aria-label="Fechar">
							<X class="size-5" />
						</button>
					</div>
				</div>

				{#if aba === 'texto'}
					<div class="flex-1 min-h-0 flex flex-col px-5 sm:px-6 py-4 gap-4">
						<div class="grid gap-3 sm:grid-cols-[1fr_14rem_auto] sm:items-end">
							<div>
								<label class="label" for="p-titulo">Título</label>
								<input id="p-titulo" bind:value={fTitulo} maxlength="200" placeholder="Ex.: Servidor fora do ar" class="field" />
							</div>
							<div>
								<label class="label" for="p-categoria">Categoria</label>
								<input
									id="p-categoria"
									bind:value={fCategoria}
									maxlength="80"
									list="p-categorias"
									placeholder="Ex.: Infra"
									class="field"
								/>
								<datalist id="p-categorias">
									{#each categorias as c}
										<option value={c}></option>
									{/each}
								</datalist>
							</div>
							<label class="flex items-center gap-2 h-10 text-sm font-medium text-ink cursor-pointer">
								<input type="checkbox" bind:checked={fAtivo} class="check" />
								Ativo no tira-dúvidas
							</label>
						</div>

						<div class="flex flex-wrap items-center justify-between gap-2">
							<div class="flex items-center gap-0.5" role="toolbar" aria-label="Formatação">
								<button type="button" class="icon-btn" title="Título de seção" aria-label="Título de seção" onclick={() => prefixarLinhas(() => '## ')}>
									<Heading2 class="size-4" />
								</button>
								<button type="button" class="icon-btn" title="Negrito (Ctrl+B)" aria-label="Negrito" onclick={() => envolver('**')}>
									<Bold class="size-4" />
								</button>
								<button type="button" class="icon-btn" title="Lista" aria-label="Lista" onclick={() => prefixarLinhas(() => '- ')}>
									<List class="size-4" />
								</button>
								<button type="button" class="icon-btn" title="Lista numerada" aria-label="Lista numerada" onclick={() => prefixarLinhas((i) => `${i + 1}. `)}>
									<ListOrdered class="size-4" />
								</button>
								<button type="button" class="icon-btn" title="Link" aria-label="Link" onclick={() => envolver('[', '](https://)', 'texto do link')}>
									<LinkIcon class="size-4" />
								</button>
							</div>
							<div class="segmented lg:hidden" role="group" aria-label="Visão">
								<button type="button" aria-pressed={visaoCelular === 'escrever'} onclick={() => (visaoCelular = 'escrever')}>Escrever</button>
								<button type="button" aria-pressed={visaoCelular === 'visualizar'} onclick={() => (visaoCelular = 'visualizar')}>Visualizar</button>
							</div>
						</div>

						<div class="flex-1 min-h-0 grid gap-4 lg:grid-cols-2">
							<textarea
								bind:this={textarea}
								bind:value={fCorpo}
								onkeydown={aoTeclar}
								maxlength="20000"
								spellcheck="true"
								aria-label="Texto do procedimento em Markdown"
								class="field h-full! min-h-64 resize-none font-mono text-[13px] {visaoCelular === 'visualizar' ? 'hidden lg:block' : ''}"
							></textarea>
							<div
								class="h-full min-h-64 overflow-y-auto rounded-lg border border-line bg-sunken px-4 py-3 {visaoCelular === 'escrever' ? 'hidden lg:block' : ''}"
								aria-label="Pré-visualização"
							>
								{#if fCorpo.trim()}
									<div class="md">{@html previa}</div>
								{:else}
									<p class="text-sm text-ink-3">A pré-visualização aparece aqui.</p>
								{/if}
							</div>
						</div>
						<p class="hint -mt-2">
							A seção <strong>## Contato</strong> aparece como cartão no tira-dúvidas, exatamente como escrita. Descreva em
							"Como as pessoas descrevem" as palavras que usam: é por elas que a busca encontra o procedimento.
						</p>
					</div>

					<div class="modal-foot">
						<button onclick={fecharEditor} class="btn btn-ghost">Cancelar</button>
						<button onclick={salvar} disabled={salvando || (!!editando && !alterado)} class="btn btn-primary">
							{salvando ? 'Salvando…' : editando ? 'Salvar alterações' : 'Criar procedimento'}
						</button>
					</div>
				{:else}
					<div class="flex-1 min-h-0 grid md:grid-cols-[18rem_1fr]">
						<div class="border-b md:border-b-0 md:border-r border-line overflow-y-auto max-h-48 md:max-h-none">
							{#if carregandoRevisoes}
								<div class="flex justify-center py-10"><div class="spinner"></div></div>
							{:else}
								<ul class="py-2">
									{#each revisoes as r, i (r.id)}
										<li>
											<button
												type="button"
												onclick={() => {
													revisaoSel = r;
													visaoRevisao = 'texto';
												}}
												aria-current={revisaoSel?.id === r.id}
												class="w-full text-left px-4 py-2.5 cursor-pointer transition-colors {revisaoSel?.id === r.id
													? 'bg-accent-soft'
													: 'hover:bg-sunken'}"
											>
												<span class="flex items-center gap-2 text-sm font-semibold text-ink">
													<time datetime={r.criado_em}>{dataHora(r.criado_em)}</time>
													{#if i === 0}<span class="tag tag-accent">Atual</span>{/if}
												</span>
												<span class="block text-[13px] text-ink-3">{r.nota} · {r.editado_por_nome}</span>
											</button>
										</li>
									{/each}
								</ul>
							{/if}
						</div>

						<div class="min-h-0 flex flex-col">
							{#if revisaoSel}
								{@const atual = revisoes[0]?.id === revisaoSel.id}
								<div class="flex flex-wrap items-center justify-between gap-3 px-5 sm:px-6 py-3 border-b border-line">
									<div class="segmented" role="group" aria-label="Visão da versão">
										<button type="button" aria-pressed={visaoRevisao === 'texto'} onclick={() => (visaoRevisao = 'texto')}>Texto</button>
										<button
											type="button"
											aria-pressed={visaoRevisao === 'comparar'}
											disabled={atual}
											onclick={() => (visaoRevisao = 'comparar')}>Comparar com o atual</button
										>
									</div>
									{#if !atual}
										<button onclick={() => restaurar(revisaoSel!)} disabled={restaurando} class="btn btn-secondary btn-sm">
											<RotateCcw class="size-3.5" />
											{restaurando ? 'Restaurando…' : 'Restaurar esta versão'}
										</button>
									{/if}
								</div>
								<div class="flex-1 overflow-y-auto px-5 sm:px-6 py-4">
									{#if visaoRevisao === 'texto'}
										<h4 class="text-base font-bold text-ink">{revisaoSel.titulo}</h4>
										<p class="mt-0.5 mb-4 text-[13px] text-ink-3">
											{revisaoSel.categoria || 'Sem categoria'} · {revisaoSel.ativo ? 'Ativo' : 'Inativo'}
										</p>
										<div class="md">{@html renderMarkdown(revisaoSel.corpo)}</div>
									{:else}
										{#if revisaoSel.titulo !== editando?.titulo}
											<p class="mb-3 text-sm text-ink-2">
												Título: <del class="text-danger">{revisaoSel.titulo}</del> → <ins class="text-ok no-underline">{editando?.titulo}</ins>
											</p>
										{/if}
										<pre class="text-[13px] leading-relaxed font-mono whitespace-pre-wrap break-words">{#each diferencas as d}<span
													class={d.added ? 'block bg-ok-soft text-ok' : d.removed ? 'block bg-danger-soft text-danger line-through decoration-danger/40' : 'text-ink-2'}
													>{d.value}</span
												>{/each}</pre>
										<p class="hint">
											<span class="text-danger">Vermelho</span>: só nesta versão ·
											<span class="text-ok">verde</span>: só no texto atual.
										</p>
									{/if}
								</div>
							{:else if !carregandoRevisoes}
								<p class="p-6 text-sm text-ink-3">Escolha uma versão à esquerda.</p>
							{/if}
						</div>
					</div>
				{/if}
			</div>
		</div>
	{/if}
{/if}
