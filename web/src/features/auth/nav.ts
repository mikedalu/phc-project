import {
  Activity,
  LayoutDashboard,
  Stethoscope,
  Banknote,
  BedDouble,
  Pill,
  FlaskConical,
  Home,
  UserCog,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { ALL_ROLES, hasAnyRole, type AppRole } from './roles';

/**
 * The one table that decides what appears in the sidebar.
 *
 * `roles` is required, deliberately. A new screen that forgets to declare its
 * audience will not compile — which is the only reliable way to prevent a route
 * appearing for everybody by accident.
 */
export interface NavEntry {
  to: string;
  label: string;
  icon: LucideIcon;
  roles: AppRole[];
  /** Set for screens that are still stubs, so the UI can say so honestly. */
  plannedSprint?: string;
}

export const ROLE_NAV: NavEntry[] = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, roles: ALL_ROLES },
  {
    to: '/patients',
    label: 'Patients',
    icon: Users,
    // Everyone who might need to find a patient, which is everyone.
    roles: ALL_ROLES,
    plannedSprint: 'Sprint 3',
  },
  {
    to: '/queue',
    label: 'Queue',
    icon: Stethoscope,
    // Front desk starts an encounter; nurses and doctors move it along.
    // A cashier or pharmacist has no business on the triage board.
    roles: ['front_desk', 'nurse', 'doctor', 'admin'],
    plannedSprint: 'Sprint 4',
  },
  {
    to: '/households',
    label: 'Households',
    icon: Home,
    // A family is the billing party, so the money roles need it too.
    roles: ['front_desk', 'nurse', 'doctor', 'cashier', 'accountant', 'admin'],
  },
  {
    to: '/lab',
    label: 'Laboratory',
    icon: FlaskConical,
    roles: ['lab_scientist', 'doctor', 'nurse', 'admin'],
  },
  {
    to: '/ward',
    label: 'Ward',
    icon: BedDouble,
    roles: ['nurse', 'doctor', 'admin'],
  },
  {
    to: '/pharmacy',
    label: 'Pharmacy',
    icon: Pill,
    roles: ['pharmacist', 'doctor', 'nurse', 'admin'],
  },
  {
    to: '/billing',
    label: 'Billing',
    icon: Banknote,
    roles: ['cashier', 'front_desk', 'accountant', 'admin'],
  },
  { to: '/sync', label: 'Sync status', icon: Activity, roles: ALL_ROLES },
  { to: '/admin/users', label: 'Staff accounts', icon: UserCog, roles: ['admin'] },
  {
    to: '/dev/sync',
    label: 'Sync probe',
    icon: FlaskConical,
    roles: ['admin'],
    plannedSprint: 'Sprint 2',
  },
];

/** The items the holder of `roles` may see, in table order. */
export function navForRoles(roles: readonly string[]): NavEntry[] {
  return ROLE_NAV.filter((entry) => hasAnyRole(roles, entry.roles));
}
