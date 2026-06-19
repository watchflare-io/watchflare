import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ApiError } from './api';

// Mock fetch globally
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

// Mock window.location for auth redirect tests
const mockLocation = { href: '' };
vi.stubGlobal('window', { location: mockLocation });

function jsonResponse(data: unknown, status = 200, statusText = 'OK') {
	return Promise.resolve({
		ok: status >= 200 && status < 300,
		status,
		statusText,
		json: () => Promise.resolve(data)
	});
}

function networkError(message = 'Failed to fetch') {
	return Promise.reject(new Error(message));
}

function nonJsonResponse(status: number, statusText: string) {
	return Promise.resolve({
		ok: status >= 200 && status < 300,
		status,
		statusText,
		json: () => Promise.reject(new SyntaxError('Unexpected token'))
	});
}

beforeEach(() => {
	mockFetch.mockReset();
	mockLocation.href = '';
});

describe('ApiError', () => {
	it('creates error with message from data.error', () => {
		const err = new ApiError(400, 'Bad Request', { error: 'Invalid input' });
		expect(err.message).toBe('Invalid input');
		expect(err.status).toBe(400);
	});

	it('creates error with message from data.message', () => {
		const err = new ApiError(500, 'Internal', { message: 'Something broke' });
		expect(err.message).toBe('Something broke');
	});

	it('falls back to statusText', () => {
		const err = new ApiError(404, 'Not Found');
		expect(err.message).toBe('Not Found');
	});

	it('falls back to default message', () => {
		const err = new ApiError(0, '');
		expect(err.message).toBe('API request failed');
	});

	describe('status getters', () => {
		it('isAuthError is true for 401', () => {
			expect(new ApiError(401, 'Unauthorized').isAuthError).toBe(true);
			expect(new ApiError(403, 'Forbidden').isAuthError).toBe(false);
		});

		it('isForbidden is true for 403', () => {
			expect(new ApiError(403, 'Forbidden').isForbidden).toBe(true);
			expect(new ApiError(401, 'Unauthorized').isForbidden).toBe(false);
		});

		it('isNotFound is true for 404', () => {
			expect(new ApiError(404, 'Not Found').isNotFound).toBe(true);
			expect(new ApiError(400, 'Bad Request').isNotFound).toBe(false);
		});

		it('isServerError is true for 500+', () => {
			expect(new ApiError(500, 'Internal').isServerError).toBe(true);
			expect(new ApiError(502, 'Bad Gateway').isServerError).toBe(true);
			expect(new ApiError(499, 'Client').isServerError).toBe(false);
		});
	});
});

describe('login', () => {
	it('sends POST with email and password', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ token: 'jwt123' }));
		const { login } = await import('./api');

		await login('test@example.com', 'password123');

		expect(mockFetch).toHaveBeenCalledWith(
			'/api/v1/auth/login',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ email: 'test@example.com', password: 'password123' }),
				credentials: 'include'
			})
		);
	});

	it('returns response data on success', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ token: 'jwt123', user: { id: 1 } }));
		const { login } = await import('./api');

		const result = await login('test@example.com', 'password123');
		expect(result).toEqual({ token: 'jwt123', user: { id: 1 } });
	});

	it('throws ApiError on invalid credentials', async () => {
		mockFetch.mockReturnValueOnce(
			jsonResponse({ error: 'invalid credentials' }, 401, 'Unauthorized')
		);
		const { login } = await import('./api');

		try {
			await login('test@example.com', 'wrong');
			expect.unreachable('should have thrown');
		} catch (err) {
			expect(err).toBeInstanceOf(ApiError);
			// ApiError capitalizes backend messages (lowercase by Go convention).
			expect((err as ApiError).message).toBe('Invalid credentials');
		}
	});

	it('does not redirect on 401 (skipAuthRedirect)', async () => {
		mockFetch.mockReturnValueOnce(
			jsonResponse({ error: 'invalid credentials' }, 401, 'Unauthorized')
		);
		const { login } = await import('./api');

		await login('test@example.com', 'wrong').catch(() => {});
		expect(mockLocation.href).toBe('');
	});
});

describe('register', () => {
	it('sends POST with email and password', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ token: 'jwt123' }));
		const { register } = await import('./api');

		await register('new@example.com', 'longpassword12');

		expect(mockFetch).toHaveBeenCalledWith(
			'/api/v1/auth/register',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ email: 'new@example.com', password: 'longpassword12', username: '' })
			})
		);
	});

	it('does not redirect on 401 (skipAuthRedirect)', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ error: 'unauthorized' }, 401, 'Unauthorized'));
		const { register } = await import('./api');

		await register('new@example.com', 'pass').catch(() => {});
		expect(mockLocation.href).toBe('');
	});
});

describe('listHosts', () => {
	it('fetches hosts without params', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ hosts: [], total: 0 }));
		const { listHosts } = await import('./api');

		await listHosts();

		expect(mockFetch).toHaveBeenCalledWith(
			'/api/v1/hosts',
			expect.objectContaining({ credentials: 'include' })
		);
	});

	it('adds pagination query params', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ hosts: [], total: 0 }));
		const { listHosts } = await import('./api');

		await listHosts({ page: 2, perPage: 10 });

		expect(mockFetch).toHaveBeenCalledWith('/api/v1/hosts?page=2&per_page=10', expect.anything());
	});
});

describe('createHost', () => {
	it('sends POST with host data', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ host: { id: '1' }, token: 'wf_reg_...' }));
		const { createHost } = await import('./api');

		await createHost('web-01', '192.168.1.1', false);

		expect(mockFetch).toHaveBeenCalledWith(
			'/api/v1/hosts',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({
					display_name: 'web-01',
					configured_ip: '192.168.1.1',
					allow_any_ip: false
				})
			})
		);
	});
});

describe('deleteHost', () => {
	it('sends DELETE request', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ message: 'deleted' }));
		const { deleteHost } = await import('./api');

		await deleteHost('abc-123');

		expect(mockFetch).toHaveBeenCalledWith(
			'/api/v1/hosts/abc-123',
			expect.objectContaining({ method: 'DELETE' })
		);
	});
});

describe('changePassword', () => {
	it('sends PUT with current and new password', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ message: 'ok' }));
		const { changePassword } = await import('./api');

		await changePassword('oldpass', 'newpass12345');

		expect(mockFetch).toHaveBeenCalledWith(
			'/api/v1/auth/change-password',
			expect.objectContaining({
				method: 'PUT',
				body: JSON.stringify({ current_password: 'oldpass', new_password: 'newpass12345' })
			})
		);
	});
});

describe('getHostMetrics', () => {
	it('fetches metrics with time_range param', async () => {
		mockFetch.mockReturnValueOnce(jsonResponse({ metrics: [] }));
		const { getHostMetrics } = await import('./api');

		await getHostMetrics('srv-1', { time_range: '24h' });

		expect(mockFetch).toHaveBeenCalledWith(
			'/api/v1/hosts/srv-1/metrics?time_range=24h',
			expect.anything()
		);
	});
});

describe('error handling (via login)', () => {
	it('throws ApiError on network error', async () => {
		mockFetch.mockReturnValueOnce(networkError('Network failure'));
		const { login } = await import('./api');

		try {
			await login('test@example.com', 'pass');
			expect.unreachable('should have thrown');
		} catch (err) {
			expect(err).toBeInstanceOf(ApiError);
			expect((err as ApiError).status).toBe(0);
			expect((err as ApiError).message).toBe('Network failure');
		}
	});

	it('throws ApiError on non-JSON error response', async () => {
		mockFetch.mockReturnValueOnce(nonJsonResponse(500, 'Internal Server Error'));
		const { login } = await import('./api');

		try {
			await login('test@example.com', 'pass');
			expect.unreachable('should have thrown');
		} catch (err) {
			expect(err).toBeInstanceOf(ApiError);
			expect((err as ApiError).status).toBe(500);
		}
	});

	it('throws ApiError with server error message', async () => {
		mockFetch.mockReturnValueOnce(
			jsonResponse({ error: 'rate limited' }, 429, 'Too Many Requests')
		);
		const { login } = await import('./api');

		try {
			await login('test@example.com', 'pass');
			expect.unreachable('should have thrown');
		} catch (err) {
			expect(err).toBeInstanceOf(ApiError);
			// ApiError capitalizes backend messages (lowercase by Go convention).
			expect((err as ApiError).message).toBe('Rate limited');
			expect((err as ApiError).status).toBe(429);
		}
	});
});
