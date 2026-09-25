import { apiFetch } from './api';

// Contador de monitores fora do ar, exibido no menu e atualizado pela tela do monitor
class MonitoresStore {
	offline = $state(0);

	async atualizar() {
		try {
			const res = await apiFetch<{ offline: number }>('/api/monitores/resumo', { silent: true });
			this.offline = res.offline;
		} catch {
			// Sem sessão ou falha momentânea: mantém o último valor
		}
	}
}

export const monitoresStore = new MonitoresStore();
