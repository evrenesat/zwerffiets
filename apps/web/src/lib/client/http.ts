export interface ApiErrorPayload {
  error?: string;
  message?: string;
}

export class ApiRequestError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

const parseApiErrorPayload = async (response: Response): Promise<ApiErrorPayload> => {
  try {
    const parsed = (await response.json()) as ApiErrorPayload;
    return parsed;
  } catch {
    return {};
  }
};

export const fetchJson = async <T>(input: string, init: RequestInit = {}): Promise<T> => {
  const response = await fetch(input, {
    credentials: 'include',
    ...init
  });

  if (!response.ok) {
    const payload = await parseApiErrorPayload(response);
    throw new ApiRequestError(
      response.status,
      payload.error ?? 'request_failed',
      payload.message ?? `Request failed (${response.status})`
    );
  }

  return (await response.json()) as T;
};

