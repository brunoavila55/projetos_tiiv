// Escala de plantão: tipos e utilitários compartilhados pelo módulo, painel e modo TV.
// Quem fica escalado é só um nome (não precisa ser usuário do sistema).

export type TipoPlantao = 'plantao' | 'sobreaviso';

export interface Turno {
	id: string;
	nome: string;
	tipo: TipoPlantao;
	inicio: string; // AAAA-MM-DD
	fim: string; // AAAA-MM-DD, inclusivo
	observacao: string;
	// Folga opcional da pessoa, marcada junto com o turno (dias inclusivos)
	folga_inicio: string | null;
	folga_fim: string | null;
}

export const TIPO_PLANTAO: Record<TipoPlantao, string> = { plantao: 'Plantão', sobreaviso: 'Sobreaviso' };

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
