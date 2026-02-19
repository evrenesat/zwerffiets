import { describe, expect, it } from 'vitest';
import {
  isUserSessionOk,
  isUserSessionUnauthorized,
  USER_LOGOUT_ENDPOINT,
  USER_MAGIC_LINK_ENDPOINT,
  USER_SESSION_ENDPOINT
} from '$lib/client/user-auth';

describe('user auth helpers', () => {
  it('uses stable user auth endpoints', () => {
    expect(USER_SESSION_ENDPOINT).toBe('/api/v1/auth/session');
    expect(USER_LOGOUT_ENDPOINT).toBe('/api/v1/auth/logout');
    expect(USER_MAGIC_LINK_ENDPOINT).toBe('/api/v1/auth/request-magic-link');
  });

  it('identifies unauthorized session status', () => {
    expect(isUserSessionUnauthorized(401)).toBe(true);
    expect(isUserSessionUnauthorized(200)).toBe(false);
  });

  it('identifies successful session status', () => {
    expect(isUserSessionOk(200)).toBe(true);
    expect(isUserSessionOk(204)).toBe(true);
    expect(isUserSessionOk(401)).toBe(false);
    expect(isUserSessionOk(500)).toBe(false);
  });
});
