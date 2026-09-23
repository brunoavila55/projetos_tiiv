<script lang="ts">
	import { Users } from 'lucide-svelte';
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold text-slate-800">Gestão de Usuários</h1>
			<p class="text-sm text-slate-500">Administração de operadores, PINs e permissões</p>
		</div>
	</div>

	<div class="bg-white rounded-2xl p-6 border border-slate-200 shadow-xs">
		<div class="text-center py-12 text-slate-500">
			<Users class="w-12 h-12 mx-auto mb-3 text-purple-400" />
			<p class="font-medium">Gestão de Usuários (Admin)</p>
			<p class="text-xs text-slate-400 mt-1">CRUD de usuários, redefinição de PIN e desbloqueio de operadores</p>
		</div>
	</div>
</div>
