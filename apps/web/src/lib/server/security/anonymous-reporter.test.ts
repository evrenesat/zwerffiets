import { describe, expect, it } from 'vitest';
import type { Cookies } from '@sveltejs/kit';
import {
  ANON_REPORTER_COOKIE_MAX_AGE_SECONDS,
  ANON_REPORTER_COOKIE_NAME
} from '$lib/constants';
import { ensureAnonymousReporterIdentity } from '$server/security/anonymous-reporter';

interface MockCookieState {
  values: Map<string, string>;
  lastSet:
    | {
        name: string;
        value: string;
        options: Parameters<Cookies['set']>[2] | undefined;
      }
    | null;
}

const createCookieJar = (initial?: Record<string, string>): { cookies: Cookies; state: MockCookieState } => {
  const state: MockCookieState = {
    values: new Map(Object.entries(initial ?? {})),
    lastSet: null
  };

  const cookies = {
    get: (name: string) => state.values.get(name),
    set: (name: string, value: string, options?: Parameters<Cookies['set']>[2]) => {
      state.values.set(name, value);
      state.lastSet = {
        name,
        value,
        options
      };
    }
  } as unknown as Cookies;

  return { cookies, state };
};

describe('anonymous reporter identity', () => {
  it('creates a new anonymous reporter cookie and derives a reporter hash', () => {
    const { cookies, state } = createCookieJar();

    const identity = ensureAnonymousReporterIdentity(cookies, true);

    expect(identity.anonymousReporterId.length).toBeGreaterThan(10);
    expect(identity.reporterHash).toHaveLength(64);
    expect(state.lastSet?.name).toBe(ANON_REPORTER_COOKIE_NAME);
    expect(state.lastSet?.options?.httpOnly).toBe(true);
    expect(state.lastSet?.options?.sameSite).toBe('lax');
    expect(state.lastSet?.options?.secure).toBe(true);
    expect(state.lastSet?.options?.maxAge).toBe(ANON_REPORTER_COOKIE_MAX_AGE_SECONDS);
  });

  it('reuses existing cookie without writing a new one and keeps hash stable', () => {
    const existingId = '10000000-0000-4000-8000-000000000001';
    const { cookies, state } = createCookieJar({
      [ANON_REPORTER_COOKIE_NAME]: existingId
    });

    const first = ensureAnonymousReporterIdentity(cookies, false);
    const second = ensureAnonymousReporterIdentity(cookies, false);

    expect(first.anonymousReporterId).toBe(existingId);
    expect(second.anonymousReporterId).toBe(existingId);
    expect(first.reporterHash).toBe(second.reporterHash);
    expect(state.lastSet).toBeNull();
  });
});
