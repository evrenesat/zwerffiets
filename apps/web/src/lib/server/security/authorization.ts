import { ApiError } from '$server/errors';
import type { OperatorRole } from '$lib/types';
import type { RequestEvent } from '@sveltejs/kit';

export const requireOperatorSession = (
  event: RequestEvent,
  allowedRoles: OperatorRole[] = ['operator']
) => {
  const session = event.locals.operatorSession;

  if (!session) {
    throw new ApiError(401, 'unauthorized', 'Operator session required');
  }

  if (!allowedRoles.includes(session.role)) {
    throw new ApiError(403, 'forbidden', 'Insufficient role');
  }

  return session;
};
