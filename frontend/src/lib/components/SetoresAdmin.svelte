<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { MODULOS, type Modulo } from '$lib/modulos';
	import { X, Plus, Trash2, Pencil, Building2, ChevronDown } from 'lucide-svelte';

	interface SetorItem {
		id: string;
		nome: string;
		aceita_pedidos: boolean;
		modulos_desativados: Modulo[];
		usuarios_ativos: number;
	}

	let { onfechar, onalterado }: { onfechar: () => void; onalterado: () => void } = $props();

	let setores = $state<SetorItem[]>([]);
	let loading = $state(true);
	let nome = $state('');
	let salvando = $state(false);
	// Setor com a lista de módulos aberta
	let expandido = $state<string | null>(null);

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

	async function salvar(s: SetorItem, mudancas: Partial<Pick<SetorItem, 'nome' | 'aceita_pedidos' | 'modulos_desativados'>>) {
		try {
			await apiFetch(`/api/setores/${s.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					nome: s.nome,
					aceita_pedidos: s.aceita_pedidos,
					modulos_desativados: s.modulos_desativados,
					...mudancas
				})
			});
			await recarregar();
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	// Ligar um módulo liga também o que ele precisa; desligar desliga quem depende dele
	function alternarModulo(s: SetorItem, m: Modulo, ligar: boolean) {
		const desligados = new Set(s.modulos_desativados);
		if (ligar) {
			desligados.delete(m);
			for (const d of MODULOS.find((x) => x.id === m)?.depende ?? []) desligados.delete(d);
		} else {
			desligados.add(m);
			for (const x of MODULOS) if (x.depende?.includes(m)) desligados.add(x.id);
		}
		salvar(s, { modulos_desativados: MODULOS.map((x) => x.id).filter((id) => desligados.has(id)) });
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

	const nomeModulo = (id: Modulo) => MODULOS.find((m) => m.id === id)?.nome ?? id;
</script>

<div class="modal-backdrop">
	<div class="modal max-w-xl" role="dialog" aria-modal="true" aria-labelledby="setores-titulo">
		<div class="modal-head">
			<div>
				<h3 id="setores-titulo" class="modal-title">Setores</h3>
				<p class="text-sm text-ink-3 mt-0.5">
					Cada setor tem seus próprios tickets, tarefas, calendário, plantão, técnicos, procedimentos, avisos e links. Monitor e estoque são de todos. Em Módulos você escolhe o que cada setor usa.
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
						{@const ligados = MODULOS.length - s.modulos_desativados.length}
						<li class="px-4 py-3">
							<div class="flex items-center gap-3">
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
							</div>

							<button
								onclick={() => (expandido = expandido === s.id ? null : s.id)}
								class="mt-2 ml-8 inline-flex items-center gap-1 text-xs font-semibold text-ink-2 hover:text-accent cursor-pointer"
								aria-expanded={expandido === s.id}
							>
								Módulos: {ligados === MODULOS.length ? 'todos ligados' : `${ligados} de ${MODULOS.length} ligados`}
								<ChevronDown class="size-3.5 transition-transform {expandido === s.id ? 'rotate-180' : ''}" />
							</button>

							{#if expandido === s.id}
								<div class="mt-2 ml-8 grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2">
									{#each MODULOS as m (m.id)}
										{@const ligado = !s.modulos_desativados.includes(m.id)}
										<label class="flex items-start gap-2 cursor-pointer">
											<input
												type="checkbox"
												class="check mt-0.5"
												checked={ligado}
												onchange={(e) => alternarModulo(s, m.id, e.currentTarget.checked)}
											/>
											<span class="min-w-0">
												<span class="block text-sm font-medium {ligado ? 'text-ink' : 'text-ink-3'}">{m.nome}</span>
												<span class="block text-xs text-ink-3">
													{m.descricao}{#if m.depende}. Precisa de {m.depende.map(nomeModulo).join(', ')}{/if}
												</span>
											</span>
										</label>
									{/each}
								</div>
							{/if}
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
