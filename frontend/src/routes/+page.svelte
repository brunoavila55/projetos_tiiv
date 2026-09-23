<script lang="ts">
	import { auth } from '$lib/auth.svelte';
	import { 
		Calendar, 
		CheckSquare, 
		AlertTriangle, 
		Headphones, 
		Package, 
		ArrowRight,
		Clock
	} from 'lucide-svelte';
</script>

<div class="space-y-6">
	<!-- Banner de boas-vindas -->
	<div class="bg-white rounded-2xl p-6 shadow-xs border border-slate-200 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Olá, {auth.user?.nome}!</h1>
			<p class="text-sm text-slate-500 mt-1">Bem-vindo ao terminal interno do setor. Aqui está o resumo das suas atividades.</p>
		</div>
		<div class="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-blue-50 text-blue-700 text-xs font-semibold">
			<Clock class="w-3.5 h-3.5" />
			<span>Rede Interna Ativa</span>
		</div>
	</div>

	<!-- Blocos do Painel -->
	<div class="grid grid-cols-1 md:grid-cols-3 gap-6">
		<!-- Próximos Eventos -->
		<div class="bg-white rounded-2xl p-6 shadow-xs border border-slate-200 flex flex-col justify-between">
			<div>
				<div class="flex items-center justify-between mb-4">
					<div class="flex items-center gap-2 font-bold text-slate-800">
						<div class="p-2 rounded-xl bg-blue-50 text-blue-600">
							<Calendar class="w-5 h-5" />
						</div>
						<span>Próximos Eventos</span>
					</div>
					<span class="text-xs font-medium text-slate-400">Hoje e amanhã</span>
				</div>
				<div class="text-sm text-slate-500 py-6 text-center border border-dashed border-slate-200 rounded-xl">
					Nenhum evento agendado para hoje.
				</div>
			</div>
			<a href="/calendario" class="mt-4 flex items-center justify-between text-sm font-semibold text-blue-600 hover:text-blue-700 pt-2 border-t border-slate-100">
				<span>Abrir calendário</span>
				<ArrowRight class="w-4 h-4" />
			</a>
		</div>

		<!-- Minhas Tarefas Pendentes -->
		<div class="bg-white rounded-2xl p-6 shadow-xs border border-slate-200 flex flex-col justify-between">
			<div>
				<div class="flex items-center justify-between mb-4">
					<div class="flex items-center gap-2 font-bold text-slate-800">
						<div class="p-2 rounded-xl bg-amber-50 text-amber-600">
							<CheckSquare class="w-5 h-5" />
						</div>
						<span>Minhas Tarefas</span>
					</div>
					<span class="text-xs font-medium text-slate-400">Pendentes</span>
				</div>
				<div class="text-sm text-slate-500 py-6 text-center border border-dashed border-slate-200 rounded-xl">
					Nenhuma tarefa pendente atribuída.
				</div>
			</div>
			<a href="/tarefas" class="mt-4 flex items-center justify-between text-sm font-semibold text-amber-600 hover:text-amber-700 pt-2 border-t border-slate-100">
				<span>Ver todas as tarefas</span>
				<ArrowRight class="w-4 h-4" />
			</a>
		</div>

		<!-- Alerta de Estoque Mínimo -->
		<div class="bg-white rounded-2xl p-6 shadow-xs border border-slate-200 flex flex-col justify-between">
			<div>
				<div class="flex items-center justify-between mb-4">
					<div class="flex items-center gap-2 font-bold text-slate-800">
						<div class="p-2 rounded-xl bg-red-50 text-red-600">
							<AlertTriangle class="w-5 h-5" />
						</div>
						<span>Estoque Crítico</span>
					</div>
					<span class="text-xs font-medium text-red-500">Abaixo do mín.</span>
				</div>
				<div class="text-sm text-slate-500 py-6 text-center border border-dashed border-slate-200 rounded-xl">
					Todos os itens estão com saldo regular.
				</div>
			</div>
			<a href="/estoque" class="mt-4 flex items-center justify-between text-sm font-semibold text-red-600 hover:text-red-700 pt-2 border-t border-slate-100">
				<span>Acessar estoque</span>
				<ArrowRight class="w-4 h-4" />
			</a>
		</div>
	</div>

	<!-- Atalhos rápidos -->
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
</div>
