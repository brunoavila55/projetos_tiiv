<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { 
		Users, 
		UserPlus, 
		KeyRound, 
		Unlock, 
		Edit3, 
		ShieldCheck, 
		ShieldAlert, 
		X, 
		Check,
		AlertCircle
	} from 'lucide-svelte';

	interface UsuarioItem {
		id: string;
		nome: string;
		cor: string;
		papel: 'admin' | 'usuario';
		ativo: boolean;
		tentativas_falhas: number;
		bloqueado_ate: string | null;
		criado_em: string;
	}

	let usuarios = $state<UsuarioItem[]>([]);
	let loading = $state(true);
	let errorMsg = $state<string | null>(null);
	let successMsg = $state<string | null>(null);

	// Modais
	let modalCriarAberto = $state(false);
	let modalEditarAberto = $state(false);
	let modalPinAberto = $state(false);
	let usuarioSelecionado = $state<UsuarioItem | null>(null);

	// Formulário Criar
	let formNome = $state('');
	let formCor = $state('#2563EB');
	let formPin = $state('');
	let formPapel = $state<'admin' | 'usuario'>('usuario');

	// Formulário Editar
	let editNome = $state('');
	let editCor = $state('#2563EB');
	let editPapel = $state<'admin' | 'usuario'>('usuario');
	let editAtivo = $state(true);

	// Formulário Redefinir PIN
	let novoPin = $state('');

	const paletaCores = [
		'#2563EB', '#3B82F6', '#0284C7', '#0D9488', 
		'#059669', '#16A34A', '#D97706', '#EA580C', 
		'#DC2626', '#E11D48', '#9333EA', '#4F46E5'
	];

	async function carregar() {
		loading = true;
		errorMsg = null;
		try {
			usuarios = await apiFetch<UsuarioItem[]>('/api/usuarios');
		} catch (err: any) {
			errorMsg = err.message || 'Erro ao carregar usuários';
		} finally {
			loading = false;
		}
	}

	function abrirCriar() {
		formNome = '';
		formCor = paletaCores[Math.floor(Math.random() * paletaCores.length)];
		formPin = '';
		formPapel = 'usuario';
		modalCriarAberto = true;
	}

	async function salvarCriar() {
		if (!formNome || !formPin) {
			alert('Preencha nome e PIN.');
			return;
		}
		try {
			await apiFetch('/api/usuarios', {
				method: 'POST',
				body: JSON.stringify({
					nome: formNome,
					cor: formCor,
					pin: formPin,
					papel: formPapel
				})
			});
			modalCriarAberto = false;
			successMsg = 'Usuário cadastrado com sucesso!';
			setTimeout(() => successMsg = null, 4000);
			carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao criar usuário');
		}
	}

	function abrirEditar(u: UsuarioItem) {
		usuarioSelecionado = u;
		editNome = u.nome;
		editCor = u.cor;
		editPapel = u.papel;
		editAtivo = u.ativo;
		modalEditarAberto = true;
	}

	async function salvarEditar() {
		if (!usuarioSelecionado) return;
		try {
			await apiFetch(`/api/usuarios/${usuarioSelecionado.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					nome: editNome,
					cor: editCor,
					papel: editPapel,
					ativo: editAtivo
				})
			});
			modalEditarAberto = false;
			successMsg = 'Usuário atualizado com sucesso!';
			setTimeout(() => successMsg = null, 4000);
			carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao atualizar usuário');
		}
	}

	function abrirRedefinirPin(u: UsuarioItem) {
		usuarioSelecionado = u;
		novoPin = '';
		modalPinAberto = true;
	}

	async function salvarPin() {
		if (!usuarioSelecionado || !novoPin) return;
		try {
			await apiFetch(`/api/usuarios/${usuarioSelecionado.id}/pin`, {
				method: 'POST',
				body: JSON.stringify({ novo_pin: novoPin })
			});
			modalPinAberto = false;
			successMsg = `PIN de ${usuarioSelecionado.nome} redefinido com sucesso!`;
			setTimeout(() => successMsg = null, 4000);
			carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao redefinir PIN');
		}
	}

	async function desbloquear(u: UsuarioItem) {
		if (!confirm(`Deseja desbloquear o acesso de ${u.nome}?`)) return;
		try {
			await apiFetch(`/api/usuarios/${u.id}/desbloquear`, { method: 'POST' });
			successMsg = `Usuário ${u.nome} desbloqueado!`;
			setTimeout(() => successMsg = null, 4000);
			carregar();
		} catch (err: any) {
			alert(err.message || 'Erro ao desbloquear usuário');
		}
	}

	onMount(() => {
		if (auth.user?.papel === 'admin') {
			carregar();
		}
	});

	function getIniciais(nome: string): string {
		const p = nome.trim().split(/\s+/);
		if (p.length === 1) return p[0].slice(0, 2).toUpperCase();
		return (p[0][0] + p[p.length - 1][0]).toUpperCase();
	}
</script>

{#if auth.user?.papel !== 'admin'}
	<div class="bg-red-50 border border-red-200 text-red-700 p-8 rounded-2xl text-center max-w-lg mx-auto my-12">
		<ShieldAlert class="w-12 h-12 mx-auto mb-3 text-red-500" />
		<h2 class="text-xl font-bold">Acesso Restrito</h2>
		<p class="text-sm mt-1 text-red-600">Apenas administradores podem acessar a gestão de operadores.</p>
	</div>
{:else}
	<div class="space-y-6">
		<div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
			<div>
				<h1 class="text-2xl font-bold text-slate-800">Gestão de Usuários</h1>
				<p class="text-sm text-slate-500">Cadastre operadores, redefina PINs e gerencie acessos ao terminal</p>
			</div>
			<button
				onclick={abrirCriar}
				class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-semibold text-sm shadow-sm transition active:scale-95 cursor-pointer"
			>
				<UserPlus class="w-4 h-4" />
				<span>Novo Usuário</span>
			</button>
		</div>

		{#if successMsg}
			<div class="bg-emerald-50 border border-emerald-200 text-emerald-800 p-3.5 rounded-xl text-sm flex items-center gap-2">
				<Check class="w-4 h-4 text-emerald-600" />
				<span>{successMsg}</span>
			</div>
		{/if}

		{#if errorMsg}
			<div class="bg-red-50 border border-red-200 text-red-700 p-3.5 rounded-xl text-sm flex items-center gap-2">
				<AlertCircle class="w-4 h-4 text-red-600" />
				<span>{errorMsg}</span>
			</div>
		{/if}

		{#if loading}
			<div class="flex justify-center py-16">
				<div class="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin"></div>
			</div>
		{:else}
			<div class="bg-white rounded-2xl border border-slate-200 overflow-hidden shadow-xs">
				<div class="overflow-x-auto">
					<table class="w-full text-left border-collapse text-sm">
						<thead>
							<tr class="bg-slate-50/75 border-b border-slate-200 text-slate-600 font-semibold">
								<th class="py-3 px-4">Operador</th>
								<th class="py-3 px-4">Papel</th>
								<th class="py-3 px-4">Status</th>
								<th class="py-3 px-4">Falhas / Bloqueio</th>
								<th class="py-3 px-4 text-right">Ações</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-slate-100">
							{#each usuarios as u (u.id)}
								<tr class="hover:bg-slate-50/50 transition">
									<td class="py-3.5 px-4 flex items-center gap-3">
										<div
											class="w-10 h-10 rounded-full flex items-center justify-center text-white font-bold text-xs shadow-xs"
											style="background-color: {u.cor || '#2563EB'};"
										>
											{getIniciais(u.nome)}
										</div>
										<div>
											<div class="font-semibold text-slate-800">{u.nome}</div>
											<div class="text-xs text-slate-400">ID: {u.id.slice(0, 8)}...</div>
										</div>
									</td>
									<td class="py-3.5 px-4">
										{#if u.papel === 'admin'}
											<span class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-purple-100 text-purple-700">
												<ShieldCheck class="w-3 h-3" /> Admin
											</span>
										{:else}
											<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-700">
												Operador
											</span>
										{/if}
									</td>
									<td class="py-3.5 px-4">
										{#if u.ativo}
											<span class="inline-flex items-center gap-1.5 text-emerald-600 font-medium text-xs">
												<span class="w-2 h-2 rounded-full bg-emerald-500"></span> Ativo
											</span>
										{:else}
											<span class="inline-flex items-center gap-1.5 text-slate-400 font-medium text-xs">
												<span class="w-2 h-2 rounded-full bg-slate-300"></span> Inativo
											</span>
										{/if}
									</td>
									<td class="py-3.5 px-4">
										{#if u.bloqueado_ate}
											<span class="inline-flex items-center gap-1 text-xs font-semibold text-red-600 bg-red-50 px-2 py-0.5 rounded-md">
												<AlertCircle class="w-3 h-3" /> Bloqueado
											</span>
										{:else if u.tentativas_falhas > 0}
											<span class="text-xs text-amber-600 font-medium">
												{u.tentativas_falhas} erro(s)
											</span>
										{:else}
											<span class="text-xs text-slate-400">Regular</span>
										{/if}
									</td>
									<td class="py-3.5 px-4 text-right space-x-1">
										{#if u.bloqueado_ate || u.tentativas_falhas > 0}
											<button
												onclick={() => desbloquear(u)}
												class="p-2 text-emerald-600 hover:bg-emerald-50 rounded-lg transition"
												title="Desbloquear operador"
											>
												<Unlock class="w-4 h-4" />
											</button>
										{/if}
										<button
											onclick={() => abrirRedefinirPin(u)}
											class="p-2 text-amber-600 hover:bg-amber-50 rounded-lg transition"
											title="Redefinir PIN"
										>
											<KeyRound class="w-4 h-4" />
										</button>
										<button
											onclick={() => abrirEditar(u)}
											class="p-2 text-blue-600 hover:bg-blue-50 rounded-lg transition"
											title="Editar cadastro"
										>
											<Edit3 class="w-4 h-4" />
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	</div>

	<!-- Modal Novo Usuário -->
	{#if modalCriarAberto}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
				<div class="flex items-center justify-between border-b border-slate-100 pb-3">
					<h3 class="font-bold text-slate-800 text-lg">Novo Operador</h3>
					<button onclick={() => modalCriarAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="space-y-3">
					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Nome completo</label>
						<input 
							type="text" 
							bind:value={formNome} 
							placeholder="Ex: Carlos Silva"
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
						/>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">PIN Inicial (4 a 6 dígitos)</label>
						<input 
							type="password" 
							bind:value={formPin} 
							maxlength="6"
							placeholder="••••"
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500 font-mono tracking-widest" 
						/>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Papel</label>
						<select 
							bind:value={formPapel}
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500"
						>
							<option value="usuario">Operador Padrão</option>
							<option value="admin">Administrador do Sistema</option>
						</select>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1.5">Cor do Avatar e Calendário</label>
						<div class="flex flex-wrap gap-2">
							{#each paletaCores as c}
								<button
									type="button"
									onclick={() => formCor = c}
									class="w-8 h-8 rounded-full border-2 transition cursor-pointer flex items-center justify-center text-white"
									style="background-color: {c}; border-color: {formCor === c ? '#0f172a' : 'transparent'};"
								>
									{#if formCor === c}
										<Check class="w-4 h-4" />
									{/if}
								</button>
							{/each}
						</div>
					</div>
				</div>

				<div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
					<button 
						onclick={() => modalCriarAberto = false}
						class="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-xl font-medium"
					>
						Cancelar
					</button>
					<button 
						onclick={salvarCriar}
						class="px-5 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs"
					>
						Salvar Operador
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal Editar Usuário -->
	{#if modalEditarAberto && usuarioSelecionado}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-2xl max-w-md w-full p-6 shadow-xl space-y-4">
				<div class="flex items-center justify-between border-b border-slate-100 pb-3">
					<h3 class="font-bold text-slate-800 text-lg">Editar Operador</h3>
					<button onclick={() => modalEditarAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<div class="space-y-3">
					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Nome completo</label>
						<input 
							type="text" 
							bind:value={editNome} 
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500" 
						/>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1">Papel</label>
						<select 
							bind:value={editPapel}
							class="w-full px-3 py-2 border border-slate-200 rounded-xl text-sm focus:outline-blue-500"
						>
							<option value="usuario">Operador Padrão</option>
							<option value="admin">Administrador do Sistema</option>
						</select>
					</div>

					<div class="flex items-center gap-2 pt-1">
						<input 
							type="checkbox" 
							id="editAtivo"
							bind:checked={editAtivo}
							class="w-4 h-4 rounded text-blue-600"
						/>
						<label for="editAtivo" class="text-xs font-semibold text-slate-700 cursor-pointer">
							Usuário Ativo (desmarcar encerra as sessões e desativa o login)
						</label>
					</div>

					<div>
						<label class="block text-xs font-semibold text-slate-700 mb-1.5">Cor</label>
						<div class="flex flex-wrap gap-2">
							{#each paletaCores as c}
								<button
									type="button"
									onclick={() => editCor = c}
									class="w-8 h-8 rounded-full border-2 transition cursor-pointer flex items-center justify-center text-white"
									style="background-color: {c}; border-color: {editCor === c ? '#0f172a' : 'transparent'};"
								>
									{#if editCor === c}
										<Check class="w-4 h-4" />
									{/if}
								</button>
							{/each}
						</div>
					</div>
				</div>

				<div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
					<button 
						onclick={() => modalEditarAberto = false}
						class="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-xl font-medium"
					>
						Cancelar
					</button>
					<button 
						onclick={salvarEditar}
						class="px-5 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-xl font-semibold shadow-xs"
					>
						Atualizar
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal Redefinir PIN -->
	{#if modalPinAberto && usuarioSelecionado}
		<div class="fixed inset-0 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-2xl max-w-sm w-full p-6 shadow-xl space-y-4">
				<div class="flex items-center justify-between border-b border-slate-100 pb-3">
					<h3 class="font-bold text-slate-800 text-base">Redefinir PIN</h3>
					<button onclick={() => modalPinAberto = false} class="text-slate-400 hover:text-slate-600">
						<X class="w-5 h-5" />
					</button>
				</div>

				<p class="text-xs text-slate-500">
					Defina um novo PIN numérico de 4 a 6 dígitos para <strong>{usuarioSelecionado.nome}</strong>.
				</p>

				<div>
					<label class="block text-xs font-semibold text-slate-700 mb-1">Novo PIN</label>
					<input 
						type="password" 
						bind:value={novoPin} 
						maxlength="6"
						placeholder="••••"
						class="w-full px-3 py-2 border border-slate-200 rounded-xl text-center text-lg font-mono tracking-widest focus:outline-blue-500" 
					/>
				</div>

				<div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-100">
					<button 
						onclick={() => modalPinAberto = false}
						class="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-xl font-medium"
					>
						Cancelar
					</button>
					<button 
						onclick={salvarPin}
						disabled={novoPin.length < 4}
						class="px-5 py-2 text-sm bg-amber-600 hover:bg-amber-700 text-white rounded-xl font-semibold shadow-xs disabled:opacity-40"
					>
						Salvar PIN
					</button>
				</div>
			</div>
		</div>
	{/if}
{/if}
