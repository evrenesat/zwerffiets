import { SignJWT, jwtVerify } from 'jose';
import { env } from '$server/env';

const encoder = new TextEncoder();
const secret = encoder.encode(env.APP_SIGNING_SECRET);

interface TrackingClaims {
  public_id: string;
}

export const createTrackingToken = async (publicId: string, expiresIn: string): Promise<string> => {
  return await new SignJWT({ public_id: publicId } satisfies TrackingClaims)
    .setProtectedHeader({ alg: 'HS256' })
    .setIssuedAt()
    .setExpirationTime(expiresIn)
    .sign(secret);
};

export const verifyTrackingToken = async (token: string): Promise<TrackingClaims> => {
  const { payload } = await jwtVerify(token, secret, { algorithms: ['HS256'] });

  if (typeof payload.public_id !== 'string') {
    throw new Error('Invalid tracking token payload');
  }

  return { public_id: payload.public_id };
};
