/**
 * Brand strings in one place.
 *
 * These are presentation constants, not facility configuration. The facility's
 * legal name, address and document-number prefix arrive from `/meta` and the
 * synced facility row; this module only names the product chrome so a rebrand
 * does not require a grep across every screen.
 */
export const HOSPITAL = {
  name: 'Spring Care Hospital',
  shortName: 'Spring Care',
  systemName: 'Clinical Records',
  tagline: 'Primary & Emergency Care',
} as const;

/** Two letters at most, for the compact staff avatar. */
export function initials(displayName: string): string {
  const parts = displayName.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return 'SC';

  const first = parts[0]?.[0] ?? '';
  const last = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? '') : '';
  return `${first}${last}`.toUpperCase() || 'SC';
}
