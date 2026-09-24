import { marked } from 'marked';
import DOMPurify from 'dompurify';

marked.setOptions({ gfm: true, breaks: true });

// Links abrem em outra aba, sem dar acesso à janela de origem
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
	if (node.tagName === 'A') {
		node.setAttribute('target', '_blank');
		node.setAttribute('rel', 'noopener noreferrer');
	}
});

/**
 * Converte Markdown em HTML seguro. Sempre sanitizado: o texto pode vir do
 * modelo de IA na tela pública, não só do admin.
 */
export function renderMarkdown(texto: string): string {
	const html = marked.parse(texto, { async: false });
	return DOMPurify.sanitize(html, { FORBID_TAGS: ['img', 'style', 'iframe', 'form', 'input'] });
}
