import { apiFetch } from './api';

const LADO = 480;

/**
 * Recorta a imagem no centro em formato quadrado e reduz para 480×480 JPEG.
 * Assim uma foto de celular de vários MB vira ~50 KB antes de sair do navegador.
 */
export async function prepararFoto(arquivo: File): Promise<Blob> {
	if (!arquivo.type.startsWith('image/')) {
		throw new Error('Escolha um arquivo de imagem.');
	}

	let bitmap: ImageBitmap;
	try {
		bitmap = await createImageBitmap(arquivo, { imageOrientation: 'from-image' });
	} catch {
		throw new Error('Não foi possível ler esta imagem. Tente uma foto JPEG ou PNG.');
	}

	const lado = Math.min(bitmap.width, bitmap.height);
	const sx = (bitmap.width - lado) / 2;
	const sy = (bitmap.height - lado) / 2;
	const destino = Math.min(LADO, lado);

	const canvas = document.createElement('canvas');
	canvas.width = destino;
	canvas.height = destino;
	const ctx = canvas.getContext('2d')!;
	ctx.imageSmoothingQuality = 'high';
	ctx.drawImage(bitmap, sx, sy, lado, lado, 0, 0, destino, destino);
	bitmap.close();

	return new Promise((resolve, reject) =>
		canvas.toBlob(
			(blob) => (blob ? resolve(blob) : reject(new Error('Falha ao processar a imagem.'))),
			'image/jpeg',
			0.86
		)
	);
}

export async function enviarFoto(usuarioId: string, arquivo: File): Promise<number | null> {
	const blob = await prepararFoto(arquivo);
	const res = await apiFetch<{ foto_versao: number | null }>(`/api/usuarios/${usuarioId}/foto`, {
		method: 'PUT',
		body: blob,
		headers: { 'Content-Type': 'image/jpeg' }
	});
	return res.foto_versao;
}

export async function removerFoto(usuarioId: string): Promise<void> {
	await apiFetch(`/api/usuarios/${usuarioId}/foto`, { method: 'DELETE' });
}
