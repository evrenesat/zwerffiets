export const USER_SESSION_ENDPOINT = '/api/v1/auth/session';
export const USER_LOGOUT_ENDPOINT = '/api/v1/auth/logout';
export const USER_MAGIC_LINK_ENDPOINT = '/api/v1/auth/request-magic-link';

export const isUserSessionUnauthorized = (status: number): boolean => {
  return status === 401;
};

export const isUserSessionOk = (status: number): boolean => {
  return status >= 200 && status < 300;
};
