/**
 * The eight roles, and the single table that maps them to navigation.
 *
 * ═══ WHY ONE TABLE ═══
 *
 * A menu filtered in three places drifts. Every item declares its roles here,
 * `navForRoles()` is the only function that filters, and adding a screen without
 * giving it roles is a type error rather than a silently public route.
 *
 * ═══ WHY THE CLIENT FILTERS AT ALL ═══
 *
 * Because the client works offline. A device with no network must still decide
 * whether to show the dispense screen. This is a USABILITY decision, not a
 * security one: every capability is re-checked server-side, because with a
 * replicated client an attacker controls everything in this file.
 */

export type AppRole =
  | 'front_desk'
  | 'nurse'
  | 'doctor'
  | 'lab_scientist'
  | 'pharmacist'
  | 'cashier'
  | 'accountant'
  | 'admin';

export const ALL_ROLES: AppRole[] = [
  'front_desk',
  'nurse',
  'doctor',
  'lab_scientist',
  'pharmacist',
  'cashier',
  'accountant',
  'admin',
];

export const ROLE_LABELS: Record<AppRole, string> = {
  front_desk: 'Front desk',
  nurse: 'Nurse',
  doctor: 'Doctor',
  lab_scientist: 'Lab scientist',
  pharmacist: 'Pharmacist',
  cashier: 'Cashier',
  accountant: 'Accountant',
  admin: 'Administrator',
};

/** Narrows an arbitrary string from the API to a known role. */
export function asRole(value: string): AppRole | null {
  return (ALL_ROLES as string[]).includes(value) ? (value as AppRole) : null;
}

export function roleLabel(value: string): string {
  const role = asRole(value);
  return role ? ROLE_LABELS[role] : value;
}

export function hasAnyRole(held: readonly string[], wanted: readonly AppRole[]): boolean {
  return held.some((role) => (wanted as readonly string[]).includes(role));
}
