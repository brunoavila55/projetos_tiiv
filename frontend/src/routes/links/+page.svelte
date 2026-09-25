<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { Link as LinkIcon, Plus, Pencil, Trash2, Search, X, ExternalLink } from 'lucide-svelte';

	interface LinkUtil {
		id: string;
		titulo: string;
		url: string;
		descricao: string;
		categoria: string;
		criador_nome: string;
		pode_editar: boolean;
	}

	const SEM_CATEGORIA = 'Geral';

	let links = $state<LinkUtil[]>([]);
	let loading = $state(true);
	let busca = $state('');

	// Busca sem acento e sem diferenciar maiúsculas
	function normalizar(s: string): string {
		return s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
	}

	function host(url: string): string {
		try {
			return new URL(url).host;
		} catch {
			return url;
		}
	}

	const categorias = $derived([...new Set(links.map((l) => l.categoria).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'pt-BR')));

	const grupos = $derived.by(() => {
		const termo = normalizar(busca.trim());
		const filtrados = termo
			? links.filter((l) => normalizar(`${l.titulo} ${l.descricao} ${l.categoria} ${l.url}`).includes(termo))
			: links;
		const mapa = new Map<string, LinkUtil[]>();
		for (const l of filtrados) {
			const cat = l.categoria || SEM_CATEGORIA;
			mapa.set(cat, [...(mapa.get(cat) ?? []), l]);
		}
		// "Geral" (sem categoria) primeiro, depois em ordem alfabética
		return [...mapa.entries()].sort(([a], [b]) =>
			a === SEM_CATEGORIA ? -1 : b === SEM_CATEGORIA ? 1 : a.localeCompare(b, 'pt-BR')
		);
	});

	async function carregar() {
		try {
			links = await apiFetch<LinkUtil[]>('/api/links');
		} catch (err) {
			console.error('Erro ao listar links:', err);
		} finally {
			loading = false;
		}
	}

	onMount(carregar);

	// ---------- Cadastro ----------
	let modalAberto = $state(false);
	let salvando = $state(false);
	let editId = $state<string | null>(null);
	let fTitulo = $state('');
	let fUrl = $state('');
	let fDescricao = $state('');
	let fCategoria = $state('');

	function abrirNovo() {
		editId = null;
		fTitulo = '';
		fUrl = '';
		fDescricao = '';
		fCategoria = '';
		modalAberto = true;
	}

	function abrirEditar(l: LinkUtil) {
		editId = l.id;
		fTitulo = l.titulo;
		fUrl = l.url;
		fDescricao = l.descricao;
		fCategoria = l.categoria;
		modalAberto = true;
	}

	async function salvar() {
		if (!fTitulo.trim() || !fUrl.trim()) {
			alert('Título e endereço são obrigatórios.');
			return;
		}
		let url = fUrl.trim();
		// "intranet/" vira "http://intranet/": sistemas internos costumam ser http
		if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(url)) url = `http://${url}`;

		salvando = true;
		try {
			await apiFetch(editId ? `/api/links/${editId}` : '/api/links', {
				method: editId ? 'PUT' : 'POST',
				body: JSON.stringify({
					titulo: fTitulo.trim(),
					url,
					descricao: fDescricao.trim(),
					categoria: fCategoria.trim()
				})
			});
			modalAberto = false;
			await carregar();
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			salvando = false;
		}
	}

	async function excluir(l: LinkUtil) {
		if (!confirm(`Excluir o link "${l.titulo}"?`)) return;
		try {
			await apiFetch(`/api/links/${l.id}`, { method: 'DELETE' });
			await carregar();
		} catch {
			// apiFetch já mostrou o erro
		}
	}
</script>

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Links úteis</h1>
			<p class="page-sub">Sistemas, painéis e documentação da equipe num lugar só.</p>
		</div>
		<button onclick={abrirNovo} class="btn btn-primary">
			<Plus class="size-4" />
			<span>Novo link</span>
		</button>
	</div>

	{#if loading}
		<div class="flex justify-center py-20"><div class="spinner"></div></div>
	{:else if links.length === 0}
		<div class="panel empty">
			<LinkIcon class="size-9 text-ink-3" strokeWidth={1.5} />
			<h3 class="empty-title">Nenhum link cadastrado</h3>
			<p class="empty-text">Adicione os endereços que a equipe usa todo dia: monitoramento, roteadores, documentação.</p>
		</div>
	{:else}
		<div class="relative max-w-md">
			<Search class="size-4 text-ink-3 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
			<input type="search" bind:value={busca} placeholder="Buscar por nome, categoria ou endereço" class="field pl-9" />
		</div>

		{#if grupos.length === 0}
			<p class="py-10 text-sm text-ink-3 text-center">Nenhum link encontrado para "{busca}".</p>
		{/if}

		{#each grupos as [categoria, itens] (categoria)}
			<section>
				<h2 class="mb-2.5 text-[13px] font-semibold text-ink-3">
					{categoria} <span class="tabular">{itens.length}</span>
				</h2>
				<ul class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-3">
					{#each itens as l (l.id)}
						<li class="group panel relative flex items-start gap-3 p-4 hover:border-line-strong transition-colors">
							<div class="grid place-items-center size-9 shrink-0 rounded-lg bg-accent-soft text-accent font-bold uppercase" aria-hidden="true">
								{l.titulo.trim().charAt(0)}
							</div>
							<div class="min-w-0 flex-1">
								<a
									href={l.url}
									target="_blank"
									rel="noopener noreferrer"
									class="font-semibold text-ink leading-snug hover:text-accent after:absolute after:inset-0 after:rounded-xl"
								>
									{l.titulo}
								</a>
								<div class="flex items-center gap-1 text-[13px] text-ink-3 min-w-0">
									<ExternalLink class="size-3 shrink-0" aria-hidden="true" />
									<span class="truncate">{host(l.url)}</span>
								</div>
								{#if l.descricao}
									<p class="text-[13px] text-ink-2 mt-1 line-clamp-2">{l.descricao}</p>
								{/if}
							</div>
							{#if l.pode_editar}
								<div class="relative z-10 flex shrink-0 gap-0.5 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-within:opacity-100 transition-opacity">
									<button onclick={() => abrirEditar(l)} class="icon-btn size-7" title="Editar" aria-label="Editar {l.titulo}">
										<Pencil class="size-3.5" />
									</button>
									<button onclick={() => excluir(l)} class="icon-btn icon-btn-danger size-7" title="Excluir" aria-label="Excluir {l.titulo}">
										<Trash2 class="size-3.5" />
									</button>
								</div>
							{/if}
						</li>
					{/each}
				</ul>
			</section>
		{/each}
	{/if}

	{#if modalAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{editId ? 'Editar link' : 'Novo link'}</h3>
					<button onclick={() => (modalAberto = false)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div>
						<label class="label" for="ln-titulo">Título</label>
						<input id="ln-titulo" type="text" bind:value={fTitulo} maxlength="120" placeholder="Ex.: Zabbix" class="field" autofocus />
					</div>
					<div>
						<label class="label" for="ln-url">Endereço</label>
						<input id="ln-url" type="text" inputmode="url" bind:value={fUrl} maxlength="2000" placeholder="http://10.0.0.5/zabbix" class="field font-mono" autocapitalize="off" spellcheck="false" />
					</div>
					<div>
						<label class="label" for="ln-cat">Categoria <span class="font-normal text-ink-3">(opcional)</span></label>
						<input id="ln-cat" type="text" bind:value={fCategoria} maxlength="60" list="ln-categorias" placeholder="Ex.: Monitoramento" class="field" />
						<datalist id="ln-categorias">
							{#each categorias as c}<option value={c}></option>{/each}
						</datalist>
					</div>
					<div>
						<label class="label" for="ln-desc">Descrição <span class="font-normal text-ink-3">(opcional)</span></label>
						<input id="ln-desc" type="text" bind:value={fDescricao} maxlength="300" class="field" />
					</div>
				</div>

				<div class="modal-foot">
					<button onclick={() => (modalAberto = false)} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvar} class="btn btn-primary" disabled={salvando}>
						{editId ? 'Salvar alterações' : 'Adicionar link'}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
