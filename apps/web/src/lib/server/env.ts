import { z } from 'zod';

const normalizeBaseUrl = (value: string): string => value.replace(/\/$/, '');

const envSchema = z.object({
  APP_SIGNING_SECRET: z.string().min(16).default('local-development-secret-change-me'),
  OPERATOR_EMAIL: z.string().email().default('operator@zwerffiets.local'),
  OPERATOR_PASSWORD: z.string().min(8).default('changeme-operator'),
  EXPORT_EMAIL_TO: z.string().default('ops@zwerffiets.local'),
  ENABLE_EXPORT_SCHEDULER: z.string().default('true'),
  PUBLIC_BASE_URL: z.string().url().default('https://zwerffiets.org')
});

export const env = envSchema.parse(process.env);

export const schedulerEnabled = env.ENABLE_EXPORT_SCHEDULER.toLowerCase() === 'true';
export const publicBaseUrl = normalizeBaseUrl(env.PUBLIC_BASE_URL);

export const buildPublicUrl = (path: string): string => {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${publicBaseUrl}${normalizedPath}`;
};
