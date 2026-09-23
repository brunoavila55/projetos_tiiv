export interface ApiErrorResponse {
	error: string;
}

export class ApiError extends Error {
	constructor(
		public status: number,
		message: string
	) {
		super(message);
		this.name = 'ApiError';
	}
}

export async function apiFetch<T>(
	url: string,
	options: RequestInit = {}
): Promise<T> {
	const defaultHeaders: Record<string, string> = {
		Accept: 'application/json'
	};

	if (options.body && typeof options.body === 'string') {
		defaultHeaders['Content-Type'] = 'application/json';
	}

	const response = await fetch(url, {
		...options,
		credentials: 'include', // Envia os cookies HttpOnly da sessão
		headers: {
			...defaultHeaders,
			...options.headers
		}
	});

	if (response.status === 204) {
		return {} as T;
	}

	const isJson = response.headers.get('content-type')?.includes('application/json');
	const data = isJson ? await response.json() : await response.text();

	if (!response.ok) {
		const message =
			(typeof data === 'object' && data?.error) ||
			(typeof data === 'string' && data) ||
			`Erro ${response.status}: ${response.statusText}`;
		throw new ApiError(response.status, message);
	}

	return data as T;
}
