import { ApiError } from '$server/errors';

const internalErrorBody = {
  error: 'internal_error',
  message: 'Internal server error'
};

export const toHttpResponse = (err: unknown): Response => {
  if (err instanceof ApiError) {
    return new Response(
      JSON.stringify({
        error: err.code,
        message: err.message
      }),
      {
        status: err.status,
        headers: {
          'content-type': 'application/json'
        }
      }
    );
  }

  if (err instanceof Error) {
    return new Response(
      JSON.stringify({
        error: 'internal_error',
        message: err.message
      }),
      {
        status: 500,
        headers: {
          'content-type': 'application/json'
        }
      }
    );
  }

  return new Response(JSON.stringify(internalErrorBody), {
    status: 500,
    headers: {
      'content-type': 'application/json'
    }
  });
};
