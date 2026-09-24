import { apiFetch } from './api';

// Contador de tickets abertos, exibido no menu e atualizado pela tela de tickets
class TicketsStore {
	abertos = $state(0);

	async atualizar() {
		try {
			const res = await apiFetch<{ abertos: number }>('/api/tickets/resumo', { silent: true });
			this.abertos = res.abertos;
		} catch {
			// Sem sessão ou falha momentânea: mantém o último valor
		}
	}
}

export const ticketsStore = new TicketsStore();
