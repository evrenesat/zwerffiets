import type { Tag } from '$lib/types';

export const DEFAULT_TAGS: ReadonlyArray<Omit<Tag, 'id'>> = [
  { code: 'flat_tires', label: 'Flat tires', isActive: true },
  { code: 'rusted', label: 'Rusted', isActive: true },
  { code: 'missing_parts', label: 'Missing parts', isActive: true },
  { code: 'blocking_sidewalk', label: 'Blocking sidewalk', isActive: true },
  { code: 'damaged_frame', label: 'Damaged frame', isActive: true },
  { code: 'abandoned_long_time', label: 'Abandoned for long time', isActive: true },
  { code: 'no_chain', label: 'No chain', isActive: true },
  { code: 'wheel_missing', label: 'Missing wheel', isActive: true },
  { code: 'no_seat', label: 'No seat', isActive: true },
  { code: 'other_visibility_issue', label: 'Other visibility issue', isActive: true }
] as const;
