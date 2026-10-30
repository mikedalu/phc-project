import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import {
  ArrowRight,
  Banknote,
  BedDouble,
  FileText,
  FlaskConical,
  History,
  KeyRound,
  LayoutDashboard,
  Pill,
  Search,
  Stethoscope,
  TriangleAlert,
  UserCog,
  UserPlus,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { db } from '@/db';
import type { PatientRow } from '@/db/types';
import { cn } from '@/lib/cn';
import { hasAnyRole, type AppRole } from '@/features/auth/roles';
import { useSession } from '@/features/auth/session';
import { useCommandPalette } from '@/features/ui/commandPalette';

/**
 * The global command palette (Ctrl/Cmd+K).
 *
 * It is the single search surface for the front desk: one keystroke finds a
 * patient by name, number or phone, and typing `>` turns the same box into
 * department navigation. Everything runs against Dexie, so it behaves exactly
 * the same with the uplink down.
 *
 * Keyboard model:
 *   ↑ / ↓   move the highlight
 *   Enter   open the highlighted result
 *   Escape  close
 */

type Destination =
  | '/'
  | '/patients/new'
  | '/patients'
  | '/queue'
  | '/lab'
  | '/pharmacy'
  | '/billing'
  | '/ward'
  | '/referrals/new'
  | '/admin/users'
  | '/admin/activity'
  | '/change-password';

interface Command {
  id: string;
  label: string;
  description: string;
  keywords?: string;
  to: Destination;
  icon: LucideIcon;
  roles?: AppRole[];
  emergency?: boolean;
}

const COMMANDS: Command[] = [
  {
    id: 'dashboard',
    label: 'Dashboard',
    description: 'Shift overview and facility pulse',
    keywords: 'home overview',
    to: '/',
    icon: LayoutDashboard,
  },
  {
    id: 'change-password',
    label: 'Change password',
    description: 'Update your own sign-in password',
    keywords: 'security account credential',
    to: '/change-password',
    icon: KeyRound,
  },
  {
    id: 'emergency',
    label: 'Emergency check-in',
    description: 'Register an urgent patient now',
    keywords: 'emergency urgent triage',
    to: '/patients/new',
    icon: TriangleAlert,
    roles: ['front_desk', 'nurse', 'doctor', 'admin'],
    emergency: true,
  },
  {
    id: 'new-patient',
    label: 'Register a patient',
    description: 'Quick intake · Alt+N',
    keywords: 'new intake registration',
    to: '/patients/new',
    icon: UserPlus,
    roles: ['front_desk', 'nurse', 'doctor', 'admin'],
  },
  {
    id: 'patients',
    label: 'Find a patient',
    description: 'Search the local register',
    keywords: 'patient list records',
    to: '/patients',
    icon: Users,
  },
  {
    id: 'queue',
    label: 'Open the queue',
    description: 'Triage, vitals and consultations',
    keywords: 'triage vitals doctor',
    to: '/queue',
    icon: Stethoscope,
    roles: ['front_desk', 'nurse', 'doctor', 'admin'],
  },
  {
    id: 'lab',
    label: 'Laboratory',
    description: 'Specimens and results',
    keywords: 'lab specimen result',
    to: '/lab',
    icon: FlaskConical,
    roles: ['lab_scientist', 'doctor', 'nurse', 'admin'],
  },
  {
    id: 'pharmacy',
    label: 'Pharmacy',
    description: 'Dispense and verify prescriptions',
    keywords: 'drug dispense medicine',
    to: '/pharmacy',
    icon: Pill,
    roles: ['pharmacist', 'doctor', 'nurse', 'admin'],
  },
  {
    id: 'billing',
    label: 'Cashier & billing',
    description: 'Invoices and payments',
    keywords: 'cash payment invoice money',
    to: '/billing',
    icon: Banknote,
    roles: ['cashier', 'front_desk', 'accountant', 'admin'],
  },
  {
    id: 'ward',
    label: 'Ward round',
    description: 'Inpatient board',
    keywords: 'admission inpatient bed',
    to: '/ward',
    icon: BedDouble,
    roles: ['nurse', 'doctor', 'admin'],
  },
  {
    id: 'referrals',
    label: 'Issue a referral',
    description: 'Digital referral with QR code',
    keywords: 'refer transfer',
    to: '/referrals/new',
    icon: FileText,
    roles: ['doctor', 'nurse', 'admin'],
  },
  {
    id: 'admin-users',
    label: 'Staff accounts',
    description: 'Roles and registered devices',
    keywords: 'admin users staff',
    to: '/admin/users',
    icon: UserCog,
    roles: ['admin'],
  },
  {
    id: 'admin-activity',
    label: 'Audit activity',
    description: 'Change log and audit trail',
    keywords: 'admin audit history',
    to: '/admin/activity',
    icon: History,
    roles: ['admin'],
  },
];

interface PaletteItem {
  key: string;
  kind: 'patient' | 'command';
  label: string;
  description: string;
  icon: LucideIcon;
  run: () => void;
}

export function CommandPalette() {
  const open = useCommandPalette((state) => state.open);
  const closePalette = useCommandPalette((state) => state.closePalette);
  const roles = useSession((state) => state.user?.roles ?? []);
  const navigate = useNavigate();

  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const returnFocusRef = useRef<HTMLElement | null>(null);

  // Reset the query each time the palette opens, and keep the page behind it
  // from scrolling. Restoring the previous value matters because the shell owns
  // the scroll container on patient list pages.
  useEffect(() => {
    if (!open) return;

    setQuery('');
    setActive(0);
    returnFocusRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;

    const frame = window.requestAnimationFrame(() => inputRef.current?.focus());
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    return () => {
      window.cancelAnimationFrame(frame);
      document.body.style.overflow = previousOverflow;
      returnFocusRef.current?.focus();
    };
  }, [open]);

  // Escape is handled here as well as on the input: if focus ever leaves the
  // input (a mouse click on a result), the dialog must still be dismissible.
  useEffect(() => {
    if (!open) return;

    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') closePalette();
    };

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [open, closePalette]);

  // Keep the keyboard highlight visible when the result list is longer than the
  // scroll container. `nearest` avoids yanking the page behind the dialog.
  useEffect(() => {
    if (!open) return;
    document.getElementById(`palette-item-${active}`)?.scrollIntoView({ block: 'nearest' });
  }, [open, active, query]);

  const trimmed = query.trim();
  const scope = trimmed.startsWith('!')
    ? 'emergency'
    : trimmed.startsWith('>')
      ? 'commands'
      : 'search';
  const needle = trimmed.replace(/^[>!]/, '').trim().toLowerCase();

  const availableCommands = useMemo(
    () => COMMANDS.filter((command) => !command.roles || hasAnyRole(roles, command.roles)),
    [roles],
  );

  const commandMatches = useMemo(() => {
    let list = availableCommands;

    if (scope === 'emergency') {
      list = list.filter((command) => command.emergency);
    }

    if (needle) {
      list = list.filter((command) =>
        `${command.label} ${command.description} ${command.keywords ?? ''}`
          .toLowerCase()
          .includes(needle),
      );
    }

    return list;
  }, [availableCommands, scope, needle]);

  // Read the local register once when the palette opens, then filter in memory
  // as the user types. Re-reading the whole table on every keystroke would make
  // a large register stutter on the low-power laptops this runs on. Dexie still
  // pushes an update if another tab registers a patient while the palette is up.
  const allPatients = useLiveQuery(
    async () => {
      if (!open) return [];

      const rows = await db.patients.toArray();
      return rows.filter((patient) => !patient.deletedAt);
    },
    [open],
    [],
  );

  const patients = useMemo(() => {
    if (scope !== 'search' || needle.length === 0) return [];

    return allPatients
      .filter((patient) =>
        [
          patient.firstName,
          patient.middleName ?? '',
          patient.surname,
          patient.patientNumber,
          patient.id,
          patient.phone ?? '',
        ]
          .join(' ')
          .toLowerCase()
          .includes(needle),
      )
      .sort((a, b) =>
        `${a.surname} ${a.firstName}`.localeCompare(`${b.surname} ${b.firstName}`),
      )
      .slice(0, 8);
  }, [allPatients, scope, needle]);

  const runDestination = useCallback(
    (to: Destination) => {
      closePalette();
      void navigate({ to });
    },
    [closePalette, navigate],
  );

  const openPatient = useCallback(
    (id: string) => {
      closePalette();
      void navigate({ to: '/patients/$id', params: { id } });
    },
    [closePalette, navigate],
  );

  const patientItems = useMemo<PaletteItem[]>(
    () =>
      patients.map((patient) => ({
        key: `patient-${patient.id}`,
        kind: 'patient',
        label: `${patient.surname}, ${patient.firstName}`,
        description: patientMeta(patient),
        icon: Users,
        run: () => openPatient(patient.id),
      })),
    [patients, openPatient],
  );

  const commandItems = useMemo<PaletteItem[]>(
    () =>
      commandMatches.map((command) => ({
        key: `command-${command.id}`,
        kind: 'command',
        label: command.label,
        description: command.description,
        icon: command.icon,
        run: () => runDestination(command.to),
      })),
    [commandMatches, runDestination],
  );

  const items = useMemo(
    () => [...patientItems, ...commandItems],
    [patientItems, commandItems],
  );

  if (!open) return null;

  const onInputKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActive((index) => (items.length === 0 ? 0 : (index + 1) % items.length));
      return;
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActive((index) =>
        items.length === 0 ? 0 : (index - 1 + items.length) % items.length,
      );
      return;
    }

    if (event.key === 'Enter') {
      event.preventDefault();
      items[active]?.run();
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      closePalette();
    }
  };

  return (
    <div
      className="fixed inset-0 z-(--z-index-modal-backdrop) flex items-start justify-center bg-slate-950/40 px-4 pt-[10vh] backdrop-blur-[2px]"
      onMouseDown={closePalette}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Search patients and navigate"
        className="w-full max-w-xl overflow-hidden rounded-lg border border-border bg-surface shadow-modal"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="flex items-center gap-3 border-b border-border px-4">
          <Search aria-hidden="true" className="size-4 shrink-0 text-text-muted" />
          <input
            ref={inputRef}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setActive(0);
            }}
            onKeyDown={onInputKeyDown}
            role="combobox"
            aria-expanded="true"
            aria-controls="command-palette-results"
            aria-autocomplete="list"
            aria-activedescendant={items[active] ? `palette-item-${active}` : undefined}
            placeholder="Search patients, or type > for departments…"
            className="h-14 min-w-0 flex-1 bg-transparent text-md text-text outline-none placeholder:text-text-muted"
          />
          <kbd className="hidden rounded-xs border border-border bg-surface-sunken px-1.5 py-0.5 font-mono text-xs text-text-muted sm:inline">
            ESC
          </kbd>
        </div>

        <div
          id="command-palette-results"
          role="listbox"
          aria-label="Results"
          className="max-h-[24rem] overflow-y-auto p-2"
        >
          {items.length === 0 ? (
            <p className="px-3 py-8 text-center text-sm text-text-muted">
              No matches. Try a surname, patient number or phone.
            </p>
          ) : null}

          {patientItems.length > 0 ? (
            <PaletteSection
              label="Patients"
              items={patientItems}
              offset={0}
              active={active}
              onHover={setActive}
            />
          ) : null}

          {commandItems.length > 0 ? (
            <PaletteSection
              label={scope === 'emergency' ? 'Emergency' : 'Go to'}
              items={commandItems}
              offset={patientItems.length}
              active={active}
              onHover={setActive}
            />
          ) : null}
        </div>

        <div className="flex items-center gap-4 border-t border-border bg-surface-sunken px-4 py-2 text-xs text-text-muted">
          <span className="flex items-center gap-1">
            <kbd className="rounded-xs border border-border bg-surface px-1 font-mono">↑</kbd>
            <kbd className="rounded-xs border border-border bg-surface px-1 font-mono">↓</kbd>
            to navigate
          </span>
          <span className="flex items-center gap-1">
            <kbd className="rounded-xs border border-border bg-surface px-1 font-mono">↵</kbd>
            to open
          </span>
          <span className="ml-auto hidden sm:inline">Type &gt; for departments</span>
        </div>
      </div>
    </div>
  );
}

function PaletteSection({
  label,
  items,
  offset,
  active,
  onHover,
}: {
  label: string;
  items: PaletteItem[];
  offset: number;
  active: number;
  onHover: (index: number) => void;
}) {
  return (
    <div className="mb-1 last:mb-0">
      <p className="px-2 py-1.5 text-xs font-medium uppercase tracking-wide text-text-muted">
        {label}
      </p>
      {items.map((item, index) => {
        const itemIndex = offset + index;
        const isActive = itemIndex === active;
        const Icon = item.icon;

        return (
          <button
            key={item.key}
            id={`palette-item-${itemIndex}`}
            type="button"
            role="option"
            aria-selected={isActive}
            onMouseEnter={() => onHover(itemIndex)}
            onClick={item.run}
            className={cn(
              'flex w-full items-center gap-3 rounded-sm px-3 py-2.5 text-left',
              'transition-colors duration-100',
              isActive
                ? 'bg-spring-50 text-spring-900'
                : 'text-text hover:bg-surface-sunken',
            )}
          >
            <span
              className={cn(
                'grid size-8 shrink-0 place-items-center rounded-sm',
                isActive
                  ? 'bg-spring-100 text-spring-800'
                  : 'bg-surface-sunken text-text-muted',
              )}
            >
              <Icon aria-hidden="true" className="size-4" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{item.label}</span>
              <span className="block truncate text-xs text-text-muted">{item.description}</span>
            </span>
            {item.kind === 'patient' ? (
              <ArrowRight aria-hidden="true" className="size-3.5 shrink-0 text-text-muted" />
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

function patientMeta(patient: PatientRow): string {
  const age =
    patient.ageYears !== null
      ? `${patient.ageYears}y`
      : patient.dateOfBirth
        ? patient.dateOfBirth.slice(0, 10)
        : 'Age unknown';

  const status = patient.isDeceased ? 'Deceased' : patient.isActive ? null : 'Inactive';

  return [patient.patientNumber, age, patient.phone, status]
    .filter((value): value is string => Boolean(value))
    .join(' · ');
}
