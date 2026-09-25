// Escala de plantão: tipos e utilitários compartilhados pelo módulo, painel e modo TV.
// Quem fica escalado é só um nome (não precisa ser usuário do sistema).
// Três escalas: plantão interno (a equipe do setor, aos domingos e feriados)
// e, para os técnicos externos, plantão noturno e plantão de domingo, cada um
// por cidade.

export type TipoPlantao = 'interno' | 'noturno' | 'domingo';
export type Cidade = 'sao_gabriel' | 'bage' | 'passo_fundo';

export interface Turno {
	id: string;
	nome: string;
	tipo: TipoPlantao;
	cidade: Cidade | null; // só no noturno e no de domingo
	inicio: string; // AAAA-MM-DD
	fim: string; // AAAA-MM-DD, inclusivo
	observacao: string;
	// Folga opcional da pessoa, marcada junto com o turno (dias inclusivos)
	folga_inicio: string | null;
	folga_fim: string | null;
}

export const TIPO_PLANTAO: Record<TipoPlantao, string> = {
	interno: 'Plantão interno',
	noturno: 'Plantão noturno',
	domingo: 'Plantão de domingo'
};

// Na ordem em que aparecem na dash e na TV
export const CIDADES: { id: Cidade; nome: string }[] = [
	{ id: 'sao_gabriel', nome: 'São Gabriel' },
	{ id: 'bage', nome: 'Bagé' },
	{ id: 'passo_fundo', nome: 'Passo Fundo' }
];

export function nomeCidade(c: Cidade | null): string {
	return CIDADES.find((x) => x.id === c)?.nome ?? '';
}

// "Plantão interno", "Plantão noturno · Bagé"
export function rotuloEscala(t: { tipo: TipoPlantao; cidade: Cidade | null }): string {
	return t.cidade ? `${TIPO_PLANTAO[t.tipo]} · ${nomeCidade(t.cidade)}` : TIPO_PLANTAO[t.tipo];
}

// Rótulo curto, para listas e o calendário: "interno", "noturno Bagé"
export function rotuloCurto(t: { tipo: TipoPlantao; cidade: Cidade | null }): string {
	return t.cidade ? `${t.tipo} ${nomeCidade(t.cidade)}` : t.tipo;
}

// Feriados que o plantão interno cobre (vêm do servidor: nacionais e do RS)
export interface Feriado {
	dia: string; // AAAA-MM-DD
	nome: string;
}

// Dias de feriado, para consultar rápido
export type DiasFeriado = Map<string, string>;

export function diasFeriado(lista: Feriado[]): DiasFeriado {
	return new Map(lista.map((f) => [f.dia, f.nome]));
}

// O plantão interno é aos domingos e feriados
export function diaDoInterno(dia: string, feriados: DiasFeriado): boolean {
	return ehDomingo(dia) || feriados.has(dia);
}

// O próprio dia, se tiver plantão interno, ou o próximo que tiver
export function proximoDiaInterno(dia: string, feriados: DiasFeriado): string {
	let d = dia;
	while (!diaDoInterno(d, feriados)) d = paraDia(somarDias(diaLocal(d), 1));
	return d;
}

// "dom, 27/09" ou "seg, 12/10 · Nossa Senhora Aparecida"
export function diaComFeriado(dia: string, feriados: DiasFeriado): string {
	const f = feriados.get(dia);
	return f ? `${diaSemana(dia)} · ${f}` : diaSemana(dia);
}

// Uma escala a cobrir: o noturno todo dia, o de domingo aos domingos e o
// interno aos domingos e feriados
export interface Escala {
	tipo: TipoPlantao;
	cidade: Cidade | null;
	rotulo: string;
	precisa: (dia: string, feriados: DiasFeriado) => boolean;
}

export const ESCALAS: Escala[] = [
	{ tipo: 'interno', cidade: null, rotulo: TIPO_PLANTAO.interno, precisa: diaDoInterno },
	...CIDADES.flatMap((c) => [
		{ tipo: 'noturno' as const, cidade: c.id, rotulo: `Noturno · ${c.nome}`, precisa: () => true },
		{ tipo: 'domingo' as const, cidade: c.id, rotulo: `Domingo · ${c.nome}`, precisa: ehDomingo }
	])
];

export function daEscala(t: Turno, tipo: TipoPlantao, cidade: Cidade | null): boolean {
	return t.tipo === tipo && t.cidade === cidade;
}

export function ehDomingo(dia: string): boolean {
	return diaLocal(dia).getDay() === 0;
}

// O próprio dia, se for domingo, ou o domingo seguinte
export function proximoDomingo(dia: string): string {
	const d = diaLocal(dia);
	return paraDia(somarDias(d, (7 - d.getDay()) % 7));
}

// Mesma paleta das cores de operador
const CORES = [
	'#1F5C5A', '#2E7D6B', '#4D7A3A', '#8A6D1E',
	'#B0622B', '#A8433A', '#8E3B6B', '#6B4C9A',
	'#4A5AA8', '#2F5D8A', '#5B6770', '#7A5840'
];

// Cor fixa por nome (sem diferenciar maiúsculas e espaços), para a mesma pessoa
// ter sempre a mesma cor no calendário, na dash e na TV
export function corDaPessoa(nome: string): string {
	const chave = nome.trim().replace(/\s+/g, ' ').toLowerCase();
	let h = 0;
	for (const c of chave) h = (h * 31 + c.codePointAt(0)!) >>> 0;
	return CORES[h % CORES.length];
}

export function mesmaPessoa(a: string, b: string): boolean {
	const n = (s: string) => s.trim().replace(/\s+/g, ' ').toLowerCase();
	return n(a) === n(b);
}

// "AAAA-MM-DD" como meia-noite local (new Date("AAAA-MM-DD") seria UTC)
export function diaLocal(dia: string): Date {
	return new Date(dia + 'T00:00:00');
}

export function paraDia(d: Date): string {
	const pad = (n: number) => n.toString().padStart(2, '0');
	return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function somarDias(d: Date, dias: number): Date {
	const r = new Date(d);
	r.setDate(r.getDate() + dias);
	return r;
}

// Dias entre duas datas AAAA-MM-DD (b - a)
export function diasEntre(a: string, b: string): number {
	return Math.round((diaLocal(b).getTime() - diaLocal(a).getTime()) / 86_400_000);
}

export function diaMes(d: Date): string {
	return d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' });
}

export function diaSemana(dia: string): string {
	return diaLocal(dia).toLocaleDateString('pt-BR', { weekday: 'short', day: '2-digit', month: '2-digit' }).replace('.', '');
}

export function periodo(t: { inicio: string; fim: string }): string {
	return t.inicio === t.fim ? diaSemana(t.inicio) : `${diaSemana(t.inicio)} a ${diaSemana(t.fim)}`;
}

export function cobre(t: { inicio: string; fim: string }, dia: string): boolean {
	return t.inicio <= dia && t.fim >= dia;
}

export function deFolga(t: Turno, dia: string): boolean {
	return !!t.folga_inicio && !!t.folga_fim && t.folga_inicio <= dia && t.folga_fim >= dia;
}

// Período da folga, ou null se o turno não tem folga
export function periodoFolga(t: Turno): string | null {
	return t.folga_inicio && t.folga_fim ? periodo({ inicio: t.folga_inicio, fim: t.folga_fim }) : null;
}

// Quando começa, relativo a hoje: "hoje", "amanhã", "em 3 dias"
export function quandoComeca(inicio: string, hoje: string): string {
	const n = diasEntre(hoje, inicio);
	if (n <= 0) return 'hoje';
	if (n === 1) return 'amanhã';
	return `em ${n} dias`;
}

// Quanto falta para acabar, contando o dia de hoje: "último dia", "mais 2 dias"
export function quantoFalta(fim: string, hoje: string): string {
	const n = diasEntre(hoje, fim);
	if (n <= 0) return 'último dia';
	return n === 1 ? 'até amanhã' : `mais ${n} dias`;
}
