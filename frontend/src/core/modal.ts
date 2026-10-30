/**
 * Modals: whether a click lands on the backdrop. Pure geometry; the dialog
 * element stays in the interface.
 */

export interface Point {
  x: number;
  y: number;
}

export interface Rect {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

/** Whether a point lies outside a rect; a point on its edge is inside. */
export function outside(rect: Rect, p: Point): boolean {
  return p.x < rect.left || p.x > rect.right || p.y < rect.top || p.y > rect.bottom;
}

/**
 * Whether a press and its release close the modal: both must fall outside the
 * dialog, so a drag that starts inside (selecting text) keeps it open.
 */
export function backdropClick(rect: Rect, down: Point, up: Point): boolean {
  return outside(rect, down) && outside(rect, up);
}
