import { createHash, randomUUID } from 'node:crypto';
import type { Cookies } from '@sveltejs/kit';
import {
  ANON_REPORTER_COOKIE_MAX_AGE_SECONDS,
  ANON_REPORTER_COOKIE_NAME
} from '$lib/constants';
import { env } from '$server/env';

const deriveReporterHash = (anonymousReporterId: string): string => {
  return createHash('sha256')
    .update(`${anonymousReporterId}:${env.APP_SIGNING_SECRET}`)
    .digest('hex');
};

const generateAnonymousReporterId = (): string => randomUUID();

export const ensureAnonymousReporterIdentity = (
  cookies: Cookies,
  isSecure: boolean
): { anonymousReporterId: string; reporterHash: string } => {
  let anonymousReporterId = cookies.get(ANON_REPORTER_COOKIE_NAME);

  if (!anonymousReporterId) {
    anonymousReporterId = generateAnonymousReporterId();
    cookies.set(ANON_REPORTER_COOKIE_NAME, anonymousReporterId, {
      path: '/',
      httpOnly: true,
      sameSite: 'lax',
      secure: isSecure,
      maxAge: ANON_REPORTER_COOKIE_MAX_AGE_SECONDS
    });
  }

  return {
    anonymousReporterId,
    reporterHash: deriveReporterHash(anonymousReporterId)
  };
};
