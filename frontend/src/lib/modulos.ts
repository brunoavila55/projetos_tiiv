// Módulos que o superadmin liga e desliga por setor. Mesmas chaves e mesma
// ordem de backend/internal/modulos; painel, operadores e setores são núcleo.
export type Modulo =
	| 'tickets'
	| 'tarefas'
	| 'calendario'
	| 'plantao'
	| 'estoque'
	| 'tecnicos'
	| 'monitor'
	| 'links'
	| 'avisos'
	| 'tira_duvidas'
	| 'tv'
	| 'radio';

export interface ModuloInfo {
	id: Modulo;
	nome: string;
	descricao: string;
	// Telas do módulo (prefixos de rota)
	rotas: string[];
	// Só funciona com estes ligados
	depende?: Modulo[];
}

export const MODULOS: ModuloInfo[] = [
	{ id: 'tickets', nome: 'Tickets', descricao: 'Fila de pedidos abertos pela tela de acesso', rotas: ['/tickets'], depende: ['tarefas'] },
	{ id: 'tarefas', nome: 'Tarefas', descricao: 'Tarefas da equipe com prazo e responsável', rotas: ['/tarefas'] },
	{ id: 'calendario', nome: 'Calendário', descricao: 'Reuniões e marcações', rotas: ['/calendario'] },
	{ id: 'plantao', nome: 'Plantão', descricao: 'Escala de plantão e telão da escala', rotas: ['/plantao'] },
	{ id: 'estoque', nome: 'Estoque', descricao: 'Itens e movimentações de material', rotas: ['/estoque'] },
	{ id: 'tecnicos', nome: 'Técnicos', descricao: 'Entrada e saída de técnicos e relatórios', rotas: ['/tecnicos'] },
	{ id: 'monitor', nome: 'Monitor', descricao: 'Disponibilidade de hosts e serviços', rotas: ['/monitor'] },
	{ id: 'links', nome: 'Links úteis', descricao: 'Atalhos da equipe', rotas: ['/links'] },
	{ id: 'avisos', nome: 'Mural de avisos', descricao: 'Avisos no painel e na TV', rotas: [] },
	{ id: 'tira_duvidas', nome: 'Tira-dúvidas', descricao: 'Assistente da tela de acesso e seus procedimentos', rotas: ['/procedimentos'] },
	{ id: 'tv', nome: 'Modo TV', descricao: 'Painel para o telão e telas cadastradas', rotas: ['/tv'] },
	{ id: 'radio', nome: 'Rádio', descricao: 'Rádio no menu lateral', rotas: [] }
];

// Módulo dono da rota, se houver (a rota mais específica ganha: /plantao/tv é do plantão)
export function moduloDaRota(path: string): ModuloInfo | undefined {
	let achado: ModuloInfo | undefined;
	let tamanho = 0;
	for (const m of MODULOS) {
		for (const r of m.rotas) {
			if ((path === r || path.startsWith(r + '/')) && r.length > tamanho) {
				achado = m;
				tamanho = r.length;
			}
		}
	}
	return achado;
}
