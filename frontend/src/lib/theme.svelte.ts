import { apiFetch } from './api';

export type Tema = 'claro' | 'escuro' | 'sistema';

class ThemeStore {
	tema = $state<Tema>('sistema');

	constructor() {
		if (typeof window !== 'undefined') {
			const saved = localStorage.getItem('tiiv_tema') as Tema | null;
			if (saved && (saved === 'claro' || saved === 'escuro' || saved === 'sistema')) {
				this.tema = saved;
			}
			this.applyTheme();

			// Ouvir mudanças na preferência do sistema
			const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
			mediaQuery.addEventListener('change', () => {
				if (this.tema === 'sistema') {
					this.applyTheme();
				}
			});
		}
	}

	initFromUser(userTema?: string) {
		if (userTema === 'claro' || userTema === 'escuro' || userTema === 'sistema') {
			this.tema = userTema as Tema;
			localStorage.setItem('tiiv_tema', userTema);
			this.applyTheme();
		}
	}

	async setTema(novoTema: Tema) {
		this.tema = novoTema;
		if (typeof window !== 'undefined') {
			localStorage.setItem('tiiv_tema', novoTema);
			this.applyTheme();
		}

		try {
			await apiFetch('/api/auth/tema', {
				method: 'PUT',
				body: JSON.stringify({ tema: novoTema }),
				silent: true
			});
		} catch {
			// Não bloqueia caso o usuário esteja offline ou requisição falhe
		}
	}

	applyTheme() {
		if (typeof document === 'undefined') return;

		let isDark = false;
		if (this.tema === 'escuro') {
			isDark = true;
		} else if (this.tema === 'claro') {
			isDark = false;
		} else {
			isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
		}

		if (isDark) {
			document.documentElement.classList.add('dark');
		} else {
			document.documentElement.classList.remove('dark');
		}
	}
}

export const themeStore = new ThemeStore();
