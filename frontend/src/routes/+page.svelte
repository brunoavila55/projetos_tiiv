<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { Calendar, CheckSquare, Package, Headphones, Plus, Flag } from 'lucide-svelte';

	interface PainelDados {
		proximos_eventos: {
			id: string;
			titulo: string;
			inicio: string;
			fim: string;
			dia_inteiro: boolean;
			criador_nome: string;
			criador_cor: string;
		}[];
		tarefas_pendentes: {
			id: string;
			titulo: string;
			prioridade: string;
			prazo: string | null;
			atrasada: boolean;
		}[];
		itens_abaixo_do_minimo: {
			id: string;
			nome: string;
			unidade: string;
			categoria: string;
			estoque_minimo: number;
			saldo: number;
		}[];
	}

	let dados = $state<PainelDados | null>(null);
	let loading = $state(true);

	async function carregarPainel() {
		loading = true;
		try {
			// Uma única chamada GET /api/painel conforme requisito do P1.6
			dados = await apiFetch<PainelDados>('/api/painel');
		} catch (err) {
			console.error('Erro ao carregar dados do painel:', err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		carregarPainel();
	});

	const hoje = new Date();

	function saudacao(): string {
		const h = hoje.getHours();
		if (h < 12) return 'Bom dia';
		if (h < 18) return 'Boa tarde';
		return 'Boa noite';
	}

	function primeiroNome(nome?: string): string {
		return nome?.trim().split(/\s+/)[0] ?? '';
	}

	function rotuloDia(iso: string): string {
		const d = new Date(iso);
		return d.toDateString() === hoje.toDateString() ? 'Hoje' : 'Amanhã';
	}

	function hora(iso: string): string {
		return new Date(iso).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });
	}
</script>

{#snippet cabecalho(titulo: string, total: number | null, href: string, linkTexto: string)}
	<div class="flex items-baseline justify-between gap-3 px-5 pt-4 pb-3 border-b border-line">
		<h2 class="text-[15px] font-bold text-ink">
			{titulo}
			{#if total}
				<span class="ml-1 font-semibold text-ink-3 tabular">{total}</span>
			{/if}
		</h2>
		<a href={href} class="text-[13px] font-semibold text-accent hover:underline underline-offset-2">{linkTexto}</a>
	</div>
{/snippet}

{#snippet vazio(texto: string)}
	<p class="px-5 py-10 text-sm text-ink-3 text-center">{texto}</p>
{/snippet}

<div class="space-y-8">
	<div class="page-head">
		<div>
			<p class="text-sm text-ink-3 first-letter:uppercase">
				{hoje.toLocaleDateString('pt-BR', { weekday: 'long', day: 'numeric', month: 'long' })}
			</p>
			<h1 class="page-title mt-0.5">{saudacao()}, {primeiroNome(auth.user?.nome)}</h1>
		</div>
		<div class="flex flex-wrap gap-2">
			<a href="/atendimentos" class="btn btn-primary">
				<Headphones class="size-4" />
				<span>Registrar atendimento</span>
			</a>
			<a href="/tarefas" class="btn btn-secondary">
				<Plus class="size-4" />
				<span>Nova tarefa</span>
			</a>
		</div>
	</div>

	{#if loading}
		<div class="flex justify-center py-20"><div class="spinner"></div></div>
	{:else if dados}
		<div class="grid grid-cols-1 lg:grid-cols-3 gap-5 items-start">
			<!-- Agenda -->
			<section class="panel">
				{@render cabecalho('Agenda', dados.proximos_eventos.length, '/calendario', 'Calendário')}
				{#if dados.proximos_eventos.length === 0}
					{@render vazio('Nada marcado para hoje ou amanhã.')}
				{:else}
					<ul class="divide-y divide-line">
						{#each dados.proximos_eventos.slice(0, 6) as ev (ev.id)}
							<li class="flex gap-4 px-5 py-3">
								<div class="w-14 shrink-0 tabular">
									<div class="text-[13px] font-semibold text-ink-3">{rotuloDia(ev.inicio)}</div>
									<div class="text-[15px] font-bold text-ink">{ev.dia_inteiro ? 'Dia todo' : hora(ev.inicio)}</div>
								</div>
								<div class="min-w-0 flex-1 border-l-[3px] pl-3" style="border-color: {ev.criador_cor};">
									<div class="font-semibold text-ink text-sm leading-snug">{ev.titulo}</div>
									<div class="flex gap-3 text-[13px] text-ink-3 mt-0.5 min-w-0">
										{#if !ev.dia_inteiro}<span class="shrink-0">até {hora(ev.fim)}</span>{/if}
										<span class="truncate">{ev.criador_nome}</span>
									</div>
								</div>
							</li>
						{/each}
					</ul>
					{#if dados.proximos_eventos.length > 6}
						<a href="/calendario" class="block px-5 py-3 border-t border-line text-[13px] font-semibold text-ink-3 hover:text-accent">
							Mais {dados.proximos_eventos.length - 6} no calendário
						</a>
					{/if}
				{/if}
			</section>

			<!-- Minhas tarefas -->
			<section class="panel">
				{@render cabecalho('Minhas tarefas', dados.tarefas_pendentes.length, '/tarefas', 'Ver todas')}
				{#if dados.tarefas_pendentes.length === 0}
					{@render vazio('Nenhuma tarefa pendente com você.')}
				{:else}
					<ul class="divide-y divide-line">
						{#each dados.tarefas_pendentes as t (t.id)}
							<li class="flex items-start gap-3 px-5 py-3">
								<span
									class="mt-1.5 size-2 shrink-0 rounded-full {t.atrasada ? 'bg-danger' : t.prioridade === 'alta' ? 'bg-warn' : 'bg-line-strong'}"
									aria-hidden="true"
								></span>
								<div class="min-w-0 flex-1">
									<div class="font-semibold text-ink text-sm leading-snug">{t.titulo}</div>
									<div class="text-[13px] mt-0.5 {t.atrasada ? 'text-danger font-semibold' : 'text-ink-3'}">
										{t.atrasada ? 'Atrasada, prazo' : t.prazo ? 'Prazo' : 'Sem prazo'}
										{#if t.prazo}{new Date(t.prazo).toLocaleDateString('pt-BR')}{/if}
									</div>
								</div>
								{#if t.prioridade === 'alta'}
									<Flag class="size-3.5 mt-1 text-warn shrink-0" aria-label="Prioridade alta" />
								{/if}
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<!-- Estoque abaixo do mínimo -->
			<section class="panel">
				{@render cabecalho('Estoque baixo', dados.itens_abaixo_do_minimo.length, '/estoque', 'Estoque')}
				{#if dados.itens_abaixo_do_minimo.length === 0}
					{@render vazio('Todos os itens estão acima do mínimo.')}
				{:else}
					<ul class="divide-y divide-line">
						{#each dados.itens_abaixo_do_minimo as it (it.id)}
							{@const pct = it.estoque_minimo > 0 ? Math.max(0, Math.min(100, (it.saldo / it.estoque_minimo) * 100)) : 0}
							<li class="px-5 py-3">
								<div class="flex items-baseline justify-between gap-3">
									<span class="font-semibold text-ink text-sm truncate">{it.nome}</span>
									<span class="text-sm tabular whitespace-nowrap">
										<strong class="text-danger">{it.saldo}</strong>
										<span class="text-ink-3">/ {it.estoque_minimo} {it.unidade}</span>
									</span>
								</div>
								<div class="mt-2 h-1.5 rounded-full bg-muted overflow-hidden" aria-hidden="true">
									<div class="h-full rounded-full bg-danger" style="width: {pct}%"></div>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		</div>

		<!-- Atalhos -->
		<nav class="grid grid-cols-2 md:grid-cols-4 gap-3" aria-label="Atalhos">
			{#each [
				{ href: '/atendimentos', label: 'Atendimentos', desc: 'Histórico e busca', icon: Headphones },
				{ href: '/tarefas', label: 'Tarefas', desc: 'Quadro da equipe', icon: CheckSquare },
				{ href: '/calendario', label: 'Calendário', desc: 'Reuniões da semana', icon: Calendar },
				{ href: '/estoque', label: 'Estoque', desc: 'Saldos e movimentações', icon: Package }
			] as atalho}
				{@const Icon = atalho.icon}
				<a href={atalho.href} class="group flex items-center gap-3 px-4 py-3.5 rounded-xl border border-line hover:bg-surface hover:border-line-strong transition-colors">
					<Icon class="size-5 text-ink-3 group-hover:text-accent transition-colors" />
					<div class="min-w-0">
						<div class="text-sm font-semibold text-ink">{atalho.label}</div>
						<div class="text-[13px] text-ink-3 truncate">{atalho.desc}</div>
					</div>
				</a>
			{/each}
		</nav>
	{:else}
		<div class="alert bg-danger-soft text-danger">Não foi possível carregar o painel. Recarregue a página para tentar de novo.</div>
	{/if}
</div>
