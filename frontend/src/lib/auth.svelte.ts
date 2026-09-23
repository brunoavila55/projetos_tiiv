import { apiFetch, ApiError } from './api';

export interface UserProfile {
	id: string;
	nome: string;
	cor: string;
	papel: 'admin' | 'usuario';
}

export interface UsuarioPublico {
	id: string;
	nome: string;
	cor: string;
}

class AuthStore {
	user = $state<UserProfile | null>(null);
	loading = $state<boolean>(true);
	lastActivity = $state<number>(Date.now());
	private idleTimer: any = null;

	constructor() {
		if (typeof window !== 'undefined') {
			this.setupActivityListeners();
		}
	}

	async checkAuth(): Promise<UserProfile | null> {
		this.loading = true;
		try {
			const u = await apiFetch<UserProfile>('/api/auth/me');
			this.user = u;
			this.lastActivity = Date.now();
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
		return u;
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
