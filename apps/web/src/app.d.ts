import type { OperatorSession } from '$lib/types';

declare global {
  const __BUILD_ID__: string;

  namespace App {
    interface Locals {
      operatorSession: OperatorSession | null;
    }
  }
}

export {};
