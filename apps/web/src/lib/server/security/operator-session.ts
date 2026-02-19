import { SignJWT, jwtVerify } from 'jose';
import { env } from '$server/env';
import type { OperatorRole, OperatorSession } from '$lib/types';

const encoder = new TextEncoder();
const secret = encoder.encode(env.APP_SIGNING_SECRET);

export const OPERATOR_COOKIE_NAME = 'zwerffiets_operator_session';

export const createOperatorSessionToken = async (session: OperatorSession): Promise<string> => {
  return await new SignJWT({ email: session.email, role: session.role })
    .setProtectedHeader({ alg: 'HS256' })
    .setIssuedAt()
    .setExpirationTime('8h')
    .sign(secret);
};

export const verifyOperatorSessionToken = async (token: string): Promise<OperatorSession> => {
  const { payload } = await jwtVerify(token, secret, { algorithms: ['HS256'] });

  if (typeof payload.email !== 'string') {
    throw new Error('Session is missing email');
  }

  if (payload.role !== 'operator') {
    throw new Error('Session role is invalid');
  }

  return { email: payload.email, role: payload.role as OperatorRole };
};
