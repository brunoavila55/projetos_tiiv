<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch, ApiError } from '$lib/api';
	import { auth, type UsuarioPublico } from '$lib/auth.svelte';
	import { Delete, X, AlertCircle, Lock, ShieldAlert } from 'lucide-svelte';

	let usuarios = $state<UsuarioPublico[]>([]);
	let loading = $state<boolean>(true);
	let errorMsg = $state<string | null>(null);

	// Estado do modal de PIN
	let selectedUser = $state<UsuarioPublico | null>(null);
	let pin = $state<string>('');
	let pinError = $state<string | null>(null);
	let submitting = $state<boolean>(false);

	async function carregarUsuarios() {
		loading = true;
		errorMsg = null;
		try {
			usuarios = await apiFetch<UsuarioPublico[]>('/api/auth/usuarios');
		} catch (err: any) {
			errorMsg = err.message || 'Erro ao carregar usuários';
		} finally {
			loading = false;
		}
	}

	function abrirTeclado(u: UsuarioPublico) {
		selectedUser = u;
		pin = '';
		pinError = null;
	}

	function fecharTeclado() {
		selectedUser = null;
		pin = '';
		pinError = null;
	}

	function adicionarDigito(d: string) {
		if (pin.length < 6) {
			pin += d;
			pinError = null;
			// Se atingir 4 a 6 dígitos, o usuário pode clicar em Entrar ou digitar Enter
		}
	}

	function apagarDigito() {
		if (pin.length > 0) {
			pin = pin.slice(0, -1);
			pinError = null;
		}
	}

	async function submeterPin() {
		if (!selectedUser || pin.length < 4 || submitting) return;

		submitting = true;
		pinError = null;

		try {
			await auth.login(selectedUser.id, pin);
			selectedUser = null;
			pin = '';
		} catch (err: any) {
			pinError = err.message || 'Erro ao autenticar';
			pin = ''; // limpa pin em caso de erro
		} finally {
			submitting = false;
		}
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (!selectedUser) return;

		if (e.key >= '0' && e.key <= '9') {
			e.preventDefault();
			adicionarDigito(e.key);
		} else if (e.key === 'Backspace') {
			e.preventDefault();
			apagarDigito();
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (pin.length >= 4) {
				submeterPin();
			}
		} else if (e.key === 'Escape') {
			e.preventDefault();
			fecharTeclado();
		}
	}

	onMount(() => {
		carregarUsuarios();
		window.addEventListener('keydown', handleKeyDown);
		return () => {
			window.removeEventListener('keydown', handleKeyDown);
		};
	});

	function getIniciais(nome: string): string {
		const partes = nome.trim().split(/\s+/);
		if (partes.length === 1) return partes[0].slice(0, 2).toUpperCase();
		return (partes[0][0] + partes[partes.length - 1][0]).toUpperCase();
	}
</script>

<div class="min-h-screen bg-slate-100 flex flex-col items-center justify-center p-4 sm:p-6 select-none">
	<div class="w-full max-w-4xl">
		<!-- Cabeçalho -->
		<div class="text-center mb-8">
			<h1 class="text-3xl font-extrabold text-slate-800 tracking-tight">TIIV — Terminal do Setor</h1>
			<p class="text-slate-500 mt-1 text-sm sm:text-base">Toque no seu perfil para digitar seu PIN de acesso</p>
		</div>

		{#if loading}
			<div class="flex justify-center items-center py-20">
				<div class="w-10 h-10 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
			</div>
		{:else if errorMsg}
			<div class="bg-red-50 border border-red-200 text-red-700 p-4 rounded-xl flex items-center justify-between mb-6 max-w-md mx-auto">
				<div class="flex items-center gap-2">
					<AlertCircle class="w-5 h-5 flex-shrink-0" />
					<span class="text-sm">{errorMsg}</span>
				</div>
				<button 
					onclick={carregarUsuarios}
					class="text-sm font-semibold underline hover:text-red-900"
				>
					Tentar de novo
				</button>
			</div>
		{:else if usuarios.length === 0}
			<div class="text-center py-16 bg-white rounded-2xl shadow-sm border border-slate-200 p-8 max-w-md mx-auto">
				<ShieldAlert class="w-12 h-12 text-amber-500 mx-auto mb-3" />
				<h3 class="text-lg font-bold text-slate-800">Nenhum usuário ativo</h3>
				<p class="text-sm text-slate-500 mt-1">O sistema está pronto, mas não possui usuários habilitados no momento.</p>
			</div>
		{:else}
			<!-- Grade de usuários estilo PDV -->
			<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4 sm:gap-6">
				{#each usuarios as u (u.id)}
					<button
						onclick={() => abrirTeclado(u)}
						class="flex flex-col items-center justify-center p-5 bg-white rounded-2xl shadow-sm border border-slate-200 hover:shadow-md hover:border-blue-400 active:scale-95 transition duration-150 cursor-pointer text-center group"
					>
						<div
							class="w-20 h-20 rounded-full flex items-center justify-center text-white text-2xl font-bold mb-3 shadow-inner group-hover:scale-105 transition transform"
							style="background-color: {u.cor || '#2563EB'};"
						>
							{getIniciais(u.nome)}
						</div>
						<span class="font-semibold text-slate-800 text-base line-clamp-1 w-full">{u.nome}</span>
					</button>
				{/each}
			</div>
		{/if}
	</div>

	<!-- Modal de PIN com teclado numérico grande estilo PDV -->
	{#if selectedUser}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-3xl shadow-2xl border border-slate-100 w-full max-w-sm overflow-hidden flex flex-col animate-in fade-in zoom-in-95 duration-150">
				<!-- Topo do modal -->
				<div class="p-5 pb-3 flex items-center justify-between border-b border-slate-100">
					<div class="flex items-center gap-3">
						<div 
							class="w-10 h-10 rounded-full flex items-center justify-center text-white font-bold text-sm shadow-sm"
							style="background-color: {selectedUser.cor || '#2563EB'};"
						>
							{getIniciais(selectedUser.nome)}
						</div>
						<div>
							<h3 class="font-bold text-slate-800 text-base leading-tight">{selectedUser.nome}</h3>
							<p class="text-xs text-slate-400">Digite seu PIN (4 a 6 dígitos)</p>
						</div>
					</div>
					<button
						onclick={fecharTeclado}
						class="w-9 h-9 rounded-full bg-slate-100 hover:bg-slate-200 flex items-center justify-center text-slate-500 transition"
						aria-label="Fechar"
					>
						<X class="w-5 h-5" />
					</button>
				</div>

				<!-- Mostrador de PIN mascarado -->
				<div class="px-6 py-4 flex flex-col items-center justify-center bg-slate-50/70 border-b border-slate-100">
					<div class="flex items-center gap-3 h-10">
						{#each Array(6) as _, i}
							<div
								class="w-4 h-4 rounded-full transition-all duration-150 {i < pin.length ? 'bg-blue-600 scale-110 shadow-sm' : 'border-2 border-slate-300 bg-white'}"
							></div>
						{/each}
					</div>

					{#if pinError}
						<div class="mt-3 px-3 py-1.5 rounded-lg bg-red-100 text-red-700 text-xs font-medium text-center flex items-center gap-1.5 animate-shake">
							<AlertCircle class="w-4 h-4 flex-shrink-0" />
							<span>{pinError}</span>
						</div>
					{/if}
				</div>

				<!-- Teclado Numérico Grande PDV -->
				<div class="p-4 grid grid-cols-3 gap-2.5 bg-white">
					{#each ['1', '2', '3', '4', '5', '6', '7', '8', '9'] as digit}
						<button
							type="button"
							onclick={() => adicionarDigito(digit)}
							disabled={submitting}
							class="h-14 rounded-2xl bg-slate-100 hover:bg-slate-200 active:bg-blue-100 active:text-blue-700 text-2xl font-semibold text-slate-800 flex items-center justify-center transition active:scale-95 cursor-pointer disabled:opacity-50"
						>
							{digit}
						</button>
					{/each}

					<!-- Botão Limpar/Cancelar -->
					<button
						type="button"
						onclick={fecharTeclado}
						disabled={submitting}
						class="h-14 rounded-2xl bg-slate-50 hover:bg-slate-100 active:scale-95 text-sm font-semibold text-slate-600 flex items-center justify-center transition cursor-pointer disabled:opacity-50"
					>
						Cancelar
					</button>

					<!-- Tecla 0 -->
					<button
						type="button"
						onclick={() => adicionarDigito('0')}
						disabled={submitting}
						class="h-14 rounded-2xl bg-slate-100 hover:bg-slate-200 active:bg-blue-100 active:text-blue-700 text-2xl font-semibold text-slate-800 flex items-center justify-center transition active:scale-95 cursor-pointer disabled:opacity-50"
					>
						0
					</button>

					<!-- Tecla Apagar (Backspace) -->
					<button
						type="button"
						onclick={apagarDigito}
						disabled={submitting || pin.length === 0}
						class="h-14 rounded-2xl bg-slate-100 hover:bg-slate-200 active:bg-slate-300 active:scale-95 text-slate-700 flex items-center justify-center transition cursor-pointer disabled:opacity-40"
						aria-label="Apagar dígito"
					>
						<Delete class="w-6 h-6" />
					</button>
				</div>

				<!-- Ação de confirmação -->
				<div class="p-4 pt-1 border-t border-slate-100 bg-white">
					<button
						type="button"
						onclick={submeterPin}
						disabled={pin.length < 4 || submitting}
						class="w-full h-12 rounded-xl bg-blue-600 hover:bg-blue-700 active:scale-98 text-white font-bold text-base flex items-center justify-center gap-2 transition disabled:opacity-40 disabled:cursor-not-allowed shadow-md shadow-blue-500/20"
					>
						{#if submitting}
							<div class="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
							<span>Verificando...</span>
						{:else}
							<Lock class="w-4 h-4" />
							<span>Entrar</span>
						{/if}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
