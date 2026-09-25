<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { toast } from '$lib/toast.svelte';
	import { X, Plus, Trash2, Copy, Tv } from 'lucide-svelte';

	interface TelaTV {
		id: string;
		nome: string;
		criador_nome: string;
		criado_em: string;
		ultimo_acesso_em: string | null;
		ultimo_ip: string;
		chave?: string;
	}

	let { onfechar }: { onfechar: () => void } = $props();

	const FUSO = 'America/Sao_Paulo';

	let telas = $state<TelaTV[]>([]);
	let loading = $state(true);
	let nome = $state('');
	let salvando = $state(false);
	// Link da tela recém-criada: a chave só aparece uma vez
	let linkNovo = $state<{ nome: string; url: string } | null>(null);

	async function carregar() {
		try {
			telas = await apiFetch<TelaTV[]>('/api/tv/telas');
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			loading = false;
		}
	}

	onMount(carregar);

	async function criar() {
		if (!nome.trim()) return;
		salvando = true;
		try {
			const t = await apiFetch<TelaTV>('/api/tv/telas', { method: 'POST', body: JSON.stringify({ nome: nome.trim() }) });
			linkNovo = { nome: t.nome, url: `${location.origin}/tv#chave=${t.chave}` };
			nome = '';
			await carregar();
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			salvando = false;
		}
	}

	async function excluir(t: TelaTV) {
		if (!confirm(`Revogar a tela "${t.nome}"? Ela deixa de mostrar o painel na hora.`)) return;
		try {
			await apiFetch(`/api/tv/telas/${t.id}`, { method: 'DELETE' });
			await carregar();
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	async function copiar(campo: HTMLInputElement) {
		campo.select();
		try {
			await navigator.clipboard.writeText(campo.value);
			toast.success('Link copiado');
		} catch {
			// Sem HTTPS o navegador bloqueia a área de transferência; o texto fica selecionado
			toast.error('Não deu para copiar sozinho: use Ctrl+C no link selecionado');
		}
	}

	function ultimoAcesso(t: TelaTV): string {
		if (!t.ultimo_acesso_em) return 'Nunca abriu';
		const quando = new Date(t.ultimo_acesso_em).toLocaleString('pt-BR', { timeZone: FUSO, dateStyle: 'short', timeStyle: 'short' });
		return `Último acesso ${quando}${t.ultimo_ip ? ` · ${t.ultimo_ip}` : ''}`;
	}

	let campoLink = $state<HTMLInputElement>();
</script>

<div class="modal-backdrop">
	<div class="modal max-w-xl" role="dialog" aria-modal="true" aria-labelledby="tv-titulo">
		<div class="modal-head">
			<div>
				<h3 id="tv-titulo" class="modal-title">Telas de TV</h3>
				<p class="text-sm text-ink-3 mt-0.5">
					Cada tela recebe um link próprio que abre o modo TV sem login, só para leitura. Revogar a tela desliga o link.
				</p>
			</div>
			<button onclick={onfechar} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
				<X class="size-5" />
			</button>
		</div>

		<div class="modal-body">
			{#if linkNovo}
				<div class="rounded-xl border border-accent bg-accent-soft p-4 space-y-2">
					<p class="text-sm font-semibold text-ink">Link da tela "{linkNovo.nome}"</p>
					<p class="text-xs text-ink-2">
						Abra este link no navegador da TV. Ele aparece só agora: se perder, revogue a tela e cadastre outra.
					</p>
					<div class="flex gap-2">
						<input
							bind:this={campoLink}
							type="text"
							readonly
							value={linkNovo.url}
							class="field field-sm font-mono text-xs"
							onfocus={(e) => e.currentTarget.select()}
						/>
						<button onclick={() => campoLink && copiar(campoLink)} class="btn btn-primary btn-sm shrink-0">
							<Copy class="size-4" /> Copiar
						</button>
					</div>
				</div>
			{/if}

			<form
				class="flex gap-2"
				onsubmit={(e) => {
					e.preventDefault();
					criar();
				}}
			>
				<input type="text" bind:value={nome} maxlength="80" placeholder="Ex.: Telão da sala do NOC" class="field" aria-label="Nome da tela" />
				<button type="submit" class="btn btn-secondary shrink-0" disabled={salvando || !nome.trim()}>
					<Plus class="size-4" /> Cadastrar
				</button>
			</form>

			{#if loading}
				<div class="flex justify-center py-6"><div class="spinner"></div></div>
			{:else if telas.length === 0}
				<p class="text-sm text-ink-3 text-center py-4">Nenhuma tela cadastrada.</p>
			{:else}
				<ul class="divide-y divide-line border border-line rounded-xl">
					{#each telas as t (t.id)}
						<li class="flex items-center gap-3 px-4 py-3">
							<Tv class="size-5 text-ink-3 shrink-0" />
							<div class="min-w-0 flex-1">
								<div class="text-sm font-semibold text-ink truncate">{t.nome}</div>
								<div class="text-xs text-ink-3 truncate">{ultimoAcesso(t)} · cadastrada por {t.criador_nome}</div>
							</div>
							<button onclick={() => excluir(t)} class="icon-btn icon-btn-danger" title="Revogar tela" aria-label="Revogar {t.nome}">
								<Trash2 class="size-4" />
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<div class="modal-foot">
			<a href="/tv" target="_blank" rel="noopener" class="btn btn-ghost mr-auto">Ver o modo TV</a>
			<button onclick={onfechar} class="btn btn-secondary">Fechar</button>
		</div>
	</div>
</div>
