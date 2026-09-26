<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { enviarFoto, removerFoto } from '$lib/foto';
	import Avatar from '$lib/components/Avatar.svelte';
	import SetoresAdmin from '$lib/components/SetoresAdmin.svelte';
	import TelasTV from '$lib/components/TelasTV.svelte';
	import { 
		UserPlus, 
		KeyRound, 
		Unlock, 
		Edit3, 
		ShieldAlert, 
		X, 
		Check,
		Camera,
		AlertCircle,
		Building2,
		Tv
	} from 'lucide-svelte';

	type Papel = 'superadmin' | 'admin' | 'usuario';

	interface SetorOpcao {
		id: string;
		nome: string;
	}

	interface UsuarioItem {
		id: string;
		nome: string;
		cor: string;
		papel: Papel;
		ativo: boolean;
		tentativas_falhas: number;
		bloqueado_ate: string | null;
		criado_em: string;
		foto_versao: number | null;
		setor_id: string;
		setor_nome?: string;
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
	let modalSetoresAberto = $state(false);
	let telasAberto = $state(false);

	// Superadmin vê todos os setores; admin só o próprio
	let setores = $state<SetorOpcao[]>([]);
	let filtroSetor = $state('');
	const visiveis = $derived(filtroSetor ? usuarios.filter((u) => u.setor_id === filtroSetor) : usuarios);

	// Formulário Criar
	let formNome = $state('');
	let formCor = $state('#1F5C5A');
	let formPin = $state('');
	let formPapel = $state<Papel>('usuario');
	let formSetor = $state('');
	let formFoto = $state<File | null>(null);
	let formFotoPreview = $state<string | null>(null);

	// Formulário Editar
	let editNome = $state('');
	let editCor = $state('#1F5C5A');
	let editPapel = $state<Papel>('usuario');
	let editSetor = $state('');
	let editAtivo = $state(true);
	let enviandoFoto = $state(false);

	// Formulário Redefinir PIN
	let novoPin = $state('');

	// Tons sóbrios, todos legíveis com texto branco
	const paletaCores = [
		'#1F5C5A', '#2E7D6B', '#4D7A3A', '#8A6D1E',
		'#B0622B', '#A8433A', '#8E3B6B', '#6B4C9A',
		'#4A5AA8', '#2F5D8A', '#5B6770', '#7A5840'
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

	async function carregarSetores() {
		try {
			setores = await apiFetch<SetorOpcao[]>('/api/setores');
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	function abrirCriar() {
		formNome = '';
		formCor = paletaCores[Math.floor(Math.random() * paletaCores.length)];
		formPin = '';
		formPapel = 'usuario';
		formSetor = filtroSetor || auth.user?.setor.id || '';
		definirFotoNova(null);
		modalCriarAberto = true;
	}

	function definirFotoNova(arquivo: File | null) {
		if (formFotoPreview) URL.revokeObjectURL(formFotoPreview);
		formFoto = arquivo;
		formFotoPreview = arquivo ? URL.createObjectURL(arquivo) : null;
	}

	function arquivoDoInput(e: Event): File | null {
		const input = e.currentTarget as HTMLInputElement;
		const arquivo = input.files?.[0] ?? null;
		input.value = '';
		return arquivo;
	}

	// Foto no modal de edição: envia na hora, sem esperar o "Salvar"
	async function trocarFotoEditar(arquivo: File | null) {
		if (!arquivo || !usuarioSelecionado) return;
		enviandoFoto = true;
		try {
			aplicarFotoVersao(usuarioSelecionado.id, await enviarFoto(usuarioSelecionado.id, arquivo));
		} catch (err: any) {
			alert(err.message || 'Erro ao enviar foto');
		} finally {
			enviandoFoto = false;
		}
	}

	async function removerFotoEditar() {
		if (!usuarioSelecionado) return;
		enviandoFoto = true;
		try {
			await removerFoto(usuarioSelecionado.id);
			aplicarFotoVersao(usuarioSelecionado.id, null);
		} catch (err: any) {
			alert(err.message || 'Erro ao remover foto');
		} finally {
			enviandoFoto = false;
		}
	}

	function aplicarFotoVersao(id: string, versao: number | null) {
		if (usuarioSelecionado?.id === id) usuarioSelecionado.foto_versao = versao;
		const u = usuarios.find((x) => x.id === id);
		if (u) u.foto_versao = versao;
		if (auth.user?.id === id) auth.user.foto_versao = versao;
	}

	async function salvarCriar() {
		if (!formNome || !formPin) {
			alert('Preencha nome e PIN.');
			return;
		}
		try {
			const criado = await apiFetch<UsuarioItem>('/api/usuarios', {
				method: 'POST',
				body: JSON.stringify({
					nome: formNome,
					cor: formCor,
					pin: formPin,
					papel: formPapel,
					setor_id: auth.ehSuperadmin ? formSetor : undefined
				})
			});
			if (formFoto) {
				try {
					await enviarFoto(criado.id, formFoto);
				} catch (err: any) {
					alert(`Operador cadastrado, mas a foto não foi enviada: ${err.message}`);
				}
			}
			definirFotoNova(null);
			modalCriarAberto = false;
			successMsg = 'Operador cadastrado.';
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
		editSetor = u.setor_id;
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
					ativo: editAtivo,
					setor_id: auth.ehSuperadmin ? editSetor : undefined
				})
			});
			modalEditarAberto = false;
			successMsg = 'Alterações salvas.';
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
		if (auth.ehAdmin) {
			carregar();
		}
		if (auth.ehSuperadmin) {
			carregarSetores();
		}
	});
</script>

{#snippet campoFoto(previa: import('svelte').Snippet, temFoto: boolean, ocupado: boolean, escolher: (f: File | null) => void, remover: () => void)}
	<div class="flex items-center gap-4">
		<div class="relative">
			{@render previa()}
			{#if ocupado}
				<span class="absolute inset-0 grid place-items-center rounded-full bg-overlay">
					<span class="size-5 rounded-full border-2 border-white/40 border-t-white animate-spin"></span>
				</span>
			{/if}
		</div>
		<div class="flex flex-wrap gap-2">
			<label class="btn btn-secondary btn-sm {ocupado ? 'pointer-events-none opacity-45' : ''}">
				<Camera class="size-4" />
				{temFoto ? 'Trocar foto' : 'Adicionar foto'}
				<input
					type="file"
					accept="image/jpeg,image/png,image/webp"
					class="sr-only"
					disabled={ocupado}
					onchange={(e) => escolher(arquivoDoInput(e))}
				/>
			</label>
			{#if temFoto}
				<button type="button" onclick={remover} disabled={ocupado} class="btn btn-sm btn-danger">Remover</button>
			{/if}
		</div>
	</div>
{/snippet}

{#snippet opcoesPapel()}
	<option value="usuario">Operador</option>
	<option value="admin">Administrador do setor</option>
	{#if auth.ehSuperadmin}
		<option value="superadmin">Superadmin (todos os setores)</option>
	{/if}
{/snippet}

{#snippet campoSetor(id: string, valor: string, escolher: (v: string) => void)}
	{#if auth.ehSuperadmin}
		<div>
			<label class="label" for={id}>Setor</label>
			<select {id} value={valor} onchange={(e) => escolher(e.currentTarget.value)} class="field">
				{#each setores as s (s.id)}
					<option value={s.id}>{s.nome}</option>
				{/each}
			</select>
		</div>
	{/if}
{/snippet}

{#snippet seletorCor(atual: string, escolher: (c: string) => void)}
	<div class="flex flex-wrap gap-2" role="radiogroup" aria-label="Cor">
		{#each paletaCores as c}
			<button
				type="button"
				onclick={() => escolher(c)}
				role="radio"
				aria-checked={atual === c}
				aria-label={c}
				class="size-8 rounded-full cursor-pointer flex items-center justify-center text-white transition-transform hover:scale-110 {atual === c ? 'ring-2 ring-offset-2 ring-ink ring-offset-surface' : ''}"
				style="background-color: {c};"
			>
				{#if atual === c}
					<Check class="size-4" strokeWidth={3} />
				{/if}
			</button>
		{/each}
	</div>
{/snippet}

{#if !auth.ehAdmin}
	<div class="panel empty max-w-lg mx-auto mt-12">
		<ShieldAlert class="size-10 text-danger" strokeWidth={1.5} />
		<h2 class="empty-title">Acesso restrito</h2>
		<p class="empty-text">Só administradores podem gerenciar operadores.</p>
		<a href="/" class="btn btn-secondary mt-5">Voltar ao painel</a>
	</div>
{:else}
	<div class="space-y-6">
		<div class="page-head">
			<div>
				<h1 class="page-title">Operadores</h1>
				<p class="page-sub">
					{#if auth.ehSuperadmin}
						Quem pode entrar no terminal, em qual setor, com que papel e com qual PIN.
					{:else}
						Quem do setor {auth.user?.setor.nome} pode entrar no terminal, com que papel e com qual PIN.
					{/if}
				</p>
			</div>
			<div class="flex flex-wrap gap-2">
				{#if auth.ehSuperadmin}
					<select bind:value={filtroSetor} class="field w-auto" aria-label="Filtrar por setor">
						<option value="">Todos os setores</option>
						{#each setores as s (s.id)}
							<option value={s.id}>{s.nome}</option>
						{/each}
					</select>
					<button onclick={() => (modalSetoresAberto = true)} class="btn btn-secondary">
						<Building2 class="size-4" />
						<span>Setores</span>
					</button>
				{/if}
				{#if auth.temModulo('tv')}
					<button onclick={() => (telasAberto = true)} class="btn btn-secondary">
						<Tv class="size-4" />
						<span>Telas de TV</span>
					</button>
				{/if}
				<button onclick={abrirCriar} class="btn btn-primary">
					<UserPlus class="size-4" />
					<span>Cadastrar operador</span>
				</button>
			</div>
		</div>

		{#if successMsg}
			<div class="alert bg-ok-soft text-ok" role="status">
				<Check class="size-4 shrink-0" />
				<span>{successMsg}</span>
			</div>
		{/if}

		{#if errorMsg}
			<div class="alert bg-danger-soft text-danger" role="alert">
				<AlertCircle class="size-4 shrink-0" />
				<span>{errorMsg}</span>
			</div>
		{/if}

		{#if loading}
			<div class="flex justify-center py-16"><div class="spinner"></div></div>
		{:else}
			<div class="panel overflow-hidden">
				<div class="overflow-x-auto">
					<table class="data-table">
						<thead>
							<tr>
								<th>Operador</th>
								{#if auth.ehSuperadmin}
									<th>Setor</th>
								{/if}
								<th>Papel</th>
								<th>Situação</th>
								<th>Acesso</th>
								<th class="text-right"><span class="sr-only">Ações</span></th>
							</tr>
						</thead>
						<tbody>
							{#each visiveis as u (u.id)}
								<tr class={u.ativo ? '' : 'opacity-60'}>
									<td>
										<div class="flex items-center gap-3">
											<Avatar id={u.id} nome={u.nome} cor={u.cor} fotoVersao={u.foto_versao} class="size-9 text-xs" />
											<span class="font-semibold text-ink">{u.nome}</span>
										</div>
									</td>
									{#if auth.ehSuperadmin}
										<td class="text-ink-2">{u.setor_nome}</td>
									{/if}
									<td>
										{#if u.papel === 'superadmin'}
											<span class="tag tag-accent">Superadmin</span>
										{:else if u.papel === 'admin'}
											<span class="tag tag-accent">Administrador</span>
										{:else}
											<span class="text-ink-2">Operador</span>
										{/if}
									</td>
									<td>
										<span class="inline-flex items-center gap-2 {u.ativo ? 'text-ink-2' : 'text-ink-3'}">
											<span class="size-2 rounded-full {u.ativo ? 'bg-ok' : 'bg-line-strong'}"></span>
											{u.ativo ? 'Ativo' : 'Inativo'}
										</span>
									</td>
									<td>
										{#if u.bloqueado_ate}
											<span class="tag tag-danger"><AlertCircle class="size-3" /> Bloqueado</span>
										{:else if u.tentativas_falhas > 0}
											<span class="text-warn font-semibold tabular">
												{u.tentativas_falhas} {u.tentativas_falhas === 1 ? 'tentativa errada' : 'tentativas erradas'}
											</span>
										{:else}
											<span class="text-ink-3">Normal</span>
										{/if}
									</td>
									<td class="text-right whitespace-nowrap">
										<div class="inline-flex items-center gap-0.5">
											{#if u.bloqueado_ate || u.tentativas_falhas > 0}
												<button onclick={() => desbloquear(u)} class="btn btn-sm btn-soft mr-1" title="Zerar tentativas e desbloquear">
													<Unlock class="size-3.5" />
													Desbloquear
												</button>
											{/if}
											<button onclick={() => abrirRedefinirPin(u)} class="icon-btn" title="Redefinir PIN" aria-label="Redefinir PIN">
												<KeyRound class="size-4" />
											</button>
											<button onclick={() => abrirEditar(u)} class="icon-btn" title="Editar cadastro" aria-label="Editar cadastro">
												<Edit3 class="size-4" />
											</button>
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	</div>

	{#if modalSetoresAberto}
		<!-- Os módulos do setor atual podem ter mudado: o menu acompanha -->
		<SetoresAdmin
			onfechar={() => (modalSetoresAberto = false)}
			onalterado={() => {
				carregarSetores();
				carregar();
				auth.recarregarPerfil();
			}}
		/>
	{/if}

	{#if telasAberto}
		<TelasTV onfechar={() => (telasAberto = false)} />
	{/if}

	<!-- Modal novo operador -->
	{#if modalCriarAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">Cadastrar operador</h3>
					<button onclick={() => { definirFotoNova(null); modalCriarAberto = false; }} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					{#snippet previaNova()}
						{#if formFotoPreview}
							<img src={formFotoPreview} alt="" class="size-16 rounded-full object-cover" />
						{:else}
							<Avatar id="" nome={formNome || '?'} cor={formCor} class="size-16 text-lg" />
						{/if}
					{/snippet}
					{@render campoFoto(previaNova, !!formFoto, false, definirFotoNova, () => definirFotoNova(null))}

					<div>
						<label class="label" for="u-nome">Nome</label>
						<input id="u-nome" type="text" bind:value={formNome} placeholder="Como aparece no terminal" class="field" />
					</div>

					<div class="grid grid-cols-2 gap-4">
						<div>
							<label class="label" for="u-pin">PIN inicial</label>
							<input
								id="u-pin"
								type="password"
								inputmode="numeric"
								bind:value={formPin}
								maxlength="4"
								placeholder="4 dígitos"
								class="field tracking-[0.3em] placeholder:tracking-normal"
							/>
						</div>
						<div>
							<label class="label" for="u-papel">Papel</label>
							<select id="u-papel" bind:value={formPapel} class="field">
								{@render opcoesPapel()}
							</select>
						</div>
					</div>

					{@render campoSetor('u-setor', formSetor, (v) => (formSetor = v))}

					<div>
						<span class="label">Cor no terminal e no calendário</span>
						{@render seletorCor(formCor, (c) => formCor = c)}
					</div>
				</div>

				<div class="modal-foot">
					<button onclick={() => { definirFotoNova(null); modalCriarAberto = false; }} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarCriar} class="btn btn-primary">Cadastrar</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal editar operador -->
	{#if modalEditarAberto && usuarioSelecionado}
		<div class="modal-backdrop">
			<div class="modal max-w-md" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">Editar operador</h3>
					<button onclick={() => modalEditarAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					{#snippet previaEditar()}
						<Avatar
							id={usuarioSelecionado!.id}
							nome={editNome || usuarioSelecionado!.nome}
							cor={editCor}
							fotoVersao={usuarioSelecionado!.foto_versao}
							class="size-16 text-lg"
						/>
					{/snippet}
					{@render campoFoto(previaEditar, !!usuarioSelecionado.foto_versao, enviandoFoto, trocarFotoEditar, removerFotoEditar)}

					<div>
						<label class="label" for="e-nome">Nome</label>
						<input id="e-nome" type="text" bind:value={editNome} class="field" />
					</div>

					<div>
						<label class="label" for="e-papel">Papel</label>
						<select id="e-papel" bind:value={editPapel} class="field">
							{@render opcoesPapel()}
						</select>
					</div>

					{@render campoSetor('e-setor', editSetor, (v) => (editSetor = v))}

					<div>
						<span class="label">Cor</span>
						{@render seletorCor(editCor, (c) => editCor = c)}
					</div>

					<label class="flex items-start gap-2.5 text-sm text-ink cursor-pointer pt-1">
						<input type="checkbox" bind:checked={editAtivo} class="check mt-0.5" />
						<span>
							Ativo
							<span class="block text-[13px] text-ink-3">Desmarcar encerra as sessões e impede o login.</span>
						</span>
					</label>
				</div>

				<div class="modal-foot">
					<button onclick={() => modalEditarAberto = false} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarEditar} class="btn btn-primary">Salvar alterações</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Modal redefinir PIN -->
	{#if modalPinAberto && usuarioSelecionado}
		<div class="modal-backdrop">
			<div class="modal max-w-sm" role="dialog" aria-modal="true">
				<div class="modal-head">
					<div>
						<h3 class="modal-title">Redefinir PIN</h3>
						<p class="mt-0.5 text-sm text-ink-3">{usuarioSelecionado.nome}</p>
					</div>
					<button onclick={() => modalPinAberto = false} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div>
						<label class="label" for="p-novo">Novo PIN</label>
						<input
							id="p-novo"
							type="password"
							inputmode="numeric"
							bind:value={novoPin}
							maxlength="4"
							placeholder="4 dígitos"
							class="field h-12 text-center text-xl tracking-[0.4em] placeholder:text-sm placeholder:tracking-normal"
						/>
					</div>
				</div>

				<div class="modal-foot">
					<button onclick={() => modalPinAberto = false} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvarPin} disabled={!/^[0-9]{4}$/.test(novoPin)} class="btn btn-primary">Salvar PIN</button>
				</div>
			</div>
		</div>
	{/if}
{/if}
