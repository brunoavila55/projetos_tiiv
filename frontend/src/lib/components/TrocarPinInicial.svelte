<script lang="ts">
	import { apiFetch, ApiError } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { KeyRound } from 'lucide-svelte';

	let pinAtual = $state('');
	let novoPin = $state('');
	let confirmacao = $state('');
	let erro = $state<string | null>(null);
	let enviando = $state(false);

	async function trocar(e: SubmitEvent) {
		e.preventDefault();
		if (enviando) return;
		erro = null;
		if (novoPin !== confirmacao) {
			erro = 'A confirmação não confere com o novo PIN.';
			return;
		}
		enviando = true;
		try {
			await apiFetch('/api/auth/trocar-pin', {
				method: 'POST',
				silent: true,
				body: JSON.stringify({ pin_atual: pinAtual, novo_pin: novoPin })
			});
			if (auth.user) auth.user = { ...auth.user, deve_trocar_pin: false };
		} catch (err) {
			// 423: tentativas esgotadas, o servidor encerrou a sessão
			if (err instanceof ApiError && err.status === 423) {
				auth.user = null;
				return;
			}
			erro = err instanceof Error ? err.message : 'Não foi possível trocar o PIN.';
			pinAtual = '';
		} finally {
			enviando = false;
		}
	}
</script>

<div class="min-h-dvh bg-paper text-ink grid place-items-center px-4 py-10">
	<section class="w-full max-w-sm rounded-2xl bg-surface shadow-float p-6 sm:p-8">
		<span class="grid size-12 place-items-center rounded-xl bg-accent-soft text-accent">
			<KeyRound class="size-6" />
		</span>
		<h1 class="mt-4 text-2xl font-bold leading-tight tracking-[-0.015em]">Troque seu PIN</h1>
		<p class="mt-1.5 text-ink-3">
			Este é o primeiro acesso de {auth.user?.nome}. O PIN inicial foi definido na instalação e precisa ser trocado antes de
			continuar.
		</p>

		<form class="mt-6 space-y-4" onsubmit={trocar}>
			<div>
				<label class="label" for="pin-atual">PIN atual</label>
				<input id="pin-atual" bind:value={pinAtual} type="password" inputmode="numeric" pattern="[0-9]{4}" maxlength="4" required autocomplete="current-password" class="field h-11 tabular" />
			</div>
			<div>
				<label class="label" for="pin-novo">Novo PIN</label>
				<input id="pin-novo" bind:value={novoPin} type="password" inputmode="numeric" pattern="[0-9]{4}" maxlength="4" required autocomplete="new-password" class="field h-11 tabular" />
				<p class="hint mt-1">4 dígitos. Sequências e dígitos repetidos (1234, 0000) não são aceitos.</p>
			</div>
			<div>
				<label class="label" for="pin-confirma">Confirme o novo PIN</label>
				<input id="pin-confirma" bind:value={confirmacao} type="password" inputmode="numeric" pattern="[0-9]{4}" maxlength="4" required autocomplete="new-password" class="field h-11 tabular" />
			</div>

			{#if erro}
				<p class="text-sm font-medium text-danger" role="alert">{erro}</p>
			{/if}

			<button type="submit" disabled={enviando} class="btn btn-primary w-full h-11">
				{enviando ? 'Salvando…' : 'Trocar PIN e entrar'}
			</button>
			<button type="button" onclick={() => auth.logout()} class="btn btn-secondary w-full h-11">Sair</button>
		</form>
	</section>
</div>
