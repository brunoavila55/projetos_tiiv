<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { X, Plus, Trash2, Pencil, Building2 } from 'lucide-svelte';

	interface SetorItem {
		id: string;
		nome: string;
		aceita_pedidos: boolean;
		usuarios_ativos: number;
	}

	let { onfechar, onalterado }: { onfechar: () => void; onalterado: () => void } = $props();

	let setores = $state<SetorItem[]>([]);
	let loading = $state(true);
	let nome = $state('');
	let salvando = $state(false);

	async function carregar() {
		try {
			setores = await apiFetch<SetorItem[]>('/api/setores');
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			loading = false;
		}
	}

	onMount(carregar);

	async function recarregar() {
		await carregar();
		onalterado();
	}

	async function criar() {
		if (!nome.trim()) return;
		salvando = true;
		try {
			await apiFetch('/api/setores', { method: 'POST', body: JSON.stringify({ nome: nome.trim() }) });
			nome = '';
			await recarregar();
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			salvando = false;
		}
	}

	async function salvar(s: SetorItem, mudancas: Partial<Pick<SetorItem, 'nome' | 'aceita_pedidos'>>) {
		try {
			await apiFetch(`/api/setores/${s.id}`, {
				method: 'PUT',
				body: JSON.stringify({ nome: s.nome, aceita_pedidos: s.aceita_pedidos, ...mudancas })
			});
			await recarregar();
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	function renomear(s: SetorItem) {
		const novo = prompt('Novo nome do setor', s.nome)?.trim();
		if (novo && novo !== s.nome) salvar(s, { nome: novo });
	}

	async function excluir(s: SetorItem) {
		if (!confirm(`Excluir o setor "${s.nome}"? Só dá para excluir setor sem pessoas nem registros.`)) return;
		try {
			await apiFetch(`/api/setores/${s.id}`, { method: 'DELETE' });
			await recarregar();
		} catch {
			// apiFetch já mostrou o erro
		}
	}
</script>

<div class="modal-backdrop">
	<div class="modal max-w-xl" role="dialog" aria-modal="true" aria-labelledby="setores-titulo">
		<div class="modal-head">
			<div>
				<h3 id="setores-titulo" class="modal-title">Setores</h3>
				<p class="text-sm text-ink-3 mt-0.5">
					Cada setor tem seus próprios tickets, tarefas, calendário, plantão, procedimentos, avisos e links. Monitor, estoque e técnicos são de todos.
				</p>
			</div>
			<button onclick={onfechar} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
				<X class="size-5" />
			</button>
		</div>

		<div class="modal-body">
			<form
				class="flex gap-2"
				onsubmit={(e) => {
					e.preventDefault();
					criar();
				}}
			>
				<input type="text" bind:value={nome} maxlength="60" placeholder="Ex.: Agendamento" class="field" aria-label="Nome do setor" />
				<button type="submit" class="btn btn-secondary shrink-0" disabled={salvando || !nome.trim()}>
					<Plus class="size-4" /> Criar setor
				</button>
			</form>

			{#if loading}
				<div class="flex justify-center py-6"><div class="spinner"></div></div>
			{:else}
				<ul class="divide-y divide-line border border-line rounded-xl">
					{#each setores as s (s.id)}
						<li class="flex items-center gap-3 px-4 py-3">
							<Building2 class="size-5 text-ink-3 shrink-0" />
							<div class="min-w-0 flex-1">
								<div class="text-sm font-semibold text-ink truncate">{s.nome}</div>
								<label class="mt-0.5 inline-flex items-center gap-1.5 text-xs text-ink-3 cursor-pointer">
									<input
										type="checkbox"
										class="check"
										checked={s.aceita_pedidos}
										onchange={(e) => salvar(s, { aceita_pedidos: e.currentTarget.checked })}
									/>
									Recebe tickets e perguntas pela tela de acesso
								</label>
							</div>
							<span class="text-xs text-ink-3 tabular whitespace-nowrap">
								{s.usuarios_ativos} {s.usuarios_ativos === 1 ? 'pessoa' : 'pessoas'}
							</span>
							<button onclick={() => renomear(s)} class="icon-btn" title="Renomear" aria-label="Renomear {s.nome}">
								<Pencil class="size-4" />
							</button>
							<button onclick={() => excluir(s)} class="icon-btn icon-btn-danger" title="Excluir setor" aria-label="Excluir {s.nome}">
								<Trash2 class="size-4" />
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<div class="modal-foot">
			<button onclick={onfechar} class="btn btn-secondary">Fechar</button>
		</div>
	</div>
</div>
