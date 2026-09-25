// Mecânica comum das telas de TV (/tv e /plantao/tv): chave da tela, tema
// escuro com fonte proporcional, tela sempre ligada, recarga periódica,
// controles que somem sozinhos e tela cheia.
import { themeStore } from '$lib/theme.svelte';

const CHAVE_STORAGE = 'tiiv_tv_chave';
// Recarrega a página de tempos em tempos para pegar atualizações do sistema
const RECARREGAR_MS = 6 * 60 * 60 * 1000;

export function lerChaveTV(): string | null {
	// O link da tela traz a chave no fragmento (#chave=...), que não vai ao servidor;
	// guarda e limpa a barra de endereço. A mesma chave vale para as duas TVs.
	const m = location.hash.match(/chave=([0-9a-f]{64})/i);
	if (m) {
		try {
			localStorage.setItem(CHAVE_STORAGE, m[1]);
		} catch {
			// sem armazenamento: vale só enquanto a página estiver aberta
		}
		history.replaceState(null, '', location.pathname);
		return m[1];
	}
	try {
		return localStorage.getItem(CHAVE_STORAGE);
	} catch {
		return null;
	}
}

// Chave revogada: esquece para não ficar tentando
export function esquecerChaveTV() {
	try {
		localStorage.removeItem(CHAVE_STORAGE);
	} catch {
		// nada a limpar
	}
}

export class ModoTV {
	telaCheia = $state(false);
	controlesVisiveis = $state(true);

	// Liga o modo TV; devolve a função que desfaz tudo (para o onMount)
	iniciar(opts: { aoVoltarVisivel: () => void; podeRecarregar: () => boolean }): () => void {
		// TV sempre no tema escuro e com a escala acompanhando a largura da tela
		const html = document.documentElement;
		const tamanhoAnterior = html.style.fontSize;
		html.classList.add('dark');
		html.style.fontSize = 'clamp(13px, 0.85vw, 40px)';

		const timerRecarga = setTimeout(function recarregar() {
			if (opts.podeRecarregar()) location.reload();
			else setTimeout(recarregar, 60_000);
		}, RECARREGAR_MS);

		// Mantém a tela ligada (quando o navegador permite)
		let wakeLock: { release: () => Promise<void> } | null = null;
		const pedirTelaLigada = async () => {
			try {
				wakeLock = await (navigator as any).wakeLock?.request('screen');
			} catch {
				// sem suporte ou sem permissão
			}
		};
		const aoMudarVisibilidade = () => {
			if (document.visibilityState === 'visible') {
				pedirTelaLigada();
				opts.aoVoltarVisivel();
			}
		};
		pedirTelaLigada();
		document.addEventListener('visibilitychange', aoMudarVisibilidade);

		// Controles e cursor somem sozinhos
		let timerControles: ReturnType<typeof setTimeout>;
		const mostrarControles = () => {
			this.controlesVisiveis = true;
			clearTimeout(timerControles);
			timerControles = setTimeout(() => (this.controlesVisiveis = false), 3000);
		};
		mostrarControles();
		window.addEventListener('mousemove', mostrarControles);
		window.addEventListener('keydown', mostrarControles);
		const aoMudarTelaCheia = () => (this.telaCheia = !!document.fullscreenElement);
		document.addEventListener('fullscreenchange', aoMudarTelaCheia);

		return () => {
			clearTimeout(timerRecarga);
			clearTimeout(timerControles);
			wakeLock?.release().catch(() => {});
			document.removeEventListener('visibilitychange', aoMudarVisibilidade);
			window.removeEventListener('mousemove', mostrarControles);
			window.removeEventListener('keydown', mostrarControles);
			document.removeEventListener('fullscreenchange', aoMudarTelaCheia);
			html.style.fontSize = tamanhoAnterior;
			themeStore.applyTheme();
		};
	}

	alternarTelaCheia() {
		if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
		else document.documentElement.requestFullscreen().catch(() => {});
	}
}
