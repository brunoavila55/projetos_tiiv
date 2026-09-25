<script lang="ts">
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { toast } from '$lib/toast.svelte';
	import { ticketsStore } from '$lib/tickets.svelte';
	import { Inbox, Hand, Ban, Clock, User, ArrowRight, History } from 'lucide-svelte';

	interface Ticket {
		id: string;
		numero: number;
		solicitante_nome: string;
		titulo: string;
		descricao: string;
		prioridade: 'baixa' | 'media' | 'alta';
		status: 'aberto' | 'resgatado' | 'descartado';
		tarefa_id: string | null;
		tarefa_status: 'pendente' | 'em_andamento' | 'concluida' | null;
		tratado_por_nome: string | null;
		tratado_em: string | null;
		criado_em: string;
	}

	const FUSO = 'America/Sao_Paulo';
	const prioridadeRotulo = { baixa: 'Pode esperar', media: 'Normal', alta: 'Urgente' } as const;
	const tarefaRotulo = { pendente: 'Tarefa pendente', em_andamento: 'Tarefa em andamento', concluida: 'Tarefa concluída' } as const;

	let aba = $state<'aberto' | 'tratados'>('aberto');
	let tickets = $state<Ticket[]>([]);
	let loading = $state(true);
	let processando = $state<string | null>(null);

	const visiveis = $derived(tickets.filter((t) => (aba === 'aberto' ? t.status === 'aberto' : t.status !== 'aberto')));

	function dataHora(iso: string): string {
		return new Date(iso).toLocaleString('pt-BR', { timeZone: FUSO, dateStyle: 'short', timeStyle: 'short' });
	}

	function haQuanto(iso: string): string {
		const min = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60000));
		if (min < 1) return 'agora';
		if (min < 60) return `há ${min} min`;
		const h = Math.floor(min / 60);
		if (h < 24) return `há ${h} h`;
		const d = Math.floor(h / 24);
		return `há ${d} ${d === 1 ? 'dia' : 'dias'}`;
	}

	async function carregar() {
		loading = true;
		try {
			tickets = await apiFetch<Ticket[]>('/api/tickets');
			ticketsStore.abertos = tickets.filter((t) => t.status === 'aberto').length;
		} catch (err) {
			console.error('Erro ao listar tickets:', err);
		} finally {
			loading = false;
		}
	}

	async function resgatar(t: Ticket) {
		processando = t.id;
		try {
			await apiFetch(`/api/tickets/${t.id}/resgatar`, { method: 'POST' });
			toast.success(`Ticket #${t.numero} foi para as suas tarefas`);
			await carregar();
		} catch {
			// 409 (outro operador resgatou antes) já aparece como toast; recarrega a fila
			await carregar();
		} finally {
			processando = null;
		}
	}

	async function descartar(t: Ticket) {
		if (!confirm(`Descartar o ticket #${t.numero} "${t.titulo}"?`)) return;
		processando = t.id;
		try {
			await apiFetch(`/api/tickets/${t.id}/descartar`, { method: 'POST' });
			await carregar();
		} catch {
			await carregar();
		} finally {
			processando = null;
		}
	}

	$effect(() => {
		carregar();
	});
</script>

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Tickets</h1>
			<p class="page-sub">Pedidos abertos pela tela de acesso. Ao resgatar, o ticket vira uma tarefa sua.</p>
		</div>
	</div>

	<div class="segmented" role="group" aria-label="Seção">
		<button aria-pressed={aba === 'aberto'} onclick={() => (aba = 'aberto')}>
			<Inbox class="size-4" />
			<span>Abertos</span>
			{#if ticketsStore.abertos > 0}
				<span class="ml-0.5 min-w-5 h-5 px-1.5 rounded-full bg-accent text-on-accent text-xs font-bold grid place-items-center tabular">{ticketsStore.abertos}</span>
			{/if}
		</button>
		<button aria-pressed={aba === 'tratados'} onclick={() => (aba = 'tratados')}>
			<History class="size-4" />
			<span>Tratados</span>
		</button>
	</div>

	{#if loading && tickets.length === 0}
		<div class="flex justify-center py-20"><div class="spinner"></div></div>
	{:else if visiveis.length === 0}
		<div class="panel empty">
			<Inbox class="size-9 text-ink-3" strokeWidth={1.5} />
			<h3 class="empty-title">{aba === 'aberto' ? 'Nenhum ticket na fila' : 'Nenhum ticket tratado ainda'}</h3>
			<p class="empty-text">
				{aba === 'aberto'
					? 'Quem não tem acesso pode abrir um ticket pelo botão "Reportar um problema" na tela de login.'
					: 'Tickets resgatados e descartados aparecem aqui.'}
			</p>
		</div>
	{:else}
		<ul class="space-y-3">
			{#each visiveis as t (t.id)}
				<li class="panel p-4 sm:p-5 {t.status === 'descartado' ? 'opacity-60' : ''}">
					<div class="flex flex-col sm:flex-row sm:items-start gap-4">
						<div class="flex-1 min-w-0">
							<div class="flex flex-wrap items-center gap-2">
								<span class="text-sm font-semibold text-ink-3 tabular">#{t.numero}</span>
								{#if t.prioridade === 'alta'}
									<span class="tag tag-danger">{prioridadeRotulo.alta}</span>
								{:else if t.prioridade === 'baixa'}
									<span class="tag">{prioridadeRotulo.baixa}</span>
								{/if}
								{#if t.status === 'descartado'}
									<span class="tag">Descartado</span>
								{/if}
							</div>
							<h2 class="mt-1 text-base font-bold text-ink leading-snug">{t.titulo}</h2>
							<p class="mt-1.5 text-sm text-ink-2 whitespace-pre-line break-words">{t.descricao}</p>
							<div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-[13px] text-ink-3">
								<span class="inline-flex items-center gap-1.5"><User class="size-3.5" /> {t.solicitante_nome}</span>
								<span class="inline-flex items-center gap-1.5" title={dataHora(t.criado_em)}>
									<Clock class="size-3.5" /> {haQuanto(t.criado_em)}
								</span>
							</div>
						</div>

						<div class="flex sm:flex-col items-stretch sm:items-end gap-2 shrink-0">
							{#if t.status === 'aberto'}
								<button onclick={() => resgatar(t)} disabled={processando === t.id} class="btn btn-primary flex-1 sm:flex-none">
									<Hand class="size-4" />
									<span>Resgatar</span>
								</button>
								{#if auth.ehAdmin}
									<button onclick={() => descartar(t)} disabled={processando === t.id} class="btn btn-ghost" title="Descartar ticket (spam ou duplicado)">
										<Ban class="size-4" />
										<span>Descartar</span>
									</button>
								{/if}
							{:else}
								<div class="text-[13px] text-ink-3 sm:text-right">
									{t.status === 'resgatado' ? 'Resgatado' : 'Descartado'} por
									<strong class="text-ink-2">{t.tratado_por_nome ?? '—'}</strong>
									{#if t.tratado_em}<br />{dataHora(t.tratado_em)}{/if}
								</div>
								{#if t.status === 'resgatado' && t.tarefa_status}
									<a href="/tarefas" class="btn btn-sm btn-soft">
										{tarefaRotulo[t.tarefa_status]}
										<ArrowRight class="size-3.5" />
									</a>
								{/if}
							{/if}
						</div>
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>
