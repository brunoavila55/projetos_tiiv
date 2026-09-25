<script lang="ts">
	interface Props {
		id: string;
		nome: string;
		cor?: string;
		fotoVersao?: number | null;
		class?: string;
	}

	let { id, nome, cor, fotoVersao = null, class: classe = 'size-9 text-xs' }: Props = $props();

	let falhou = $state(false);

	// Uma nova versão de foto merece uma nova tentativa de carregar
	$effect(() => {
		fotoVersao;
		falhou = false;
	});

	const iniciais = $derived.by(() => {
		const partes = nome.trim().split(/\s+/);
		if (partes.length === 1) return partes[0].slice(0, 2).toUpperCase();
		return (partes[0][0] + partes[partes.length - 1][0]).toUpperCase();
	});
</script>

{#if fotoVersao && !falhou}
	<img
		src="/api/auth/usuarios/{id}/foto?v={fotoVersao}"
		alt=""
		class="shrink-0 rounded-full object-cover bg-muted {classe}"
		onerror={() => (falhou = true)}
		draggable="false"
	/>
{:else}
	<span class="avatar {classe}" style="background-color: {cor || '#1d5bbf'};" aria-hidden="true">{iniciais}</span>
{/if}
