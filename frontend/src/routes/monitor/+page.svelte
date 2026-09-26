<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { auth } from '$lib/auth.svelte';
	import { toast } from '$lib/toast.svelte';
	import { monitoresStore } from '$lib/monitores.svelte';
	import TelasTV from '$lib/components/TelasTV.svelte';
	import { Activity, Plus, Pencil, Trash2, RefreshCw, History, X, Ticket, Tv } from 'lucide-svelte';

	type Tipo = 'ping' | 'tcp' | 'http';
	type Status = 'pendente' | 'online' | 'offline';

	interface Monitor {
		id: string;
		nome: string;
		tipo: Tipo;
		alvo: string;
		intervalo_seg: number;
		abrir_ticket: boolean;
		setor_ticket_id: string;
		ativo: boolean;
		status: Status;
		latencia_ms: number | null;
		ultimo_erro: string;
		verificado_em: string | null;
		status_desde: string;
		disponibilidade_24h: number | null;
	}

	interface Queda {
		id: string;
		inicio: string;
		fim: string | null;
		erro: string;
		ticket_numero: number | null;
	}

	const FUSO = 'America/Sao_Paulo';
	const TIPOS: Record<Tipo, { rotulo: string; exemplo: string; ajuda: string }> = {
		ping: { rotulo: 'Ping', exemplo: '192.168.0.1', ajuda: 'Host ou IP. Responde ao ping = no ar.' },
		tcp: { rotulo: 'Porta TCP', exemplo: '192.168.0.10:3389', ajuda: 'host:porta. Porta aceitando conexão = no ar.' },
		http: { rotulo: 'HTTP', exemplo: 'http://intranet/', ajuda: 'URL completa. Resposta abaixo de 400 = no ar (certificado não é validado).' }
	};
	const INTERVALOS = [
		{ valor: 30, rotulo: '30 segundos' },
		{ valor: 60, rotulo: '1 minuto' },
		{ valor: 120, rotulo: '2 minutos' },
		{ valor: 300, rotulo: '5 minutos' },
		{ valor: 900, rotulo: '15 minutos' }
	];

	const ehAdmin = $derived(auth.ehAdmin);

	let monitores = $state<Monitor[]>([]);
	let loading = $state(true);
	let verificando = $state<string | null>(null);
	let agora = $state(Date.now());

	// Histórico de quedas aberto (um por vez)
	let historicoDe = $state<string | null>(null);
	let quedas = $state<Queda[]>([]);

	const contagem = $derived({
		online: monitores.filter((m) => m.ativo && m.status === 'online').length,
		offline: monitores.filter((m) => m.ativo && m.status === 'offline').length,
		pendente: monitores.filter((m) => m.ativo && m.status === 'pendente').length
	});

	async function carregar() {
		try {
			monitores = await apiFetch<Monitor[]>('/api/monitores', { silent: !loading });
			monitoresStore.offline = monitores.filter((m) => m.ativo && m.status === 'offline').length;
			agora = Date.now();
		} catch (err) {
			console.error('Erro ao listar monitores:', err);
		} finally {
			loading = false;
		}
	}

	// O servidor verifica sozinho; a tela só relê o estado
	onMount(() => {
		carregar();
		const timer = setInterval(carregar, 15_000);
		return () => clearInterval(timer);
	});

	function duracao(desdeIso: string, ate = agora): string {
		const min = Math.max(0, Math.floor((ate - new Date(desdeIso).getTime()) / 60000));
		if (min < 1) return 'menos de 1 min';
		if (min < 60) return `${min} min`;
		const h = Math.floor(min / 60);
		if (h < 24) return `${h} h ${min % 60 ? `${min % 60} min` : ''}`.trim();
		const d = Math.floor(h / 24);
		return `${d} ${d === 1 ? 'dia' : 'dias'}${h % 24 ? ` ${h % 24} h` : ''}`;
	}

	function dataHora(iso: string): string {
		return new Date(iso).toLocaleString('pt-BR', { timeZone: FUSO, dateStyle: 'short', timeStyle: 'short' });
	}

	function descricaoStatus(m: Monitor): string {
		if (!m.ativo) return 'Desligado';
		if (m.status === 'pendente') return 'Aguardando verificação';
		return `${m.status === 'online' ? 'No ar' : 'Fora do ar'} há ${duracao(m.status_desde)}`;
	}

	function formatarPct(p: number): string {
		return p.toLocaleString('pt-BR', { maximumFractionDigits: p >= 99.9 && p < 100 ? 2 : 1 }) + '%';
	}

	async function verificarAgora(m: Monitor) {
		verificando = m.id;
		try {
			const atual = await apiFetch<Monitor>(`/api/monitores/${m.id}/verificar`, { method: 'POST' });
			monitores = monitores.map((x) => (x.id === atual.id ? atual : x));
			monitoresStore.atualizar();
			if (atual.status === 'online') toast.success(`${atual.nome} respondeu em ${atual.latencia_ms} ms`);
			else toast.error(`${atual.nome}: ${atual.ultimo_erro || 'sem resposta'}`);
			if (historicoDe === m.id) await abrirHistorico(m, true);
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			verificando = null;
		}
	}

	async function abrirHistorico(m: Monitor, recarregar = false) {
		if (historicoDe === m.id && !recarregar) {
			historicoDe = null;
			return;
		}
		try {
			quedas = await apiFetch<Queda[]>(`/api/monitores/${m.id}/quedas`);
			historicoDe = m.id;
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	// ---------- Cadastro (admin) ----------
	let modalAberto = $state(false);
	let telasAberto = $state(false);
	let salvando = $state(false);
	let editId = $state<string | null>(null);
	let fNome = $state('');
	let fTipo = $state<Tipo>('ping');
	let fAlvo = $state('');
	let fIntervalo = $state(60);
	let fTicket = $state(false);
	let fSetorTicket = $state('');
	let fAtivo = $state(true);

	// Monitores são de todos; o ticket de queda cai na fila de um setor
	let setores = $state<{ id: string; nome: string }[]>([]);
	async function carregarSetores() {
		if (setores.length) return;
		try {
			setores = await apiFetch<{ id: string; nome: string }[]>('/api/setores');
		} catch {
			// apiFetch já mostrou o erro
		}
	}

	function abrirNovo() {
		editId = null;
		fNome = '';
		fTipo = 'ping';
		fAlvo = '';
		fIntervalo = 60;
		fTicket = false;
		fSetorTicket = auth.user?.setor.id ?? '';
		fAtivo = true;
		carregarSetores();
		modalAberto = true;
	}

	function abrirEditar(m: Monitor) {
		editId = m.id;
		fNome = m.nome;
		fTipo = m.tipo;
		fAlvo = m.alvo;
		fIntervalo = m.intervalo_seg;
		fTicket = m.abrir_ticket;
		fSetorTicket = m.setor_ticket_id;
		fAtivo = m.ativo;
		carregarSetores();
		modalAberto = true;
	}

	async function salvar() {
		if (!fNome.trim() || !fAlvo.trim()) {
			alert('Nome e alvo são obrigatórios.');
			return;
		}
		salvando = true;
		try {
			const payload = {
				nome: fNome.trim(),
				tipo: fTipo,
				alvo: fAlvo.trim(),
				intervalo_seg: fIntervalo,
				abrir_ticket: fTicket,
				setor_ticket_id: fSetorTicket,
				ativo: fAtivo
			};
			if (editId) {
				await apiFetch(`/api/monitores/${editId}`, { method: 'PUT', body: JSON.stringify(payload) });
				modalAberto = false;
				await carregar();
			} else {
				const novo = await apiFetch<Monitor>('/api/monitores', { method: 'POST', body: JSON.stringify(payload) });
				modalAberto = false;
				await carregar();
				// Primeira verificação na hora, sem esperar o ciclo do servidor
				verificarAgora(novo);
			}
		} catch {
			// apiFetch já mostrou o erro
		} finally {
			salvando = false;
		}
	}

	async function excluir(m: Monitor) {
		if (!confirm(`Excluir o monitor "${m.nome}"? O histórico de quedas vai junto.`)) return;
		try {
			await apiFetch(`/api/monitores/${m.id}`, { method: 'DELETE' });
			if (historicoDe === m.id) historicoDe = null;
			await carregar();
			monitoresStore.atualizar();
		} catch {
			// apiFetch já mostrou o erro
		}
	}
</script>

<div class="space-y-6">
	<div class="page-head">
		<div>
			<h1 class="page-title">Monitor</h1>
			<p class="page-sub">Disponibilidade de hosts e serviços da rede, verificada pelo servidor. Duas falhas seguidas marcam o alvo como fora do ar.</p>
		</div>
		<div class="flex flex-wrap gap-2">
			{#if auth.temModulo('tv')}
				<a href="/tv" class="btn btn-secondary">
					<Tv class="size-4" />
					<span>Modo TV</span>
				</a>
			{/if}
			{#if ehAdmin && auth.temModulo('tv')}
				<button onclick={() => (telasAberto = true)} class="btn btn-secondary">Telas de TV</button>
				<button onclick={abrirNovo} class="btn btn-primary">
					<Plus class="size-4" />
					<span>Novo monitor</span>
				</button>
			{/if}
		</div>
	</div>

	{#if loading}
		<div class="flex justify-center py-20"><div class="spinner"></div></div>
	{:else if monitores.length === 0}
		<div class="panel empty">
			<Activity class="size-9 text-ink-3" strokeWidth={1.5} />
			<h3 class="empty-title">Nenhum monitor cadastrado</h3>
			<p class="empty-text">
				{ehAdmin
					? 'Cadastre roteadores, servidores e sistemas para acompanhar se estão no ar.'
					: 'Peça a um administrador para cadastrar os hosts e serviços da rede.'}
			</p>
		</div>
	{:else}
		<div class="flex flex-wrap gap-2 text-sm" aria-live="polite">
			<span class="tag tag-ok h-7 px-2.5 text-[13px]"><span class="size-2 rounded-full bg-ok"></span>{contagem.online} no ar</span>
			<span class="tag h-7 px-2.5 text-[13px] {contagem.offline ? 'tag-danger' : ''}">
				<span class="size-2 rounded-full {contagem.offline ? 'bg-danger' : 'bg-line-strong'}"></span>{contagem.offline} fora do ar
			</span>
			{#if contagem.pendente}
				<span class="tag h-7 px-2.5 text-[13px]"><span class="size-2 rounded-full bg-line-strong"></span>{contagem.pendente} aguardando</span>
			{/if}
		</div>

		<ul class="panel divide-y divide-line">
			{#each monitores as m (m.id)}
				{@const offline = m.ativo && m.status === 'offline'}
				<li class={!m.ativo ? 'opacity-60' : ''}>
					<div class="flex flex-col sm:flex-row sm:items-center gap-3 px-4 sm:px-5 py-3.5 {offline ? 'bg-danger-soft/60' : ''}">
						<div class="flex items-start gap-3 min-w-0 flex-1">
							<span
								class="mt-1.5 size-2.5 shrink-0 rounded-full {!m.ativo || m.status === 'pendente'
									? 'bg-line-strong'
									: m.status === 'online'
										? 'bg-ok'
										: 'bg-danger animate-pulse'}"
								aria-hidden="true"
							></span>
							<div class="min-w-0 flex-1">
								<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
									<span class="font-semibold text-ink leading-snug">{m.nome}</span>
									<span class="tag">{TIPOS[m.tipo].rotulo}</span>
									{#if m.abrir_ticket}
										<span class="tag" title="Abre ticket quando cai"><Ticket class="size-3" />ticket</span>
									{/if}
								</div>
								<div class="text-[13px] text-ink-3 font-mono break-all mt-0.5">{m.alvo}</div>
								<div class="flex flex-wrap gap-x-4 gap-y-0.5 text-[13px] mt-1">
									<span class={offline ? 'text-danger font-semibold' : 'text-ink-2'}>{descricaoStatus(m)}</span>
									{#if offline && m.ultimo_erro}
										<span class="text-danger">{m.ultimo_erro}</span>
									{/if}
								</div>
							</div>
						</div>

						<div class="flex items-center gap-5 sm:gap-6 pl-5.5 sm:pl-0 shrink-0">
							<div class="text-right tabular">
								<div class="text-[15px] font-bold text-ink">{m.ativo && m.status === 'online' && m.latencia_ms != null ? `${m.latencia_ms} ms` : '—'}</div>
								<div class="text-xs text-ink-3">latência</div>
							</div>
							<div class="text-right tabular" title="Tempo no ar nas últimas 24 horas (ou desde o cadastro)">
								<div class="text-[15px] font-bold {m.disponibilidade_24h != null && m.disponibilidade_24h < 99 ? 'text-warn' : 'text-ink'}">
									{m.disponibilidade_24h != null ? formatarPct(m.disponibilidade_24h) : '—'}
								</div>
								<div class="text-xs text-ink-3">24 h</div>
							</div>
							<div class="flex gap-1 ml-auto sm:ml-0">
								<button
									onclick={() => verificarAgora(m)}
									disabled={!m.ativo || verificando === m.id}
									class="icon-btn disabled:opacity-40 disabled:cursor-not-allowed"
									title="Verificar agora"
									aria-label="Verificar {m.nome} agora"
								>
									<RefreshCw class="size-4 {verificando === m.id ? 'animate-spin' : ''}" />
								</button>
								<button
									onclick={() => abrirHistorico(m)}
									class="icon-btn {historicoDe === m.id ? 'bg-muted text-ink' : ''}"
									title="Histórico de quedas"
									aria-label="Histórico de quedas de {m.nome}"
									aria-expanded={historicoDe === m.id}
								>
									<History class="size-4" />
								</button>
								{#if ehAdmin}
									<button onclick={() => abrirEditar(m)} class="icon-btn" title="Editar" aria-label="Editar {m.nome}">
										<Pencil class="size-4" />
									</button>
									<button onclick={() => excluir(m)} class="icon-btn icon-btn-danger" title="Excluir" aria-label="Excluir {m.nome}">
										<Trash2 class="size-4" />
									</button>
								{/if}
							</div>
						</div>
					</div>

					{#if historicoDe === m.id}
						<div class="px-4 sm:px-5 pb-4 pt-1">
							<div class="rounded-lg border border-line bg-sunken">
								<div class="px-4 py-2.5 border-b border-line text-[13px] font-semibold text-ink-2">
									Últimas quedas
									{#if m.verificado_em}<span class="font-normal text-ink-3"> · última verificação {dataHora(m.verificado_em)}</span>{/if}
								</div>
								{#if quedas.length === 0}
									<p class="px-4 py-4 text-sm text-ink-3">Nenhuma queda registrada.</p>
								{:else}
									<ul class="divide-y divide-line text-sm">
										{#each quedas as q (q.id)}
											<li class="flex flex-wrap items-baseline gap-x-4 gap-y-0.5 px-4 py-2">
												<span class="tabular text-ink">{dataHora(q.inicio)}</span>
												<span class="tabular {q.fim ? 'text-ink-2' : 'text-danger font-semibold'}">
													{q.fim ? `durou ${duracao(q.inicio, new Date(q.fim).getTime())}` : `em andamento (${duracao(q.inicio)})`}
												</span>
												{#if q.erro}<span class="text-ink-3">{q.erro}</span>{/if}
												{#if q.ticket_numero}
													<a href="/tickets" class="text-[13px] font-semibold text-accent hover:underline">ticket #{q.ticket_numero}</a>
												{/if}
											</li>
										{/each}
									</ul>
								{/if}
							</div>
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}

	{#if modalAberto}
		<div class="modal-backdrop">
			<div class="modal max-w-lg" role="dialog" aria-modal="true">
				<div class="modal-head">
					<h3 class="modal-title">{editId ? 'Editar monitor' : 'Novo monitor'}</h3>
					<button onclick={() => (modalAberto = false)} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
						<X class="size-5" />
					</button>
				</div>

				<div class="modal-body">
					<div>
						<label class="label" for="mo-nome">Nome</label>
						<input id="mo-nome" type="text" bind:value={fNome} maxlength="120" placeholder="Ex.: Roteador da recepção" class="field" autofocus />
					</div>

					<div class="grid grid-cols-1 sm:grid-cols-[10rem_1fr] gap-4">
						<div>
							<label class="label" for="mo-tipo">Tipo</label>
							<select id="mo-tipo" bind:value={fTipo} class="field">
								{#each Object.entries(TIPOS) as [valor, t]}
									<option value={valor}>{t.rotulo}</option>
								{/each}
							</select>
						</div>
						<div>
							<label class="label" for="mo-alvo">Alvo</label>
							<input id="mo-alvo" type="text" bind:value={fAlvo} maxlength="500" placeholder={TIPOS[fTipo].exemplo} class="field font-mono" autocapitalize="off" spellcheck="false" />
						</div>
					</div>
					<p class="hint -mt-2">{TIPOS[fTipo].ajuda}</p>

					<div>
						<label class="label" for="mo-intervalo">Verificar a cada</label>
						<select id="mo-intervalo" bind:value={fIntervalo} class="field">
							{#each INTERVALOS as i}
								<option value={i.valor}>{i.rotulo}</option>
							{/each}
						</select>
					</div>

					<label class="flex items-start gap-2.5 text-sm text-ink cursor-pointer">
						<input type="checkbox" bind:checked={fTicket} class="check mt-0.5" />
						<span>
							Abrir ticket quando cair
							<span class="block text-xs text-ink-3">Um ticket urgente entra na fila a cada nova queda.</span>
						</span>
					</label>

					{#if fTicket && setores.length > 1}
						<div>
							<label class="label" for="mo-setor">Fila que recebe o ticket</label>
							<select id="mo-setor" bind:value={fSetorTicket} class="field">
								{#each setores as st (st.id)}
									<option value={st.id}>{st.nome}</option>
								{/each}
							</select>
						</div>
					{/if}

					{#if editId}
						<label class="flex items-start gap-2.5 text-sm text-ink cursor-pointer">
							<input type="checkbox" bind:checked={fAtivo} class="check mt-0.5" />
							<span>
								Monitor ligado
								<span class="block text-xs text-ink-3">Desligado, o servidor para de verificar (útil em manutenção).</span>
							</span>
						</label>
					{/if}
				</div>

				<div class="modal-foot">
					<button onclick={() => (modalAberto = false)} class="btn btn-ghost">Cancelar</button>
					<button onclick={salvar} class="btn btn-primary" disabled={salvando}>
						{editId ? 'Salvar alterações' : 'Cadastrar monitor'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	{#if telasAberto}
		<TelasTV onfechar={() => (telasAberto = false)} />
	{/if}
</div>
