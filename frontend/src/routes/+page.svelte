<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { 
		Calendar, 
		CheckSquare, 
		AlertTriangle, 
		Headphones, 
		Package, 
		ArrowRight,
		Clock,
		Flag,
		AlertCircle
	} from 'lucide-svelte';

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
</script>

<div class="space-y-6">
	<!-- Banner de Boas-vindas -->
	<div class="bg-white rounded-2xl p-6 shadow-xs border border-slate-200 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Olá, {auth.user?.nome}!</h1>
			<p class="text-sm text-slate-500 mt-1">Terminal de atendimento e controle interno do setor.</p>
		</div>
		<div class="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-blue-50 text-blue-700 text-xs font-semibold">
			<Clock class="w-3.5 h-3.5" />
			<span>Rede Interna Ativa</span>
		</div>
	</div>

	{#if loading}
		<div class="flex justify-center py-20">
			<div class="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
		</div>
	{:else if dados}
		<!-- Grade dos 3 Blocos Principais -->
		<div class="grid grid-cols-1 md:grid-cols-3 gap-6">
			<!-- 1. Próximos Eventos (Hoje e Amanhã) -->
			<div class="bg-white rounded-2xl p-6 shadow-xs border border-slate-200 flex flex-col justify-between">
				<div class="space-y-4">
					<div class="flex items-center justify-between border-b border-slate-100 pb-3">
						<div class="flex items-center gap-2 font-bold text-slate-800">
							<div class="p-2 rounded-xl bg-blue-50 text-blue-600">
								<Calendar class="w-5 h-5" />
							</div>
							<span>Próximos Eventos</span>
						</div>
						<span class="text-xs font-semibold text-slate-400">Hoje e Amanhã</span>
					</div>

					{#if dados.proximos_eventos.length === 0}
						<div class="text-xs text-slate-400 py-8 text-center border border-dashed border-slate-200 rounded-xl">
							Nenhum evento agendado para hoje ou amanhã.
						</div>
					{:else}
						<div class="space-y-2.5">
							{#each dados.proximos_eventos as ev (ev.id)}
								<div class="p-3 rounded-xl bg-slate-50/80 border border-slate-100 flex items-start gap-2.5">
									<span class="w-2.5 h-2.5 rounded-full mt-1.5 flex-shrink-0" style="background-color: {ev.criador_cor};"></span>
									<div class="min-w-0 flex-1">
										<div class="font-semibold text-slate-800 text-sm truncate">{ev.titulo}</div>
										<div class="text-xs text-slate-500 mt-0.5">
											{new Date(ev.inicio).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })} - {new Date(ev.fim).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}
										</div>
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<a href="/calendario" class="mt-4 flex items-center justify-between text-sm font-semibold text-blue-600 hover:text-blue-700 pt-3 border-t border-slate-100">
					<span>Abrir calendário</span>
					<ArrowRight class="w-4 h-4" />
				</a>
			</div>

			<!-- 2. Minhas Tarefas Pendentes (Atrasadas Primeiro) -->
			<div class="bg-white rounded-2xl p-6 shadow-xs border border-slate-200 flex flex-col justify-between">
				<div class="space-y-4">
					<div class="flex items-center justify-between border-b border-slate-100 pb-3">
						<div class="flex items-center gap-2 font-bold text-slate-800">
							<div class="p-2 rounded-xl bg-amber-50 text-amber-600">
								<CheckSquare class="w-5 h-5" />
							</div>
							<span>Minhas Tarefas</span>
						</div>
						<span class="text-xs font-semibold text-slate-400">Pendentes</span>
					</div>

					{#if dados.tarefas_pendentes.length === 0}
						<div class="text-xs text-slate-400 py-8 text-center border border-dashed border-slate-200 rounded-xl">
							Você não possui tarefas pendentes atribuídas.
						</div>
					{:else}
						<div class="space-y-2.5">
							{#each dados.tarefas_pendentes as t (t.id)}
								<div class="p-3 rounded-xl border flex items-start justify-between gap-2 {t.atrasada ? 'bg-red-50/50 border-red-200' : 'bg-slate-50/80 border-slate-100'}">
									<div class="min-w-0 flex-1">
										<div class="font-semibold text-slate-800 text-sm truncate flex items-center gap-1.5">
											<span>{t.titulo}</span>
											{#if t.atrasada}
												<span class="inline-flex text-[10px] font-bold text-red-600 bg-red-100 px-1.5 py-0.2 rounded">Atrasada</span>
											{/if}
										</div>
										<div class="text-xs text-slate-400 mt-0.5">
											{t.prazo ? `Prazo: ${new Date(t.prazo).toLocaleDateString('pt-BR')}` : 'Sem prazo'}
										</div>
									</div>
									{#if t.prioridade === 'alta'}
										<span class="text-xs text-red-600 font-bold flex items-center gap-0.5">
											<Flag class="w-3 h-3" />
										</span>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<a href="/tarefas" class="mt-4 flex items-center justify-between text-sm font-semibold text-amber-600 hover:text-amber-700 pt-3 border-t border-slate-100">
					<span>Ver todas as tarefas</span>
					<ArrowRight class="w-4 h-4" />
				</a>
			</div>

			<!-- 3. Itens Abaixo do Estoque Mínimo -->
			<div class="bg-white rounded-2xl p-6 shadow-xs border border-slate-200 flex flex-col justify-between">
				<div class="space-y-4">
					<div class="flex items-center justify-between border-b border-slate-100 pb-3">
						<div class="flex items-center gap-2 font-bold text-slate-800">
							<div class="p-2 rounded-xl bg-red-50 text-red-600">
								<AlertTriangle class="w-5 h-5" />
							</div>
							<span>Estoque Crítico</span>
						</div>
						<span class="text-xs font-semibold text-red-500">Abaixo do mín.</span>
					</div>

					{#if dados.itens_abaixo_do_minimo.length === 0}
						<div class="text-xs text-slate-400 py-8 text-center border border-dashed border-slate-200 rounded-xl">
							Todos os itens estão com saldo regular.
						</div>
					{:else}
						<div class="space-y-2.5">
							{#each dados.itens_abaixo_do_minimo as it (it.id)}
								<div class="p-3 rounded-xl bg-red-50/40 border border-red-200 flex items-center justify-between gap-2">
									<div class="min-w-0 flex-1">
										<div class="font-semibold text-slate-800 text-sm truncate">{it.nome}</div>
										<div class="text-xs text-slate-500 mt-0.5">Mínimo: {it.estoque_minimo} {it.unidade}</div>
									</div>
									<div class="text-right">
										<div class="text-base font-extrabold text-red-600">{it.saldo}</div>
										<div class="text-[10px] text-slate-400">{it.unidade}</div>
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<a href="/estoque" class="mt-4 flex items-center justify-between text-sm font-semibold text-red-600 hover:text-red-700 pt-3 border-t border-slate-100">
					<span>Acessar estoque</span>
					<ArrowRight class="w-4 h-4" />
				</a>
			</div>
		</div>

		<!-- Atalhos Rápidos -->
		<div class="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-2">
			<a href="/atendimentos" class="p-4 bg-white rounded-xl border border-slate-200 hover:border-blue-400 shadow-xs flex items-center gap-3 transition">
				<div class="p-2 rounded-lg bg-emerald-50 text-emerald-600">
					<Headphones class="w-5 h-5" />
				</div>
				<div>
					<div class="text-sm font-bold text-slate-800">Atendimentos</div>
					<div class="text-xs text-slate-400">Registrar novo</div>
				</div>
			</a>

			<a href="/calendario" class="p-4 bg-white rounded-xl border border-slate-200 hover:border-blue-400 shadow-xs flex items-center gap-3 transition">
				<div class="p-2 rounded-lg bg-blue-50 text-blue-600">
					<Calendar class="w-5 h-5" />
				</div>
				<div>
					<div class="text-sm font-bold text-slate-800">Reuniões</div>
					<div class="text-xs text-slate-400">Ver agenda</div>
				</div>
			</a>

			<a href="/tarefas" class="p-4 bg-white rounded-xl border border-slate-200 hover:border-blue-400 shadow-xs flex items-center gap-3 transition">
				<div class="p-2 rounded-lg bg-amber-50 text-amber-600">
					<CheckSquare class="w-5 h-5" />
				</div>
				<div>
					<div class="text-sm font-bold text-slate-800">Tarefas</div>
					<div class="text-xs text-slate-400">Criar demanda</div>
				</div>
			</a>

			<a href="/estoque" class="p-4 bg-white rounded-xl border border-slate-200 hover:border-blue-400 shadow-xs flex items-center gap-3 transition">
				<div class="p-2 rounded-lg bg-indigo-50 text-indigo-600">
					<Package class="w-5 h-5" />
				</div>
				<div>
					<div class="text-sm font-bold text-slate-800">Estoque</div>
					<div class="text-xs text-slate-400">Movimentar item</div>
				</div>
			</a>
		</div>
	{/if}
</div>
