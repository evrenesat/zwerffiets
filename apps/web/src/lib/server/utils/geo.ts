const EARTH_RADIUS_METERS = 6371000;
const DEGREES_TO_RADIANS = Math.PI / 180;

const toRadians = (degrees: number): number => degrees * DEGREES_TO_RADIANS;

export const haversineMeters = (
  latA: number,
  lngA: number,
  latB: number,
  lngB: number
): number => {
  const deltaLat = toRadians(latB - latA);
  const deltaLng = toRadians(lngB - lngA);

  const a =
    Math.sin(deltaLat / 2) ** 2 +
    Math.cos(toRadians(latA)) * Math.cos(toRadians(latB)) * Math.sin(deltaLng / 2) ** 2;

  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return EARTH_RADIUS_METERS * c;
};
