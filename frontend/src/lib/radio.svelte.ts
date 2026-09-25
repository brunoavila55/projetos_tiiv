import type Hls from 'hls.js';

// Rádio online via Radio Browser API (https://api.radio-browser.info).
// O áudio vive neste store, fora dos componentes, para continuar tocando
// ao navegar entre as telas.

export interface Estacao {
	stationuuid: string;
	name: string;
	url_resolved: string;
	tags: string;
	state: string;
	country: string;
	codec: string;
	bitrate: number;
	hls: number;
}

type Estado = 'parado' | 'carregando' | 'tocando' | 'erro';

const SERVIDOR_PADRAO = 'https://de1.api.radio-browser.info';
const CHAVE_FAVORITAS = 'tiiv_radio_favoritas';
const CHAVE_VOLUME = 'tiiv_radio_volume';

class RadioStore {
	aberto = $state(false);
	atual = $state<Estacao | null>(null);
	estado = $state<Estado>('parado');
	volume = $state(0.7);
	favoritas = $state<Estacao[]>([]);

	private audio: HTMLAudioElement | null = null;
	// Chrome/Firefox não tocam HLS (.m3u8) nativamente: hls.js monta o stream via MSE
	private hls: Hls | null = null;
	private sessao = 0;
	private servidor: Promise<string> | null = null;

	constructor() {
		if (typeof window === 'undefined') return;
		try {
			this.favoritas = JSON.parse(localStorage.getItem(CHAVE_FAVORITAS) ?? '[]');
			const v = Number(localStorage.getItem(CHAVE_VOLUME));
			if (localStorage.getItem(CHAVE_VOLUME) !== null && v >= 0 && v <= 1) this.volume = v;
		} catch {
			// preferências corrompidas: segue com o padrão
		}
	}

	// Descobre um espelho ativo da API; se falhar, usa o servidor padrão
	private base(): Promise<string> {
		this.servidor ??= fetch('https://all.api.radio-browser.info/json/servers')
			.then((r) => r.json() as Promise<{ name: string }[]>)
			.then((lista) => {
				const nomes = [...new Set(lista.map((s) => s.name))];
				return nomes.length ? `https://${nomes[Math.floor(Math.random() * nomes.length)]}` : SERVIDOR_PADRAO;
			})
			.catch(() => SERVIDOR_PADRAO);
		return this.servidor;
	}

	async buscar(termo: string): Promise<Estacao[]> {
		const params = new URLSearchParams({
			order: 'clickcount',
			reverse: 'true',
			hidebroken: 'true',
			limit: '40'
		});
		if (termo.trim()) params.set('name', termo.trim());
		else params.set('countrycode', 'BR');

		const r = await fetch(`${await this.base()}/json/stations/search?${params}`);
		if (!r.ok) throw new Error(`Radio Browser respondeu ${r.status}`);
		const lista = (await r.json()) as Estacao[];
		// Em HTTPS o navegador bloqueia streams HTTP (conteúdo misto)
		const https = location.protocol === 'https:';
		return lista.filter((e) => e.url_resolved && (!https || e.url_resolved.startsWith('https:')));
	}

	async tocar(estacao: Estacao) {
		const sessao = ++this.sessao;
		this.soltarHls();
		if (!this.audio) {
			this.audio = new Audio();
			this.audio.addEventListener('playing', () => (this.estado = 'tocando'));
			this.audio.addEventListener('waiting', () => (this.estado = 'carregando'));
			this.audio.addEventListener('error', () => {
				if (this.audio?.getAttribute('src')) this.estado = 'erro';
			});
		}
		// Cala a estação anterior já, sem esperar a próxima conectar
		this.audio.pause();
		this.audio.removeAttribute('src');
		this.atual = estacao;
		this.estado = 'carregando';
		this.audio.volume = this.volume;

		// Estações mortas no diretório às vezes nunca respondem: desiste após 15 s
		setTimeout(() => {
			if (sessao === this.sessao && this.estado === 'carregando' && !this.audio?.currentTime) {
				this.parar();
				this.estado = 'erro';
			}
		}, 15_000);

		const url = estacao.url_resolved;
		const ehHls = estacao.hls === 1 || /\.m3u8(\?|$)/i.test(url);
		if (ehHls && !this.audio.canPlayType('application/vnd.apple.mpegurl')) {
			const { default: Hls } = await import('hls.js');
			// Outra estação foi escolhida (ou a rádio parou) enquanto a biblioteca carregava
			if (sessao !== this.sessao || !this.audio) return;
			if (!Hls.isSupported()) {
				this.estado = 'erro';
				return;
			}
			const hls = new Hls({ enableWorker: false });
			hls.on(Hls.Events.ERROR, (_e, dados) => {
				if (dados.fatal && this.hls === hls) {
					this.soltarHls();
					this.estado = 'erro';
				}
			});
			hls.loadSource(url);
			hls.attachMedia(this.audio);
			this.hls = hls;
		} else {
			this.audio.src = url;
		}
		this.audio.play().catch(() => {
			if (sessao === this.sessao) this.estado = 'erro';
		});

		// Contabiliza o clique na API (ajuda o ranking de estações); falha é irrelevante
		this.base()
			.then((b) => fetch(`${b}/json/url/${encodeURIComponent(estacao.stationuuid)}`))
			.catch(() => {});
	}

	alternar() {
		if (!this.atual) return;
		if (this.estado === 'tocando' || this.estado === 'carregando') this.parar();
		else this.tocar(this.atual);
	}

	private soltarHls() {
		this.hls?.destroy();
		this.hls = null;
	}

	parar() {
		this.sessao++;
		this.soltarHls();
		if (this.audio) {
			this.audio.pause();
			// Solta a conexão do stream em vez de só pausar
			this.audio.removeAttribute('src');
			this.audio.load();
		}
		this.estado = 'parado';
	}

	setVolume(v: number) {
		this.volume = v;
		if (this.audio) this.audio.volume = v;
		try {
			localStorage.setItem(CHAVE_VOLUME, String(v));
		} catch {
			// sem armazenamento local: volume vale só nesta sessão
		}
	}

	ehFavorita(uuid: string): boolean {
		return this.favoritas.some((e) => e.stationuuid === uuid);
	}

	alternarFavorita(estacao: Estacao) {
		this.favoritas = this.ehFavorita(estacao.stationuuid)
			? this.favoritas.filter((e) => e.stationuuid !== estacao.stationuuid)
			: [...this.favoritas, estacao];
		try {
			localStorage.setItem(CHAVE_FAVORITAS, JSON.stringify(this.favoritas));
		} catch {
			// idem
		}
	}
}

export const radio = new RadioStore();
