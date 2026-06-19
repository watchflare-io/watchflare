import { z } from 'zod';

// ===== Regex patterns =====

const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/;
const ipv6Regex = /^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$/;

function isValidIP(value: string): boolean {
	if (ipv4Regex.test(value)) {
		return value.split('.').every((part) => {
			const num = parseInt(part, 10);
			return num >= 0 && num <= 255;
		});
	}
	return ipv6Regex.test(value);
}

// ===== Schemas =====

export const loginSchema = z.object({
	email: z.string().min(1, 'Email is required').email('Invalid email address'),
	password: z.string().min(1, 'Password is required')
});

export const registerSchema = z
	.object({
		email: z.string().min(1, 'Email is required').email('Invalid email address'),
		password: z.string().min(12, 'Password must be at least 12 characters'),
		confirmPassword: z.string().min(1, 'Please confirm your password')
	})
	.refine((data) => data.password === data.confirmPassword, {
		message: 'Passwords do not match',
		path: ['confirmPassword']
	});

export const createHostSchema = z
	.object({
		name: z
			.string()
			.min(2, 'Name must be at least 2 characters')
			.max(64, 'Name must be at most 64 characters'),
		configuredIP: z.string(),
		allowAnyIP: z.boolean()
	})
	.refine((data) => data.allowAnyIP || data.configuredIP.length > 0, {
		message: 'IP address is required when "Allow any IP" is disabled',
		path: ['configuredIP']
	})
	.refine((data) => data.allowAnyIP || isValidIP(data.configuredIP), {
		message: 'Invalid IP address format',
		path: ['configuredIP']
	});

export const changePasswordSchema = z
	.object({
		currentPassword: z.string().min(1, 'Current password is required'),
		newPassword: z.string().min(8, 'New password must be at least 8 characters'),
		confirmPassword: z.string().min(1, 'Please confirm your new password')
	})
	.refine((data) => data.newPassword === data.confirmPassword, {
		message: 'Passwords do not match',
		path: ['confirmPassword']
	});

export const updateIPSchema = z.object({
	newIP: z.string().min(1, 'IP address is required').refine(isValidIP, 'Invalid IP address format')
});

// ===== Validation helper =====

export type ValidationResult<T> =
	| { success: true; data: T }
	| { success: false; errors: Record<string, string> };

export function validateForm<T>(schema: z.ZodType<T>, data: unknown): ValidationResult<T> {
	const result = schema.safeParse(data);

	if (result.success) {
		return { success: true, data: result.data };
	}

	const errors: Record<string, string> = {};
	for (const issue of result.error.issues) {
		const key = issue.path.join('.');
		if (!errors[key]) {
			errors[key] = issue.message;
		}
	}

	return { success: false, errors };
}
