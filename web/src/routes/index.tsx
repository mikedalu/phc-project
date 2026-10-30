import type { ReactNode } from 'react';
import { createRoute, Link, useNavigate } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import {
  ArrowRight,
  Banknote,
  BedDouble,
  ClipboardList,
  FileText,
  FlaskConical,
  History,
  LockKeyhole,
  Pill,
  Search,
  Stethoscope,
  UserPlus,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { useSession } from '@/features/auth/session';
import { hasAnyRole, type AppRole } from '@/features/auth/roles';
import { useCommandPalette } from '@/features/ui/commandPalette';
import { HOSPITAL } from '@/lib/branding';
import { cn } from '@/lib/cn';

/**
 * The shift dashboard.
 *
 * Every number is a live Dexie query, so the screen is correct with no network
 * and updates the instant a colleague saves on this workstation. The layout is
 * deliberately a cockpit rather than a welcome page: the first screen of a
 * shift answers "what needs doing now?" before it says hello.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: DashboardPage,
});

type RoutePath =
  | '/patients'
  | '/patients/new'
  | '/queue'
  | '/lab'
  | '/pharmacy'
  | '/billing'
  | '/ward'
  | '/referrals/new'
  | '/admin/activity';

interface RoleFocus {
  to: RoutePath;
  label: string;
  description: string;
  shortcut: string;
  icon: LucideIcon;
}

const QUEUE_ROLES: AppRole[] = ['front_desk', 'nurse', 'doctor', 'admin'];
const REGISTER_ROLES: AppRole[] = ['front_desk', 'admin'];

function DashboardPage() {
  const user = useSession((s) => s.user);
  const navigate = useNavigate();
  const openPalette = useCommandPalette((state) => state.openPalette);

  const queue = useLiveQuery(
    () => db.encounters.filter((e) => e.status === 'active').toArray(),
    [],
    [],
  );
  const patients = useLiveQuery(() => db.patients.count(), [], 0);
  const invoices = useLiveQuery(() => db.invoices.toArray(), [], []);
  const orders = useLiveQuery(() => db.lab_orders.toArray(), [], []);
  const prescriptions = useLiveQuery(() => db.prescriptions.toArray(), [], []);
  const admissions = useLiveQuery(() => db.admissions.toArray(), [], []);
  const charges = useLiveQuery(() => db.charges.toArray(), [], []);
  const allocations = useLiveQuery(() => db.payment_allocations.toArray(), [], []);

  const waitingVitals = queue.filter((e) => e.workflowStage === 'registered').length;
  const waitingDoctor = queue.filter((e) =>
    ['vitals_done', 'awaiting_doctor'].includes(e.workflowStage),
  ).length;
  const inConsultation = queue.filter((e) => e.workflowStage === 'in_consultation').length;
  const doctorQueue = waitingDoctor + inConsultation;

  const toCollect = orders.filter((o) => o.status === 'ordered').length;
  const awaitingResult = orders.filter((o) => o.status === 'collected').length;
  const pendingLabs = toCollect + awaitingResult;

  const toDispense = prescriptions.filter((p) => p.status === 'paid').length;
  const awaitingPayment = prescriptions.filter((p) => p.status === 'pending_payment').length;

  const inpatients = admissions.filter((a) => a.status === 'admitted').length;
  const unpaid = invoices.filter((i) => i.status === 'unpaid' || i.status === 'partially_paid').length;

  const settledChargeIDs = new Set(allocations.map((a) => a.chargeId));
  const outstandingMinor = charges
    .filter((c) => c.status === 'active' && !settledChargeIDs.has(c.id))
    .reduce((sum, c) => sum + c.amountMinor, 0);

  const roles = user?.roles ?? [];
  const can = (wanted: AppRole[]) => hasAnyRole(roles, wanted);
  const canQueue = can(QUEUE_ROLES);
  const canRegister = can(REGISTER_ROLES);
  const focus = focusFor(roles);
  const FocusIcon = focus.icon;

  const pulse = [
    {
      label: 'Active triage',
      value: waitingVitals,
      detail: 'registered, awaiting vitals',
      to: '/queue' as RoutePath,
      icon: Stethoscope,
      tone: 'spring' as const,
    },
    {
      label: 'Doctor queue',
      value: doctorQueue,
      detail: `${waitingDoctor} waiting · ${inConsultation} in consultation`,
      to: '/queue' as RoutePath,
      icon: ClipboardList,
      tone: 'info' as const,
    },
    {
      label: 'Pending labs',
      value: pendingLabs,
      detail: `${toCollect} to collect · ${awaitingResult} to result`,
      to: '/lab' as RoutePath,
      icon: FlaskConical,
      tone: 'warning' as const,
    },
    {
      label: 'Inpatients',
      value: inpatients,
      detail: 'currently admitted',
      to: '/ward' as RoutePath,
      icon: BedDouble,
      tone: 'neutral' as const,
    },
  ];

  const departments: Department[] = [
    {
      name: 'Reception',
      description: 'Quick intake & registration',
      to: '/patients/new',
      icon: ClipboardList,
      roles: REGISTER_ROLES,
    },
    {
      name: 'Outpatient',
      description: 'Triage, vitals & consultations',
      to: '/queue',
      icon: Stethoscope,
      roles: QUEUE_ROLES,
      badge: doctorQueue,
    },
    {
      name: 'Laboratory',
      description: 'Specimens & results',
      to: '/lab',
      icon: FlaskConical,
      roles: ['lab_scientist', 'doctor', 'nurse', 'admin'],
      badge: pendingLabs,
    },
    {
      name: 'Pharmacy',
      description: 'Dispense & verify',
      to: '/pharmacy',
      icon: Pill,
      roles: ['pharmacist', 'doctor', 'nurse', 'admin'],
      badge: toDispense,
    },
    {
      name: 'Cashier',
      description: 'Billing & POS',
      to: '/billing',
      icon: Banknote,
      roles: ['cashier', 'front_desk', 'accountant', 'admin'],
      badge: unpaid,
    },
    {
      name: 'Inpatient',
      description: 'Ward rounds & admissions',
      to: '/ward',
      icon: BedDouble,
      roles: ['nurse', 'doctor', 'admin'],
      badge: inpatients,
    },
    {
      name: 'Referrals',
      description: 'Digital referrals with QR',
      to: '/referrals/new',
      icon: FileText,
      roles: ['doctor', 'nurse', 'admin'],
    },
    {
      name: 'Admin logs',
      description: 'Audit trail & activity',
      to: '/admin/activity',
      icon: History,
      roles: ['admin'],
    },
  ];

  const secondary =
    focus.to !== '/patients/new' && canRegister
      ? {
          label: 'Register patient',
          icon: UserPlus,
          onClick: () => void navigate({ to: '/patients/new' }),
        }
      : focus.to !== '/queue' && canQueue
        ? {
            label: 'Start triage',
            icon: Stethoscope,
            onClick: () => void navigate({ to: '/queue' }),
          }
        : {
            label: 'Find a patient',
            icon: Users,
            onClick: () => void navigate({ to: '/patients' }),
          };
  const SecondaryIcon = secondary.icon;

  return (
    <div className="space-y-8">
      {/* ── Shift command banner ─────────────────────────────────────── */}
      <section className="relative overflow-hidden rounded-lg border border-spring-100 bg-linear-to-br from-spring-50 via-surface to-slate-100 p-6 shadow-card sm:p-8">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -right-20 -top-24 size-72 rounded-full bg-spring-200/40 blur-3xl"
        />
        <div className="relative">
          <p className="text-xs font-semibold uppercase tracking-wide text-spring-800">
            Welcome to {HOSPITAL.name}
          </p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-text sm:text-4xl">
            {greeting()}, {user?.displayName ?? 'there'}.
          </h1>
          <p className="mt-2 max-w-2xl text-sm text-text-muted">
            <span className="tnum">{patients}</span> patient{patients === 1 ? '' : 's'} on this
            workstation · <span className="tnum">{queue.length}</span> open visit
            {queue.length === 1 ? '' : 's'} · everything below is live from this device
          </p>

          <div className="mt-5 flex flex-col gap-3 xl:flex-row xl:items-center">
            <button
              type="button"
              onClick={openPalette}
              className={cn(
                'group flex min-h-11 w-full max-w-xl items-center gap-3 rounded-md',
                'border border-border-strong bg-surface px-4 text-left shadow-subtle',
                'transition-colors duration-100 hover:border-spring-600',
              )}
            >
              <Search aria-hidden="true" className="size-4 shrink-0 text-text-muted" />
              <span className="flex-1 truncate text-sm text-text-muted">
                Search patient ID, phone or name…
              </span>
              <kbd className="hidden rounded-xs border border-border bg-surface-sunken px-1.5 py-0.5 font-mono text-xs text-text-muted sm:inline">
                Ctrl K
              </kbd>
            </button>

            <div className="flex flex-wrap gap-2">
              <Button onClick={() => void navigate({ to: focus.to })}>
                <FocusIcon aria-hidden="true" className="size-4" />
                {focus.label}
                <kbd className="ml-1 hidden rounded-xs bg-white/15 px-1 font-mono text-xs text-white/90 lg:inline">
                  {focus.shortcut}
                </kbd>
              </Button>
              <Button variant="secondary" onClick={secondary.onClick}>
                <SecondaryIcon aria-hidden="true" className="size-4" />
                {secondary.label}
              </Button>
            </div>
          </div>

          <p className="mt-3 text-xs text-text-muted">{focus.description}</p>
        </div>
      </section>

      {/* ── Facility pulse ───────────────────────────────────────────── */}
      <section aria-labelledby="facility-pulse-heading">
        <SectionHeading
          id="facility-pulse-heading"
          title="Facility pulse"
          description="Live counts from this workstation's local replica"
          trailing={
            <span className="inline-flex items-center gap-1.5 text-xs text-text-muted">
              <span aria-hidden="true" className="size-1.5 rounded-full bg-spring-400" />
              Live on this device
            </span>
          }
        />
        <div className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {pulse.map((card) => (
            <PulseCard key={card.label} {...card} />
          ))}
        </div>
      </section>

      {/* ── Department workstations ──────────────────────────────────── */}
      <section aria-labelledby="departments-heading">
        <SectionHeading
          id="departments-heading"
          title="Department workstations"
          description="Fast access to the work this facility runs on"
        />
        <div className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {departments.map((department) => (
            <DepartmentTile
              key={department.name}
              department={department}
              allowed={can(department.roles)}
            />
          ))}
        </div>
      </section>

      {/* ── Queue of work that needs a human ─────────────────────────── */}
      <section aria-labelledby="attention-heading" className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <div className="rounded-lg border border-border bg-surface p-4 shadow-subtle">
          <SectionHeading
            id="attention-heading"
            title="Needs attention"
            description="The counts staff ask about at handover"
          />
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            <AttentionRow label="Specimens to collect" value={toCollect} to="/lab" />
            <AttentionRow label="Results to enter" value={awaitingResult} to="/lab" />
            <AttentionRow label="Prescriptions ready" value={toDispense} to="/pharmacy" />
            <AttentionRow label="Prescriptions unpaid" value={awaitingPayment} to="/pharmacy" />
            <AttentionRow label="Unpaid invoices" value={unpaid} to="/billing" />
            <AttentionRow label="Open visits" value={queue.length} to="/queue" />
          </div>
        </div>

        <div className="flex flex-col justify-between rounded-lg border border-border bg-surface p-4 shadow-subtle">
          <div>
            <p className="text-xs font-medium uppercase tracking-wide text-text-muted">
              Outstanding balance
            </p>
            <p className="mt-1 text-2xl font-semibold tnum text-text">
              {formatMoney(outstandingMinor)}
            </p>
            <p className="mt-1 text-xs text-text-muted">
              Active charges not yet settled
            </p>
          </div>
          <Link
            to="/billing"
            className="mt-4 inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline"
          >
            Open billing
            <ArrowRight aria-hidden="true" className="size-3.5" />
          </Link>
        </div>
      </section>
    </div>
  );
}

function SectionHeading({
  id,
  title,
  description,
  trailing,
}: {
  id: string;
  title: string;
  description: string;
  trailing?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-2">
      <div>
        <h2 id={id} className="text-lg font-semibold tracking-tight text-text">
          {title}
        </h2>
        <p className="mt-0.5 text-sm text-text-muted">{description}</p>
      </div>
      {trailing}
    </div>
  );
}

type PulseTone = 'spring' | 'info' | 'warning' | 'neutral';

const PULSE_TONES: Record<PulseTone, string> = {
  spring: 'bg-spring-50 text-spring-900 ring-spring-200',
  info: 'bg-info-soft text-info-fg ring-info-soft',
  warning: 'bg-warning-soft text-warning-fg ring-warning-soft',
  neutral: 'bg-surface-sunken text-text-subtle ring-border',
};

function PulseCard({
  label,
  value,
  detail,
  to,
  icon: Icon,
  tone,
}: {
  label: string;
  value: number;
  detail: string;
  to: RoutePath;
  icon: LucideIcon;
  tone: PulseTone;
}) {
  return (
    <Link
      to={to}
      className={cn(
        'group rounded-lg border border-border bg-surface p-4 shadow-subtle',
        'transition-colors duration-100 hover:border-spring-200 hover:bg-spring-50/30',
      )}
    >
      <span className="flex items-start justify-between gap-3">
        <span className="text-xs font-medium uppercase tracking-wide text-text-muted">{label}</span>
        <span className={cn('grid size-8 place-items-center rounded-sm ring-1', PULSE_TONES[tone])}>
          <Icon aria-hidden="true" className="size-4" />
        </span>
      </span>
      <span className="mt-2 block text-3xl font-semibold leading-tight tnum text-text">
        {value}
      </span>
      <span className="mt-1 block text-xs tnum text-text-muted">{detail}</span>
    </Link>
  );
}

interface Department {
  name: string;
  description: string;
  to: RoutePath;
  icon: LucideIcon;
  roles: AppRole[];
  badge?: number;
}

function DepartmentTile({
  department,
  allowed,
}: {
  department: Department;
  allowed: boolean;
}) {
  const Icon = department.icon;

  if (!allowed) {
    return (
      <div
        aria-disabled="true"
        title={`Your role cannot open ${department.name}.`}
        className="flex min-h-24 cursor-not-allowed flex-col justify-between gap-3 rounded-lg border border-border bg-surface/60 p-4 opacity-60"
      >
        <span className="flex items-start justify-between gap-2">
          <span className="grid size-9 place-items-center rounded-md bg-surface-sunken text-text-muted">
            <Icon aria-hidden="true" className="size-4" />
          </span>
          <LockKeyhole aria-hidden="true" className="size-3.5 text-text-muted" />
        </span>
        <span>
          <span className="block text-sm font-semibold text-text">{department.name}</span>
          <span className="mt-0.5 block text-xs text-text-muted">Restricted for your role</span>
        </span>
      </div>
    );
  }

  return (
    <Link
      to={department.to}
      className={cn(
        'group flex min-h-24 flex-col justify-between gap-3 rounded-lg border border-border bg-surface p-4',
        'shadow-subtle transition-colors duration-100 hover:border-spring-200 hover:bg-spring-50/30',
      )}
    >
      <span className="flex items-start justify-between gap-2">
        <span className="grid size-9 place-items-center rounded-md bg-spring-50 text-spring-800 ring-1 ring-spring-100">
          <Icon aria-hidden="true" className="size-4" />
        </span>
        {department.badge !== undefined ? (
          <span className="rounded-full bg-surface-sunken px-2 py-0.5 text-xs font-medium tnum text-text-muted">
            {department.badge}
          </span>
        ) : null}
      </span>
      <span>
        <span className="block text-sm font-semibold text-text group-hover:text-spring-900">
          {department.name}
        </span>
        <span className="mt-0.5 block text-xs text-text-muted">{department.description}</span>
      </span>
    </Link>
  );
}

function AttentionRow({
  label,
  value,
  to,
}: {
  label: string;
  value: number;
  to: RoutePath;
}) {
  return (
    <Link
      to={to}
      className="flex items-center justify-between gap-3 rounded-sm border border-border px-3 py-2 text-sm transition-colors duration-100 hover:bg-surface-sunken"
    >
      <span className="text-text-subtle">{label}</span>
      <span
        className={cn(
          'rounded-full px-2 py-0.5 text-xs font-medium tnum',
          value > 0 ? 'bg-warning-soft text-warning-fg' : 'bg-surface-sunken text-text-muted',
        )}
      >
        {value}
      </span>
    </Link>
  );
}

function focusFor(roles: readonly string[]): RoleFocus {
  if (roles.includes('front_desk')) {
    return {
      to: '/patients/new',
      label: 'Register a patient',
      description: 'Rapid check-in is ready. Press Alt+N from anywhere in the app.',
      shortcut: 'Alt+N',
      icon: UserPlus,
    };
  }

  if (roles.includes('nurse')) {
    return {
      to: '/queue',
      label: 'Start triage',
      description: 'The triage board is live. Press Alt+T from anywhere in the app.',
      shortcut: 'Alt+T',
      icon: Stethoscope,
    };
  }

  if (roles.includes('doctor')) {
    return {
      to: '/queue',
      label: 'Open your queue',
      description: 'Consultations waiting on this workstation are one click away.',
      shortcut: 'Alt+T',
      icon: Stethoscope,
    };
  }

  if (roles.includes('lab_scientist')) {
    return {
      to: '/lab',
      label: 'Open laboratory',
      description: 'Specimens to collect and results to enter are waiting.',
      shortcut: 'Alt+L',
      icon: FlaskConical,
    };
  }

  if (roles.includes('pharmacist')) {
    return {
      to: '/pharmacy',
      label: 'Open pharmacy',
      description: 'Prescriptions ready for dispensing are waiting.',
      shortcut: 'Alt+D',
      icon: Pill,
    };
  }

  if (roles.includes('cashier') || roles.includes('accountant')) {
    return {
      to: '/billing',
      label: 'Open billing',
      description: 'Unpaid invoices and payments are waiting at the cashier.',
      shortcut: 'Alt+P',
      icon: Banknote,
    };
  }

  if (roles.includes('admin')) {
    return {
      to: '/admin/activity',
      label: 'Review activity',
      description: 'The audit trail and facility-wide changes are available here.',
      shortcut: 'Alt+A',
      icon: History,
    };
  }

  return {
    to: '/patients',
    label: 'Find a patient',
    description: 'Search the local register by name, number or phone.',
    shortcut: 'Ctrl+K',
    icon: Users,
  };
}

function greeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 17) return 'Good afternoon';
  return 'Good evening';
}

function formatMoney(minor: number): string {
  return `₦${Math.floor(minor / 100).toLocaleString()}.${String(minor % 100).padStart(2, '0')}`;
}
