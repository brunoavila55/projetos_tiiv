<script lang="ts">
	import { radio, type Estacao } from '$lib/radio.svelte';
	import { Play, Pause, Square, Star, Search, X, Volume2, VolumeX, Radio } from 'lucide-svelte';

	let aba = $state<'populares' | 'favoritas'>('populares');
	let termo = $state('');
	let estacoes = $state<Estacao[]>([]);
	let buscando = $state(false);
	let erroBusca = $state('');
	let carregou = false;
	let seq = 0;

	async function buscar() {
		const minha = ++seq;
		buscando = true;
		erroBusca = '';
		try {
			const lista = await radio.buscar(termo);
			if (minha === seq) estacoes = lista;
		} catch {
			if (minha === seq) erroBusca = 'Não foi possível consultar as rádios. Tente novamente.';
		} finally {
			if (minha === seq) buscando = false;
		}
	}

	// Carrega as populares na primeira vez que o painel abre
	$effect(() => {
		if (radio.aberto && !carregou) {
			carregou = true;
			buscar();
		}
	});

	let timer: ReturnType<typeof setTimeout>;
	function aoDigitar() {
		aba = 'populares';
		clearTimeout(timer);
		timer = setTimeout(buscar, 400);
	}

	function aoTeclar(e: KeyboardEvent) {
		if (e.key === 'Escape' && radio.aberto) radio.aberto = false;
	}

	function detalhe(e: Estacao): string {
		const local = [e.state, e.country].filter(Boolean).join(', ');
		const tags = e.tags.split(',').filter(Boolean).slice(0, 2).join(' · ');
		return [local, tags].filter(Boolean).join(' — ');
	}

	const lista = $derived(aba === 'favoritas' ? radio.favoritas : estacoes);
	const tocandoOuCarregando = $derived(radio.estado === 'tocando' || radio.estado === 'carregando');
</script>

<svelte:window onkeydown={aoTeclar} />

{#if radio.aberto}
	<div class="fixed inset-0 z-40" onclick={() => (radio.aberto = false)} aria-hidden="true"></div>
	<div
		class="fixed z-50 inset-x-0 bottom-0 sm:inset-x-auto sm:left-4 sm:bottom-4 lg:left-[15.5rem] sm:w-[380px] max-h-[80vh] flex flex-col bg-surface border border-line shadow-float rounded-t-2xl sm:rounded-2xl text-ink radio-in"
		role="dialog"
		aria-label="Rádio"
	>
		<div class="flex items-center justify-between gap-3 px-4 pt-4 pb-3">
			<div class="flex items-center gap-2 font-bold">
				<Radio class="size-[18px] text-accent" />
				Rádio
			</div>
			<button class="icon-btn" onclick={() => (radio.aberto = false)} aria-label="Fechar rádio">
				<X class="size-[18px]" />
			</button>
		</div>

		<!-- Tocando agora -->
		{#if radio.atual}
			<div class="mx-4 mb-3 p-3 rounded-xl bg-sunken border border-line">
				<div class="flex items-center gap-3">
					<button
						class="grid place-items-center size-10 shrink-0 rounded-full bg-accent text-on-accent hover:bg-accent-strong transition-colors cursor-pointer"
						onclick={() => radio.alternar()}
						aria-label={tocandoOuCarregando ? 'Parar' : 'Tocar'}
					>
						{#if radio.estado === 'carregando'}
							<div class="size-4 rounded-full border-2 border-on-accent/40 border-t-on-accent animate-spin"></div>
						{:else if radio.estado === 'tocando'}
							<Square class="size-4" fill="currentColor" />
						{:else}
							<Play class="size-4 translate-x-px" fill="currentColor" />
						{/if}
					</button>
					<div class="min-w-0 flex-1">
						<div class="text-sm font-semibold truncate">{radio.atual.name}</div>
						<div class="text-xs {radio.estado === 'erro' ? 'text-danger' : 'text-ink-3'}">
							{#if radio.estado === 'erro'}
								Não foi possível tocar esta rádio
							{:else if radio.estado === 'carregando'}
								Conectando…
							{:else if radio.estado === 'tocando'}
								Ao vivo{radio.atual.bitrate ? ` · ${radio.atual.bitrate} kbps` : ''}
							{:else}
								Parada
							{/if}
						</div>
					</div>
				</div>
				<div class="flex items-center gap-2 mt-3">
					<button
						class="icon-btn size-7"
						onclick={() => radio.setVolume(radio.volume > 0 ? 0 : 0.7)}
						aria-label={radio.volume > 0 ? 'Silenciar' : 'Ativar som'}
					>
						{#if radio.volume > 0}
							<Volume2 class="size-4" />
						{:else}
							<VolumeX class="size-4" />
						{/if}
					</button>
					<input
						type="range"
						min="0"
						max="1"
						step="0.05"
						value={radio.volume}
						oninput={(e) => radio.setVolume(Number(e.currentTarget.value))}
						class="flex-1"
						style="accent-color: var(--accent)"
						aria-label="Volume"
					/>
				</div>
			</div>
		{/if}

		<div class="px-4 space-y-3">
			<div class="relative">
				<Search class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-ink-3 pointer-events-none" />
				<input
					type="search"
					class="field pl-9"
					placeholder="Buscar rádio pelo nome"
					bind:value={termo}
					oninput={aoDigitar}
					aria-label="Buscar rádio"
				/>
			</div>
			<div class="segmented w-full">
				<button class="flex-1" aria-pressed={aba === 'populares'} onclick={() => (aba = 'populares')}>
					{termo.trim() ? 'Resultados' : 'Populares no Brasil'}
				</button>
				<button class="flex-1" aria-pressed={aba === 'favoritas'} onclick={() => (aba = 'favoritas')}>
					Favoritas{radio.favoritas.length ? ` (${radio.favoritas.length})` : ''}
				</button>
			</div>
		</div>

		<div class="mt-2 px-2 pb-3 overflow-y-auto min-h-32">
			{#if aba === 'populares' && buscando}
				<div class="flex justify-center py-10"><div class="spinner size-5"></div></div>
			{:else if aba === 'populares' && erroBusca}
				<p class="px-2 py-8 text-center text-sm text-danger">{erroBusca}</p>
			{:else if lista.length === 0}
				<p class="px-2 py-8 text-center text-sm text-ink-3">
					{aba === 'favoritas' ? 'Marque rádios com a estrela para achá-las aqui.' : 'Nenhuma rádio encontrada.'}
				</p>
			{:else}
				<ul>
					{#each lista as e (e.stationuuid)}
						{@const ativa = radio.atual?.stationuuid === e.stationuuid}
						{@const fav = radio.ehFavorita(e.stationuuid)}
						<li class="flex items-center gap-1 rounded-lg {ativa ? 'bg-accent-soft' : 'hover:bg-muted'}">
							<button
								class="flex-1 min-w-0 flex items-center gap-3 px-2 py-2 text-left cursor-pointer"
								onclick={() => (ativa && tocandoOuCarregando ? radio.parar() : radio.tocar(e))}
							>
								<span class="grid place-items-center size-8 shrink-0 rounded-full {ativa ? 'bg-accent text-on-accent' : 'bg-muted text-ink-3'}">
									{#if ativa && tocandoOuCarregando}
										<Pause class="size-3.5" fill="currentColor" />
									{:else}
										<Play class="size-3.5 translate-x-px" fill="currentColor" />
									{/if}
								</span>
								<span class="min-w-0">
									<span class="block text-sm font-semibold truncate {ativa ? 'text-accent' : ''}">{e.name.trim()}</span>
									<span class="block text-xs text-ink-3 truncate">{detalhe(e)}</span>
								</span>
							</button>
							<button
								class="icon-btn mr-1"
								onclick={() => radio.alternarFavorita(e)}
								aria-label={fav ? 'Remover das favoritas' : 'Adicionar às favoritas'}
								aria-pressed={fav}
							>
								<Star class="size-4 {fav ? 'text-warn' : ''}" fill={fav ? 'currentColor' : 'none'} />
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<p class="px-4 pb-3 text-[11px] text-ink-3">Estações via Radio Browser.</p>
	</div>
{/if}

<style>
	.radio-in {
		animation: radio-in 160ms cubic-bezier(0.2, 0.8, 0.3, 1);
	}
	@keyframes radio-in {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
	}
</style>
