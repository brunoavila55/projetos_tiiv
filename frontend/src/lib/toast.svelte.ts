export interface Toast {
	id: string;
	tipo: 'erro' | 'sucesso' | 'info';
	mensagem: string;
}

class ToastStore {
	toasts = $state<Toast[]>([]);

	add(mensagem: string, tipo: 'erro' | 'sucesso' | 'info' = 'info', duracaoMs = 4000) {
		const id = Math.random().toString(36).substring(2, 9);
		this.toasts = [...this.toasts, { id, tipo, mensagem }];

		if (duracaoMs > 0) {
			setTimeout(() => {
				this.remove(id);
			}, duracaoMs);
		}
	}

	error(mensagem: string, duracaoMs = 5000) {
		this.add(mensagem, 'erro', duracaoMs);
	}

	success(mensagem: string, duracaoMs = 3000) {
		this.add(mensagem, 'sucesso', duracaoMs);
	}

	info(mensagem: string, duracaoMs = 4000) {
		this.add(mensagem, 'info', duracaoMs);
	}

	remove(id: string) {
		this.toasts = this.toasts.filter(t => t.id !== id);
	}
}

export const toast = new ToastStore();
