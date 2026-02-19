export type LayoutNavVariant = 'landing' | 'report' | 'default';

export const layoutNavVariant = (path: string): LayoutNavVariant => {
  if (path === '/') {
    return 'landing';
  }
  if (path.startsWith('/report')) {
    return 'report';
  }
  return 'default';
};
