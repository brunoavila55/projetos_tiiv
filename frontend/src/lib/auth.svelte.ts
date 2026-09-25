import { apiFetch, ApiError } from './api';
import { themeStore } from './theme.svelte';

export interface Setor {
	id: string;
	nome: string;
}

export interface UserProfile {
	id: string;
	nome: string;
	cor: string;
	papel: 'superadmin' | 'admin' | 'usuario';
	// Setor de trabalho: tudo o que é separado por setor vem filtrado por ele
	setor: Setor;
	tema?: 'claro' | 'escuro' | 'sistema';
	foto_versao?: number | null;
	// Admin inicial: precisa trocar o PIN antes de usar o sistema
	deve_trocar_pin?: boolean;
}

export interface UsuarioPublico {
	id: string;
	nome: string;
	cor: string;
	foto_versao?: number | null;
}

class AuthStore {
	user = $state<UserProfile | null>(null);
	loading = $state<boolean>(true);
	lastActivity = $state<number>(Date.now());
	private idleTimer: any = null;

	// Admin do setor ou superadmin
	ehAdmin = $derived(this.user?.papel === 'admin' || this.user?.papel === 'superadmin');
	ehSuperadmin = $derived(this.user?.papel === 'superadmin');

	constructor() {
		if (typeof window !== 'undefined') {
			this.setupActivityListeners();
		}
	}

	async checkAuth(): Promise<UserProfile | null> {
		this.loading = true;
		try {
			const u = await apiFetch<UserProfile>('/api/auth/me', { silent: true });
			this.user = u;
			this.lastActivity = Date.now();
			if (u?.tema) {
				themeStore.initFromUser(u.tema);
			}
			return u;
		} catch {
			this.user = null;
			return null;
		} finally {
			this.loading = false;
		}
	}

	async login(usuarioId: string, pin: string): Promise<UserProfile> {
		const u = await apiFetch<UserProfile>('/api/auth/login', {
			method: 'POST',
			body: JSON.stringify({ usuario_id: usuarioId, pin })
		});
		this.user = u;
		this.lastActivity = Date.now();
		if (u?.tema) {
			themeStore.initFromUser(u.tema);
		}
		return u;
	}

	// Superadmin: passa a ver outro setor nesta sessão. A página é recarregada
	// para nenhuma tela ficar com dados do setor anterior.
	async trocarSetor(setorId: string) {
		await apiFetch<Setor>('/api/auth/setor', {
			method: 'PUT',
			body: JSON.stringify({ setor_id: setorId })
		});
		window.location.reload();
	}

	async logout() {
		try {
			await apiFetch('/api/auth/logout', { method: 'POST' });
		} catch {
			// ignora erros de logout
		} finally {
			this.user = null;
		}
	}

	recordActivity() {
		this.lastActivity = Date.now();
	}

	private setupActivityListeners() {
		const update = () => this.recordActivity();
		window.addEventListener('mousemove', update, { passive: true });
		window.addEventListener('keydown', update, { passive: true });
		window.addEventListener('touchstart', update, { passive: true });
		window.addEventListener('click', update, { passive: true });

		// Checar inatividade a cada minuto (30 minutos = 1.800.000 ms)
		this.idleTimer = setInterval(() => {
			if (this.user && Date.now() - this.lastActivity > 30 * 60 * 1000) {
				console.warn('Sessão expirada por inatividade de 30 minutos');
				this.logout();
			}
		}, 60 * 1000);
	}
}

export const auth = new AuthStore();
