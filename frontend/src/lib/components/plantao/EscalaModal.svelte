<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { toast } from '$lib/toast.svelte';
	import type { UsuarioPublico } from '$lib/auth.svelte';
	import { X, Repeat, ArrowRight, Plus } from 'lucide-svelte';
	import {
		CIDADES,
		TIPO_PLANTAO,
		corDaPessoa,
		mesmaPessoa,
		diaLocal,
		diaMes,
		ehDomingo,
		diaDoInterno,
		diaComFeriado,
		diasFeriado,
		type Feriado,
		paraDia,
		somarDias,
		type Cidade,
		type TipoPlantao,
		type Turno
	} from '$lib/plantao';

	interface Props {
		// Turno em edição; sem ele, cria (avulso ou rodízio)
		turno?: Turno | null;
		// Valores iniciais ao criar (ex.: cobrir um buraco da escala)
		tipo?: TipoPlantao;
		cidade?: Cidade | null;
		inicio?: string;
		fim?: string;
		onfechar: () => void;
		onsalvo: () => void;
	}

	let { turno = null, tipo: tipoInicial, cidade: cidadeInicial, inicio, fim, onfechar, onsalvo }: Props = $props();

	// As props só dão os valores iniciais do formulário (o modal abre de novo a cada uso)
	const hoje = paraDia(new Date());
	const ini = untrack(() => ({
		nome: turno?.nome ?? '',
		tipo: turno?.tipo ?? tipoInicial ?? ('interno' as TipoPlantao),
		cidade: turno?.cidade ?? cidadeInicial ?? CIDADES[0].id,
		inicio: turno?.inicio ?? inicio ?? hoje,
		fim: turno?.fim ?? fim ?? inicio ?? hoje,
		obs: turno?.observacao ?? '',
		folgaInicio: turno?.folga_inicio ?? '',
		folgaFim: turno?.folga_fim ?? ''
	}));
	let modo = $state<'avulso' | 'rodizio'>('avulso');
	let salvando = $state(false);
	let nome = $state(ini.nome);
	let tipo = $state<TipoPlantao>(ini.tipo);
	let cidade = $state<Cidade>(ini.cidade);
	let eInicio = $state(ini.inicio);
	let eFim = $state(ini.fim);
	let obs = $state(ini.obs);
	// Folga opcional, antes do plantão (avulso: datas; rodízio: um dia, tantos
	// dias antes de cada turno; ex.: plantão no domingo, folga na quinta)
	let comFolga = $state(!!ini.folgaInicio);
	let folgaInicio = $state(ini.folgaInicio);
	let folgaFim = $state(ini.folgaFim);
	let folgaDiasAntes = $state(0);
	// Rodízio
	let pessoas = $state<string[]>([]);
	let novaPessoa = $state('');
	let dias = $state(7);
	let turnos = $state(8);

	// Sugestões: quem já esteve na escala e a equipe do setor. Os feriados
	// (de um pouco antes de hoje a pouco mais de 2 anos) guiam o rodízio do interno.
	let sugestoes = $state<string[]>([]);
	let feriados = $state(diasFeriado([]));
	onMount(async () => {
		const de = paraDia(somarDias(new Date(), -60));
		const ate = paraDia(somarDias(new Date(), 760));
		const [escala, equipe, lista] = await Promise.all([
			apiFetch<string[]>('/api/plantoes/pessoas', { silent: true }).catch(() => []),
			apiFetch<UsuarioPublico[]>('/api/equipe', { silent: true }).catch(() => []),
			apiFetch<Feriado[]>(`/api/plantoes/feriados?inicio=${de}&fim=${ate}`, { silent: true }).catch(() => [])
		]);
		feriados = diasFeriado(lista);
		const todos: string[] = [];
		for (const n of [...escala, ...equipe.map((u) => u.nome)]) {
			if (!todos.some((x) => mesmaPessoa(x, n))) todos.push(n);
		}
		sugestoes = todos.sort((a, b) => a.localeCompare(b, 'pt-BR'));
	});

	// Rodízio de domingo e do interno: um dia por turno (um domingo; ou um
	// domingo ou feriado), um depois do outro, começando num dia desses.
	// O noturno vai em turnos seguidos de "dias" dias.
	const diaDoRodizio = $derived.by((): ((dia: string) => boolean) | null => {
		if (tipo === 'domingo') return ehDomingo;
		if (tipo === 'interno') return (d) => diaDoInterno(d, feriados);
		return null;
	});
	const rodizioPorDia = $derived(modo === 'rodizio' && diaDoRodizio !== null);
	const diasTurno = $derived(rodizioPorDia ? 1 : dias);
	const textoDia = $derived(tipo === 'interno' ? 'domingo ou feriado' : 'domingo');
	const inicioForaDoDia = $derived(rodizioPorDia && !!eInicio && !diaDoRodizio!(eInicio));

	function proximoDiaDoRodizio(dia: string): string {
		let d = dia;
		for (let i = 0; i < 400 && diaDoRodizio && !diaDoRodizio(d); i++) d = paraDia(somarDias(diaLocal(d), 1));
		return d;
	}

	// Começo de cada turno do rodízio, igual ao que o servidor gera
	const inicios = $derived.by(() => {
		if (!eInicio || turnos < 1 || dias < 1) return [];
		const res: string[] = [];
		let d = eInicio;
		while (res.length < Math.min(turnos, 104)) {
			if (!rodizioPorDia) {
				res.push(d);
				d = paraDia(somarDias(diaLocal(d), dias));
				continue;
			}
			if (diaDoRodizio!(d)) res.push(d);
			d = paraDia(somarDias(diaLocal(d), 1));
		}
		return res;
	});

	const sugestoesRodizio = $derived(sugestoes.filter((s) => !pessoas.some((p) => mesmaPessoa(p, s))));

	// Prévia dos primeiros turnos do rodízio, igual ao que o servidor gera
	const previa = $derived.by(() => {
		if (pessoas.length === 0) return [];
		return inicios.slice(0, 6).map((dia, i) => {
			const ini = diaLocal(dia);
			const fim = somarDias(ini, diasTurno - 1);
			const folga = folgaDiasAntes > 0 ? somarDias(ini, -folgaDiasAntes) : null;
			return { pessoa: pessoas[i % pessoas.length], inicio: ini, fim, folga };
		});
	});

	// Dias antes do turno que a folga costuma cair (domingo → quinta)
	const FOLGA_PADRAO_DIAS_ANTES = 3;

	// Ao ligar a folga, sugere um dia, alguns dias antes do plantão
	function alternarFolga() {
		comFolga = !comFolga;
		if (comFolga && !folgaInicio) {
			folgaInicio = paraDia(somarDias(diaLocal(eInicio), -FOLGA_PADRAO_DIAS_ANTES));
			folgaFim = folgaInicio;
		}
	}

	// "3 dias antes (qui)": o dia da semana vem do começo do rodízio
	function rotuloFolgaAntes(n: number): string {
		const base = `${n} ${n === 1 ? 'dia' : 'dias'} antes`;
		if (!eInicio) return base;
		const semana = somarDias(diaLocal(eInicio), -n).toLocaleDateString('pt-BR', { weekday: 'short' }).replace('.', '');
		return `${base} (${semana})`;
	}

	function adicionarPessoa(n: string) {
		const limpo = n.trim().replace(/\s+/g, ' ');
		if (!limpo) return;
		if (pessoas.some((p) => mesmaPessoa(p, limpo))) {
			toast.error(`${limpo} já está no rodízio`);
			return;
		}
		pessoas = [...pessoas, limpo];
		novaPessoa = '';
	}

	async function salvar() {
		salvando = true;
		try {
			if (modo === 'rodizio') {
				if (novaPessoa.trim()) adicionarPessoa(novaPessoa);
				if (pessoas.length === 0) {
					toast.error('Escreva quem entra no rodízio.');
					return;
				}
				if (inicioForaDoDia) {
					toast.error(`O rodízio deve começar num ${textoDia}.`);
					return;
				}
				const res = await apiFetch<{ criados: number }>('/api/plantoes/rodizio', {
					method: 'POST',
					body: JSON.stringify({
						pessoas,
						tipo,
						cidade: tipo === 'interno' ? '' : cidade,
						inicio: eInicio,
						dias_por_turno: Number(dias),
						turnos: Number(turnos),
						folga_dias_antes: Number(folgaDiasAntes),
						observacao: obs.trim()
					})
				});
				toast.success(`${res.criados} turnos adicionados à escala`);
			} else {
				if (!nome.trim()) {
					toast.error('Escreva quem fica de plantão.');
					return;
				}
				await apiFetch(turno ? `/api/plantoes/${turno.id}` : '/api/plantoes', {
					method: turno ? 'PUT' : 'POST',
					body: JSON.stringify({
						nome: nome.trim(),
						tipo,
						cidade: tipo === 'interno' ? '' : cidade,
						inicio: eInicio,
						fim: eFim < eInicio ? eInicio : eFim,
						observacao: obs.trim(),
						...(comFolga && folgaInicio
							? { folga_inicio: folgaInicio, folga_fim: folgaFim < folgaInicio ? folgaInicio : folgaFim }
							: {})
					})
				});
			}
			onsalvo();
		} catch {
			// apiFetch já mostrou o erro (ex.: conflito na escala)
		} finally {
			salvando = false;
		}
	}
</script>

<datalist id="plantao-pessoas">
	{#each sugestoes as s (s)}
		<option value={s}></option>
	{/each}
</datalist>

<div class="modal-backdrop">
	<div class="modal max-w-lg" role="dialog" aria-modal="true" aria-labelledby="escala-titulo">
		<div class="modal-head">
			<h3 id="escala-titulo" class="modal-title">{turno ? 'Editar turno' : 'Montar escala'}</h3>
			<button onclick={onfechar} class="icon-btn -mr-1.5 -mt-1" aria-label="Fechar">
				<X class="size-5" />
			</button>
		</div>

		<div class="modal-body">
			{#if !turno}
				<div class="segmented" role="group" aria-label="Modo">
					<button aria-pressed={modo === 'avulso'} onclick={() => (modo = 'avulso')}>Turno avulso</button>
					<button aria-pressed={modo === 'rodizio'} onclick={() => (modo = 'rodizio')}>
						<Repeat class="size-3.5" />
						Rodízio
					</button>
				</div>
			{/if}

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<div class={tipo === 'interno' ? 'sm:col-span-2' : ''}>
					<label class="label" for="es-tipo">Escala</label>
					<select id="es-tipo" bind:value={tipo} class="field">
						{#each Object.entries(TIPO_PLANTAO) as [valor, rotulo] (valor)}
							<option value={valor}>{rotulo}</option>
						{/each}
					</select>
				</div>
				{#if tipo !== 'interno'}
					<div>
						<label class="label" for="es-cidade">Cidade</label>
						<select id="es-cidade" bind:value={cidade} class="field">
							{#each CIDADES as c (c.id)}
								<option value={c.id}>{c.nome}</option>
							{/each}
						</select>
					</div>
				{/if}
			</div>

			{#if modo === 'avulso'}
				<div>
					<label class="label" for="es-nome">Quem</label>
					<input
						id="es-nome"
						type="text"
						bind:value={nome}
						list="plantao-pessoas"
						maxlength="80"
						autocomplete="off"
						placeholder={tipo === 'interno' ? 'Nome de quem fica de plantão' : 'Nome do técnico'}
						class="field"
					/>
				</div>
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
					<div>
						<label class="label" for="es-ini">De</label>
						<input id="es-ini" type="date" bind:value={eInicio} class="field" />
					</div>
					<div>
						<label class="label" for="es-fim">Até <span class="font-normal text-ink-3">(inclusive)</span></label>
						<input id="es-fim" type="date" bind:value={eFim} min={eInicio} class="field" />
					</div>
				</div>
				<div class="rounded-lg border border-line px-3.5 py-3 space-y-3">
					<label class="flex items-center gap-2.5 text-sm font-semibold text-ink cursor-pointer">
						<input type="checkbox" checked={comFolga} onchange={alternarFolga} class="size-4 accent-[var(--accent)]" />
						Marcar folga <span class="font-normal text-ink-3">(opcional, normalmente antes do plantão)</span>
					</label>
					{#if comFolga}
						<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
							<div>
								<label class="label" for="es-fini">Folga de</label>
								<input id="es-fini" type="date" bind:value={folgaInicio} class="field" />
							</div>
							<div>
								<label class="label" for="es-ffim">Até <span class="font-normal text-ink-3">(inclusive)</span></label>
								<input id="es-ffim" type="date" bind:value={folgaFim} min={folgaInicio} class="field" />
							</div>
						</div>
					{/if}
				</div>
			{:else}
				<div>
					<label class="label" for="es-pessoa">Quem entra, na ordem do rodízio</label>
					<form
						class="flex gap-2"
						onsubmit={(e) => {
							e.preventDefault();
							adicionarPessoa(novaPessoa);
						}}
					>
						<input
							id="es-pessoa"
							type="text"
							bind:value={novaPessoa}
							list="plantao-pessoas"
							maxlength="80"
							autocomplete="off"
							placeholder="Escreva um nome e tecle Enter"
							class="field"
						/>
						<button type="submit" class="btn btn-secondary shrink-0" aria-label="Adicionar ao rodízio">
							<Plus class="size-4" />
						</button>
					</form>
					{#if pessoas.length > 0}
						<ol class="mt-2 flex flex-wrap gap-2">
							{#each pessoas as p, i (p)}
								<li class="inline-flex items-center gap-2 h-8 pl-1.5 pr-1 rounded-full text-[13px] font-semibold border bg-accent-soft border-accent/40 text-ink">
									<span class="grid place-items-center size-5 rounded-full text-white text-[11px] font-bold tabular" style="background-color: {corDaPessoa(p)};">{i + 1}</span>
									<span>{p}</span>
									<button
										type="button"
										onclick={() => (pessoas = pessoas.filter((x) => x !== p))}
										class="grid place-items-center size-6 rounded-full text-ink-3 hover:bg-surface hover:text-ink cursor-pointer"
										aria-label="Tirar {p} do rodízio"
									>
										<X class="size-3.5" />
									</button>
								</li>
							{/each}
						</ol>
					{/if}
					{#if sugestoesRodizio.length > 0}
						<div class="mt-2 flex flex-wrap gap-1.5 max-h-24 overflow-y-auto">
							{#each sugestoesRodizio as s (s)}
								<button
									type="button"
									onclick={() => adicionarPessoa(s)}
									class="inline-flex items-center gap-1.5 h-7 px-2.5 rounded-full text-[12px] font-medium border bg-surface border-line-strong text-ink-2 hover:bg-sunken cursor-pointer"
								>
									<Plus class="size-3" />
									{s}
								</button>
							{/each}
						</div>
					{/if}
				</div>
				<div class="grid gap-3 {rodizioPorDia ? 'grid-cols-2' : 'grid-cols-3'}">
					<div>
						<label class="label" for="es-rini">Começa em</label>
						<input id="es-rini" type="date" bind:value={eInicio} class="field" />
					</div>
					{#if !rodizioPorDia}
						<div>
							<label class="label" for="es-rdias">Cada turno</label>
							<select id="es-rdias" bind:value={dias} class="field">
								<option value={1}>1 dia</option>
								<option value={2}>2 dias</option>
								<option value={7}>1 semana</option>
								<option value={14}>2 semanas</option>
							</select>
						</div>
					{/if}
					<div>
						<label class="label" for="es-rturnos">{tipo === 'domingo' && rodizioPorDia ? 'Domingos' : 'Turnos'}</label>
						<input id="es-rturnos" type="number" min="1" max="104" bind:value={turnos} class="field tabular" />
					</div>
				</div>
				{#if inicioForaDoDia}
					<p class="-mt-2 text-[13px] text-warn">
						O rodízio começa num {textoDia}.
						<button type="button" onclick={() => (eInicio = proximoDiaDoRodizio(eInicio))} class="font-semibold underline underline-offset-2 cursor-pointer">
							Usar {diaComFeriado(proximoDiaDoRodizio(eInicio), feriados)}
						</button>
					</p>
				{:else if rodizioPorDia}
					<p class="-mt-2 text-[13px] text-ink-3">
						{tipo === 'interno'
							? 'Cada pessoa fica com um dia, alternando pelos domingos e feriados.'
							: 'Cada pessoa fica com um domingo, alternando semana a semana.'}
					</p>
				{/if}
				<div>
					<label class="label" for="es-rfolga">Folga antes de cada turno <span class="font-normal text-ink-3">(opcional, um dia)</span></label>
					<select id="es-rfolga" bind:value={folgaDiasAntes} class="field">
						<option value={0}>Sem folga</option>
						{#each [1, 2, 3, 4, 5, 6] as n (n)}
							<option value={n}>{rotuloFolgaAntes(n)}</option>
						{/each}
					</select>
				</div>
				{#if previa.length}
					<div class="rounded-lg border border-line bg-sunken px-3.5 py-2.5 text-[13px]">
						<ul class="space-y-0.5">
							{#each previa as t}
								<li class="flex items-center gap-2 tabular">
									<span class="text-ink-3 w-28 shrink-0">{diasTurno === 1 ? diaMes(t.inicio) : `${diaMes(t.inicio)} a ${diaMes(t.fim)}`}</span>
									<ArrowRight class="size-3 text-ink-3" />
									<span class="font-semibold text-ink">{t.pessoa}</span>
									{#if feriados.has(paraDia(t.inicio)) && rodizioPorDia}
										<span class="text-ink-3">· {feriados.get(paraDia(t.inicio))}</span>
									{/if}
									{#if t.folga}
										<span class="text-ink-3">· folga {t.folga.toLocaleDateString('pt-BR', { weekday: 'short' }).replace('.', '')} {diaMes(t.folga)}</span>
									{/if}
								</li>
							{/each}
						</ul>
						{#if turnos > previa.length}
							<p class="mt-1 text-ink-3">e mais {turnos - previa.length} turnos, até {diaMes(somarDias(diaLocal(inicios[inicios.length - 1]), diasTurno - 1))}</p>
						{/if}
					</div>
				{/if}
			{/if}

			<div>
				<label class="label" for="es-obs">Observação <span class="font-normal text-ink-3">(opcional)</span></label>
				<input id="es-obs" type="text" bind:value={obs} maxlength="300" placeholder="Ex.: contato pelo celular da equipe" class="field" />
			</div>
		</div>

		<div class="modal-foot">
			<button onclick={onfechar} class="btn btn-ghost">Cancelar</button>
			<button onclick={salvar} class="btn btn-primary" disabled={salvando}>
				{turno ? 'Salvar alterações' : modo === 'rodizio' ? `Gerar ${turnos || 0} turnos` : 'Adicionar à escala'}
			</button>
		</div>
	</div>
</div>
