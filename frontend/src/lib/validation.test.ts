import { describe, it, expect } from 'vitest';
import {
	loginSchema,
	registerSchema,
	createHostSchema,
	changePasswordSchema,
	updateIPSchema,
	validateForm
} from './validation';

describe('loginSchema', () => {
	it('accepts valid credentials', () => {
		const result = loginSchema.safeParse({
			email: 'test@example.com',
			password: 'secret'
		});
		expect(result.success).toBe(true);
	});

	it('rejects empty email', () => {
		const result = validateForm(loginSchema, { email: '', password: 'secret' });
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.email).toBe('Email is required');
	});

	it('rejects invalid email', () => {
		const result = validateForm(loginSchema, {
			email: 'not-an-email',
			password: 'secret'
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.email).toBe('Invalid email address');
	});

	it('rejects empty password', () => {
		const result = validateForm(loginSchema, {
			email: 'test@example.com',
			password: ''
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.password).toBe('Password is required');
	});
});

describe('registerSchema', () => {
	it('accepts valid registration', () => {
		const result = registerSchema.safeParse({
			email: 'test@example.com',
			password: 'longpassword12',
			confirmPassword: 'longpassword12'
		});
		expect(result.success).toBe(true);
	});

	it('rejects password shorter than 12 characters', () => {
		const result = validateForm(registerSchema, {
			email: 'test@example.com',
			password: 'short',
			confirmPassword: 'short'
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.password).toContain('12');
	});

	it('rejects mismatched passwords', () => {
		const result = validateForm(registerSchema, {
			email: 'test@example.com',
			password: 'longpassword12',
			confirmPassword: 'different12345'
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.confirmPassword).toBe('Passwords do not match');
	});

	it('rejects empty confirm password', () => {
		const result = validateForm(registerSchema, {
			email: 'test@example.com',
			password: 'longpassword12',
			confirmPassword: ''
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.confirmPassword).toBeDefined();
	});
});

describe('createHostSchema', () => {
	it('accepts valid host with IP', () => {
		const result = createHostSchema.safeParse({
			name: 'web-server-01',
			configuredIP: '192.168.1.100',
			allowAnyIP: false
		});
		expect(result.success).toBe(true);
	});

	it('accepts host with allowAnyIP and no IP', () => {
		const result = createHostSchema.safeParse({
			name: 'web-server-01',
			configuredIP: '',
			allowAnyIP: true
		});
		expect(result.success).toBe(true);
	});

	it('rejects name shorter than 2 characters', () => {
		const result = validateForm(createHostSchema, {
			name: 'a',
			configuredIP: '192.168.1.1',
			allowAnyIP: false
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.name).toContain('2');
	});

	it('rejects name longer than 64 characters', () => {
		const result = validateForm(createHostSchema, {
			name: 'a'.repeat(65),
			configuredIP: '192.168.1.1',
			allowAnyIP: false
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.name).toContain('64');
	});

	it('rejects empty IP when allowAnyIP is false', () => {
		const result = validateForm(createHostSchema, {
			name: 'web-server',
			configuredIP: '',
			allowAnyIP: false
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.configuredIP).toContain('required');
	});

	it('rejects invalid IP format', () => {
		const result = validateForm(createHostSchema, {
			name: 'web-server',
			configuredIP: '999.999.999.999',
			allowAnyIP: false
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.configuredIP).toContain('Invalid');
	});

	it('rejects non-IP string', () => {
		const result = validateForm(createHostSchema, {
			name: 'web-server',
			configuredIP: 'not-an-ip',
			allowAnyIP: false
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.configuredIP).toBeDefined();
	});
});

describe('changePasswordSchema', () => {
	it('accepts valid password change', () => {
		const result = changePasswordSchema.safeParse({
			currentPassword: 'oldpass',
			newPassword: 'newpass12345',
			confirmPassword: 'newpass12345'
		});
		expect(result.success).toBe(true);
	});

	it('rejects empty current password', () => {
		const result = validateForm(changePasswordSchema, {
			currentPassword: '',
			newPassword: 'newpass12345',
			confirmPassword: 'newpass12345'
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.currentPassword).toBeDefined();
	});

	it('rejects short new password', () => {
		const result = validateForm(changePasswordSchema, {
			currentPassword: 'oldpass',
			newPassword: 'short',
			confirmPassword: 'short'
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.newPassword).toContain('8');
	});

	it('rejects mismatched passwords', () => {
		const result = validateForm(changePasswordSchema, {
			currentPassword: 'oldpass',
			newPassword: 'newpass12345',
			confirmPassword: 'different123'
		});
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.confirmPassword).toBe('Passwords do not match');
	});
});

describe('updateIPSchema', () => {
	it('accepts valid IPv4', () => {
		const result = updateIPSchema.safeParse({ newIP: '192.168.1.1' });
		expect(result.success).toBe(true);
	});

	it('accepts valid IPv6', () => {
		const result = updateIPSchema.safeParse({
			newIP: '2001:0db8:85a3:0000:0000:8a2e:0370:7334'
		});
		expect(result.success).toBe(true);
	});

	it('accepts shortened IPv6', () => {
		const result = updateIPSchema.safeParse({ newIP: '::1' });
		expect(result.success).toBe(true);
	});

	it('rejects empty IP', () => {
		const result = validateForm(updateIPSchema, { newIP: '' });
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.newIP).toBeDefined();
	});

	it('rejects invalid IP', () => {
		const result = validateForm(updateIPSchema, { newIP: 'abc.def.ghi.jkl' });
		expect(result.success).toBe(false);
		if (!result.success) expect(result.errors.newIP).toContain('Invalid');
	});

	it('rejects IPv4 with octets > 255', () => {
		const result = validateForm(updateIPSchema, { newIP: '256.1.1.1' });
		expect(result.success).toBe(false);
	});

	it('rejects partial IPv4', () => {
		const result = validateForm(updateIPSchema, { newIP: '192.168.1' });
		expect(result.success).toBe(false);
	});
});

describe('validateForm', () => {
	it('returns success with data on valid input', () => {
		const result = validateForm(loginSchema, {
			email: 'test@example.com',
			password: 'secret'
		});
		expect(result.success).toBe(true);
		if (result.success) {
			expect(result.data.email).toBe('test@example.com');
			expect(result.data.password).toBe('secret');
		}
	});

	it('returns errors as Record<string, string> on invalid input', () => {
		const result = validateForm(loginSchema, { email: '', password: '' });
		expect(result.success).toBe(false);
		if (!result.success) {
			expect(typeof result.errors).toBe('object');
			expect(typeof result.errors.email).toBe('string');
			expect(typeof result.errors.password).toBe('string');
		}
	});

	it('returns only the first error per field', () => {
		const result = validateForm(loginSchema, { email: '', password: '' });
		expect(result.success).toBe(false);
		if (!result.success) {
			// email has min(1) then email() - should only show first error
			expect(result.errors.email).toBe('Email is required');
		}
	});
});
