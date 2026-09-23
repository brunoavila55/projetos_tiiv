<script lang="ts">
	import { toast } from '$lib/toast.svelte';
	import { AlertCircle, CheckCircle2, Info, X } from 'lucide-svelte';
</script>

{#if toast.toasts.length > 0}
	<div
		class="fixed bottom-4 right-4 left-4 sm:left-auto z-[60] flex flex-col gap-2 sm:w-96 pointer-events-none"
		role="status"
		aria-live="polite"
	>
		{#each toast.toasts as t (t.id)}
			<div
				class="pointer-events-auto flex items-start gap-3 pl-4 pr-2 py-3 rounded-xl bg-surface border border-line shadow-float text-sm text-ink border-l-4 {t.tipo ===
				'erro'
					? 'border-l-danger'
					: t.tipo === 'sucesso'
						? 'border-l-ok'
						: 'border-l-accent'}"
				style="animation: modal-in 160ms ease-out;"
			>
				<div class="shrink-0 mt-px">
					{#if t.tipo === 'erro'}
						<AlertCircle class="size-[18px] text-danger" />
					{:else if t.tipo === 'sucesso'}
						<CheckCircle2 class="size-[18px] text-ok" />
					{:else}
						<Info class="size-[18px] text-accent" />
					{/if}
				</div>

				<div class="flex-1 font-medium break-words leading-snug pt-px">
					{t.mensagem}
				</div>

				<button onclick={() => toast.remove(t.id)} class="icon-btn size-7 -my-0.5" aria-label="Fechar notificação">
					<X class="size-4" />
				</button>
			</div>
		{/each}
	</div>
{/if}
