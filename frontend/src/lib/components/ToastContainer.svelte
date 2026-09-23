<script lang="ts">
	import { toast } from '$lib/toast.svelte';
	import { AlertCircle, CheckCircle2, Info, X } from 'lucide-svelte';
</script>

{#if toast.toasts.length > 0}
	<div class="fixed bottom-5 right-5 z-50 flex flex-col gap-2 max-w-sm w-full pointer-events-none px-4 sm:px-0">
		{#each toast.toasts as t (t.id)}
			<div
				class="pointer-events-auto flex items-start gap-3 p-4 rounded-xl shadow-lg border text-sm backdrop-blur-xs transition-all duration-200 animate-in fade-in slide-in-from-bottom-2 {
					t.tipo === 'erro' 
						? 'bg-red-50/95 border-red-200 text-red-800' 
						: t.tipo === 'sucesso' 
							? 'bg-emerald-50/95 border-emerald-200 text-emerald-800' 
							: 'bg-slate-900/95 border-slate-700 text-white'
				}"
			>
				<div class="shrink-0 mt-0.5">
					{#if t.tipo === 'erro'}
						<AlertCircle class="w-5 h-5 text-red-600" />
					{:else if t.tipo === 'sucesso'}
						<CheckCircle2 class="w-5 h-5 text-emerald-600" />
					{:else}
						<Info class="w-5 h-5 text-blue-400" />
					{/if}
				</div>

				<div class="flex-1 font-medium break-words leading-tight">
					{t.mensagem}
				</div>

				<button
					onclick={() => toast.remove(t.id)}
					class="shrink-0 p-1 rounded-lg hover:bg-black/5 transition cursor-pointer"
					aria-label="Fechar notificação"
				>
					<X class="w-4 h-4 opacity-70" />
				</button>
			</div>
		{/each}
	</div>
{/if}
