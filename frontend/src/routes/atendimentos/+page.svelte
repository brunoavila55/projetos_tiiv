<script lang="ts">
	import { Headphones } from 'lucide-svelte';
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Atendimentos</h1>
			<p class="text-sm text-slate-500">Registro rápido de atendimentos realizados</p>
		</div>
	</div>

	<div class="bg-white rounded-2xl p-6 border border-slate-200 shadow-xs">
		<div class="text-center py-12 text-slate-500">
			<Headphones class="w-12 h-12 mx-auto mb-3 text-slate-300" />
			<p class="font-medium">Módulo de Atendimentos</p>
			<p class="text-xs text-slate-400 mt-1">Busca sem acentos/maiúsculas e filtro por período</p>
		</div>
	</div>
</div>
