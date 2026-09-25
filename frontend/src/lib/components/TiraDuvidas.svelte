<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { fly } from 'svelte/transition';
	import { apiFetch, ApiError } from '$lib/api';
	import { renderMarkdown } from '$lib/markdown';
	import { MessageCircle, X, SendHorizontal, RotateCcw, Phone, ArrowRight } from 'lucide-svelte';

	interface Fonte {
		id: string;
		titulo: string;
		contato: string | null;
	}

	interface Mensagem {
		papel: 'usuario' | 'assistente';
		texto: string;
		fontes?: Fonte[];
	}

	// setorId é o mesmo do formulário de ticket: a dúvida e o ticket vão para o mesmo setor
	let {
		onAbrirTicket,
		setores = [],
		setorId = $bindable('')
	}: {
		onAbrirTicket: (descricao: string) => void;
		setores?: { id: string; nome: string }[];
		setorId?: string;
	} = $props();

	// "desligado" = sem credenciais da Cloudflare; o botão nem aparece
	let status = $state<'carregando' | 'disponivel' | 'cota' | 'desligado'>('carregando');
	let aberto = $state(false);
	let mensagens = $state<Mensagem[]>([]);
	let pergunta = $state('');
	let enviando = $state(false);
	let erro = $state<string | null>(null);
	let lista = $state<HTMLDivElement | null>(null);
	let campo = $state<HTMLTextAreaElement | null>(null);

	const perguntou = $derived(mensagens.some((m) => m.papel === 'usuario'));

	async function carregarStatus() {
		try {
			const s = await apiFetch<{ disponivel: boolean; motivo?: 'cota' | 'desligado' }>('/api/assistente/status', {
				silent: true
			});
			status = s.disponivel ? 'disponivel' : (s.motivo ?? 'desligado');
		} catch {
			status = 'desligado';
		}
	}

	async function abrir() {
		aberto = true;
		await tick();
		campo?.focus();
	}

	async function rolarParaFim() {
		await tick();
		lista?.scrollTo({ top: lista.scrollHeight, behavior: 'smooth' });
	}

	async function enviar() {
		const texto = pergunta.trim();
		if (!texto || enviando || status !== 'disponivel') return;
		if (setores.length > 1 && !setorId) {
			erro = 'Escolha o setor da sua dúvida';
			return;
		}

		mensagens.push({ papel: 'usuario', texto });
		pergunta = '';
		erro = null;
		enviando = true;
		rolarParaFim();

		try {
			const res = await apiFetch<{ resposta: string; fontes: Fonte[] }>('/api/assistente/publico', {
				method: 'POST',
				silent: true,
				body: JSON.stringify({
					mensagens: mensagens.map(({ papel, texto }) => ({ papel, texto })),
					setor_id: setorId || undefined
				})
			});
			mensagens.push({ papel: 'assistente', texto: res.resposta, fontes: res.fontes });
		} catch (err) {
			// A pergunta volta para o campo, para tentar de novo
			mensagens.pop();
			pergunta = texto;
			erro = err instanceof Error ? err.message : 'O tira-dúvidas não respondeu';
			if (err instanceof ApiError && err.status === 503) carregarStatus();
		} finally {
			enviando = false;
			rolarParaFim();
			campo?.focus();
		}
	}

	function aoTeclar(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			enviar();
		}
	}

	function novaConversa() {
		mensagens = [];
		erro = null;
		pergunta = '';
		campo?.focus();
	}

	function abrirTicket() {
		const perguntas = mensagens.filter((m) => m.papel === 'usuario').map((m) => m.texto);
		onAbrirTicket(perguntas.join('\n'));
		aberto = false;
	}

	onMount(carregarStatus);
</script>

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape' && aberto) aberto = false;
	}}
/>

{#if status === 'disponivel' || status === 'cota'}
	{#if aberto}
		<div
			class="fixed z-40 inset-0 sm:inset-auto sm:bottom-24 sm:right-5 sm:w-[25rem] sm:h-[min(38rem,calc(100dvh-8rem))] flex flex-col bg-surface text-ink sm:rounded-2xl shadow-float sm:border sm:border-line overflow-hidden"
			role="dialog"
			aria-label="Tira-dúvidas"
			transition:fly={{ y: 16, duration: 180 }}
		>
			<div class="flex items-center justify-between gap-3 px-5 py-3.5 border-b border-line">
				<div>
					<h2 class="text-base font-bold leading-tight">Tira-dúvidas</h2>
					<p class="text-[13px] text-ink-3">Respostas com base nos procedimentos do setor</p>
				</div>
				<div class="flex items-center gap-0.5 -mr-1.5">
					{#if perguntou}
						<button type="button" onclick={novaConversa} class="icon-btn" title="Nova conversa" aria-label="Nova conversa">
							<RotateCcw class="size-4" />
						</button>
					{/if}
					<button type="button" onclick={() => (aberto = false)} class="icon-btn" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>
			</div>

			{#if setores.length > 1}
				<div class="flex items-center gap-2 px-5 py-2.5 border-b border-line">
					<label for="td-setor" class="text-[13px] font-semibold text-ink-3 shrink-0">Setor</label>
					<!-- Cada setor tem os próprios procedimentos: trocar recomeça a conversa -->
					<select
						id="td-setor"
						bind:value={setorId}
						onchange={() => {
							mensagens = [];
							erro = null;
						}}
						class="field field-sm"
					>
						<option value="" disabled>Escolha o setor</option>
						{#each setores as st (st.id)}
							<option value={st.id}>{st.nome}</option>
						{/each}
					</select>
				</div>
			{/if}

			<div bind:this={lista} class="flex-1 overflow-y-auto px-4 py-4 space-y-3 bg-sunken" aria-live="polite">
				<div class="max-w-[88%] rounded-2xl rounded-tl-md bg-surface border border-line px-3.5 py-2.5 text-sm">
					Qual seria a sua dúvida?
				</div>

				{#each mensagens as m}
					{#if m.papel === 'usuario'}
						<div class="ml-auto max-w-[88%] w-fit rounded-2xl rounded-tr-md bg-accent text-on-accent px-3.5 py-2.5 text-sm whitespace-pre-wrap break-words">
							{m.texto}
						</div>
					{:else}
						<div class="max-w-[88%] space-y-2">
							<div class="rounded-2xl rounded-tl-md bg-surface border border-line px-3.5 py-2.5">
								<div class="md">{@html renderMarkdown(m.texto)}</div>
							</div>
							{#each m.fontes?.filter((f) => f.contato) ?? [] as f (f.id)}
								<div class="rounded-xl border border-accent/30 bg-accent-soft px-3.5 py-2.5">
									<p class="flex items-center gap-1.5 text-xs font-semibold text-accent">
										<Phone class="size-3.5" />
										Contato · {f.titulo}
									</p>
									<div class="md mt-1">{@html renderMarkdown(f.contato!)}</div>
								</div>
							{/each}
						</div>
					{/if}
				{/each}

				{#if enviando}
					<div class="w-fit rounded-2xl rounded-tl-md bg-surface border border-line px-3.5 py-3" aria-label="Procurando resposta">
						<span class="flex gap-1">
							<span class="size-1.5 rounded-full bg-ink-3 animate-bounce"></span>
							<span class="size-1.5 rounded-full bg-ink-3 animate-bounce [animation-delay:120ms]"></span>
							<span class="size-1.5 rounded-full bg-ink-3 animate-bounce [animation-delay:240ms]"></span>
						</span>
					</div>
				{/if}

				{#if perguntou && !enviando}
					<div class="flex justify-center pt-1">
						<button type="button" onclick={abrirTicket} class="btn btn-sm btn-ghost text-accent">
							Não resolveu? Abrir ticket
							<ArrowRight class="size-3.5" />
						</button>
					</div>
				{/if}
			</div>

			<div class="border-t border-line p-3">
				{#if status === 'cota'}
					<p class="px-1 py-2 text-sm text-ink-2">
						O tira-dúvidas atingiu o limite de hoje e volta amanhã. Se precisar de ajuda agora,
						<button type="button" onclick={abrirTicket} class="font-semibold text-accent underline underline-offset-2 cursor-pointer">abra um ticket</button>.
					</p>
				{:else}
					{#if erro}
						<p class="px-1 pb-2 text-[13px] font-medium text-danger" role="alert">{erro}</p>
					{/if}
					<form
						class="flex items-end gap-2"
						onsubmit={(e) => {
							e.preventDefault();
							enviar();
						}}
					>
						<textarea
							bind:this={campo}
							bind:value={pergunta}
							onkeydown={aoTeclar}
							rows="1"
							maxlength="1000"
							placeholder="Ex.: o servidor caiu"
							aria-label="Sua dúvida"
							class="field flex-1 min-h-10 max-h-32 resize-none py-2! [field-sizing:content]"
						></textarea>
						<button
							type="submit"
							disabled={!pergunta.trim() || enviando}
							class="btn btn-primary size-10 px-0 shrink-0"
							aria-label="Enviar"
						>
							<SendHorizontal class="size-4" />
						</button>
					</form>
				{/if}
			</div>
		</div>
	{/if}

	<button
		type="button"
		onclick={() => (aberto ? (aberto = false) : abrir())}
		aria-expanded={aberto}
		title={status === 'cota' ? 'O tira-dúvidas volta amanhã' : undefined}
		class="fixed z-30 bottom-5 right-5 {aberto ? 'hidden sm:inline-flex' : 'inline-flex'} items-center gap-2 h-12 px-4 sm:px-5 rounded-full shadow-float cursor-pointer transition-colors {status ===
		'cota'
			? 'bg-muted text-ink-3'
			: 'bg-accent text-on-accent hover:bg-accent-strong'}"
	>
		{#if aberto}
			<X class="size-5" />
		{:else}
			<MessageCircle class="size-5" />
		{/if}
		<span class="hidden sm:inline text-[15px] font-semibold">Tira-dúvidas</span>
	</button>
{/if}
