/**
 * Profile pictures: how a chosen image becomes the square the server keeps.
 * Pure maths and addresses; the canvas itself stays in the interface.
 */

/** Side, in pixels, of the picture that is uploaded. */
export const AVATAR_SIZE = 256;

export interface SquareCrop {
  /** Top-left corner of the square in the source image, in pixels. */
  sx: number;
  sy: number;
  /** Side of the square in the source image. */
  side: number;
}

/** The largest square that fits in an image, centred; null when the image has no pixels. */
export function squareCrop(width: number, height: number): SquareCrop | null {
  if (!(width >= 1) || !(height >= 1)) return null;
  const side = Math.floor(Math.min(width, height));
  return { sx: Math.floor((width - side) / 2), sy: Math.floor((height - side) / 2), side };
}

/** Where a person's picture is served; version changes after an upload so caches let go of the old one. */
export function avatarUrl(userId: number, version = 0): string {
  return `/api/avatars/${userId}${version ? `?v=${version}` : ''}`;
}
