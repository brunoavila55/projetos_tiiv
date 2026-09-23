<script lang="ts">
	import { Package } from 'lucide-svelte';
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Estoque</h1>
			<p class="text-sm text-slate-500">Controle por movimentações (entrada, saída, ajuste)</p>
		</div>
	</div>

	<div class="bg-white rounded-2xl p-6 border border-slate-200 shadow-xs">
		<div class="text-center py-12 text-slate-500">
			<Package class="w-12 h-12 mx-auto mb-3 text-slate-300" />
			<p class="font-medium">Módulo de Estoque</p>
			<p class="text-xs text-slate-400 mt-1">Transações atômicas com bloqueio concorrente SELECT FOR UPDATE</p>
		</div>
	</div>
</div>
