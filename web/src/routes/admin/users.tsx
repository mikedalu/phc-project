import { useState } from 'react';
import { createRoute, redirect } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import { AlertTriangle, Loader2, ShieldCheck, UserPlus, UserX, X } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/cn';
import { ApiError } from '@/lib/api';
import { useSession } from '@/features/auth/session';
import { ALL_ROLES, roleLabel } from '@/features/auth/roles';
import { createUser, listUsers, updateUser, type AdminUser } from '@/features/users/api';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';

/**
 * Staff administration.
 *
 * ═══ WHY THIS SCREEN IS ONLINE-ONLY ═══
 *
 * It is the one documented exception to local-first. A role change made against
 * a six-day-old offline copy of the staff list is a change made against
 * information that has already moved. Every other screen reads Dexie; this one
 * reads the server, and TanStack Query (not Dexie) is the correct tool here —
 * see CLAUDE.md §3.17–18.
 *
 * ═══ WHAT IT REFUSES TO DO ═══
 *
 * The server rejects an admin changing their own role or disabling themselves,
 * and refuses to disable the last active administrator. The UI hides those
 * actions too, but the block is server-side: hiding a button is a courtesy,
 * refusing the request is the control.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin/users',
  beforeLoad: () => {
    const roles = useSession.getState().user?.roles ?? [];
    if (!roles.includes('admin')) {
      throw redirect({ to: '/' });
    }
  },
  component: AdminUsersPage,
});

const usersKey = ['admin', 'users'] as const;

function AdminUsersPage() {
  const queryClient = useQueryClient();
  const myId = useSession((s) => s.user?.userId ?? '');
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);

  const { data: users, isPending, isError, error, refetch } = useQuery({
    queryKey: usersKey,
    queryFn: listUsers,
    // Staff changes are rare and the screen is opened deliberately; a short
    // stale time stops a focus-triggered refetch from flickering the table.
    staleTime: 30_000,
  });

  const mutation = useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: Parameters<typeof updateUser>[1] }) =>
      updateUser(id, patch),
    onMutate: ({ id }) => setPendingId(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: usersKey });
      toast.success('Account updated.');
    },
    onError: (err) => {
      toast.error(err instanceof ApiError ? err.message : 'Could not update the account.');
    },
    onSettled: () => setPendingId(null),
  });

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-text">Staff accounts</h1>
          <p className="mt-1 text-sm text-text-muted">
            Roles decide what a person can do. Every change here is recorded against your account.
          </p>
        </div>
        <div className="flex items-center gap-3">
          {users ? (
            <p className="text-sm text-text-muted">
              {users.length} account{users.length === 1 ? '' : 's'}
            </p>
          ) : null}
          <Button size="sm" onClick={() => setAdding(true)}>
            <UserPlus aria-hidden="true" className="size-4" />
            Add staff
          </Button>
        </div>
      </header>

      {adding ? (
        <AddStaffForm
          onCancel={() => setAdding(false)}
          onCreated={() => {
            setAdding(false);
            void queryClient.invalidateQueries({ queryKey: usersKey });
          }}
        />
      ) : null}

      {isError ? (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-md border border-danger/30 bg-danger-soft p-4 text-sm text-danger-fg"
        >
          <AlertTriangle aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
          <div>
            <p className="font-medium">Could not load staff accounts.</p>
            <p className="mt-1">
              {error instanceof ApiError ? error.message : 'Check the connection and try again.'}
            </p>
            <Button variant="secondary" size="sm" className="mt-3" onClick={() => void refetch()}>
              Retry
            </Button>
          </div>
        </div>
      ) : null}

      <div className="overflow-hidden rounded-md border border-border bg-surface">
        <table className="w-full border-collapse text-left">
          <caption className="sr-only">Staff accounts, their roles and their status</caption>
          <thead>
            <tr className="border-b border-border bg-surface-sunken text-xs font-medium uppercase tracking-wide text-text-muted">
              <th scope="col" className="px-4 py-2.5">Staff</th>
              <th scope="col" className="px-4 py-2.5">Roles</th>
              <th scope="col" className="px-4 py-2.5">Status</th>
              <th scope="col" className="px-4 py-2.5 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isPending ? (
              <tr>
                <td colSpan={4} className="px-4 py-10 text-center text-sm text-text-muted">
                  <Loader2 aria-hidden="true" className="mx-auto size-4 animate-spin" />
                  <span className="sr-only">Loading staff accounts</span>
                </td>
              </tr>
            ) : null}

            {users?.map((user) => (
              <UserRow
                key={user.id}
                user={user}
                isSelf={user.id === myId}
                busy={pendingId === user.id}
                onChange={(patch) => mutation.mutate({ id: user.id, patch })}
              />
            ))}

            {users && users.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-10 text-center text-sm text-text-muted">
                  No staff accounts yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <p className="text-xs text-text-muted">
        A person can hold more than one role — a doctor who also administers keeps both, and the
        most privileged one is used for the session.
      </p>
    </div>
  );
}

interface UserRowProps {
  user: AdminUser;
  isSelf: boolean;
  busy: boolean;
  onChange: (patch: Parameters<typeof updateUser>[1]) => void;
}

function UserRow({ user, isSelf, busy, onChange }: UserRowProps) {
  const missingRoles = ALL_ROLES.filter((role) => !user.roles.includes(role));
  const name = `${user.firstName} ${user.lastName}`.trim();

  return (
    <tr className={cn('border-b border-border last:border-b-0', busy && 'opacity-60')}>
      <td className="px-4 py-3 align-top">
        <p className="text-sm font-medium text-text">
          {name}
          {isSelf ? (
            <span className="ml-2 rounded-xs bg-primary-soft px-1.5 py-0.5 text-xs font-normal text-primary-fg">
              you
            </span>
          ) : null}
        </p>
        <p className="mt-0.5 font-mono text-xs text-text-muted">{user.staffNumber}</p>
        {user.lastLoginAt ? (
          <p className="mt-0.5 text-xs text-text-muted">
            Last sign-in {formatDistanceToNow(new Date(user.lastLoginAt), { addSuffix: true })}
          </p>
        ) : (
          <p className="mt-0.5 text-xs text-text-muted">Never signed in</p>
        )}
      </td>

      <td className="px-4 py-3 align-top">
        <ul className="flex flex-wrap gap-1.5">
          {user.roles.length === 0 ? (
            <li className="text-xs text-text-muted">No role — cannot sign in</li>
          ) : null}
          {user.roles.map((role) => (
            <li key={role}>
              <span className="inline-flex items-center gap-1 rounded-xs bg-surface-sunken px-1.5 py-0.5 text-xs text-text-subtle">
                {roleLabel(role)}
                {!isSelf ? (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => onChange({ role, reason: 'role withdrawn by administrator' })}
                    aria-label={`Remove the ${roleLabel(role)} role from ${name}`}
                    className="rounded-xs text-text-muted hover:text-danger disabled:opacity-50"
                  >
                    <X aria-hidden="true" className="size-3" />
                  </button>
                ) : null}
              </span>
            </li>
          ))}
        </ul>

        {!isSelf && missingRoles.length > 0 ? (
          <label className="mt-2 flex items-center gap-1.5 text-xs text-text-muted">
            <span className="sr-only">Add a role to {name}</span>
            <select
              value=""
              disabled={busy}
              onChange={(event) => {
                if (event.target.value) {
                  onChange({ role: event.target.value, reason: 'role granted by administrator' });
                }
              }}
              className="h-7 rounded-xs border border-border-strong bg-surface px-1.5 text-xs text-text"
            >
              <option value="">Add role…</option>
              {missingRoles.map((role) => (
                <option key={role} value={role}>
                  {roleLabel(role)}
                </option>
              ))}
            </select>
          </label>
        ) : null}
      </td>

      <td className="px-4 py-3 align-top">
        <span
          className={cn(
            'inline-flex items-center gap-1 rounded-xs px-1.5 py-0.5 text-xs font-medium',
            user.isActive ? 'bg-success-soft text-success-fg' : 'bg-danger-soft text-danger-fg',
          )}
        >
          {user.isActive ? (
            <>
              <ShieldCheck aria-hidden="true" className="size-3" />
              Active
            </>
          ) : (
            <>
              <UserX aria-hidden="true" className="size-3" />
              Disabled
            </>
          )}
        </span>
        {user.mustChangePassword ? (
          <p className="mt-1 text-xs text-text-muted">Must change password</p>
        ) : null}
        {user.lockedUntil && new Date(user.lockedUntil) > new Date() ? (
          <p className="mt-1 text-xs text-danger-fg">Locked after failed sign-ins</p>
        ) : null}
      </td>

      <td className="px-4 py-3 text-right align-top">
        {isSelf ? (
          <span className="text-xs text-text-muted">Managed by another admin</span>
        ) : (
          <Button
            variant={user.isActive ? 'secondary' : 'primary'}
            size="sm"
            disabled={busy}
            onClick={() =>
              onChange({
                isActive: !user.isActive,
                reason: user.isActive
                  ? 'account disabled by administrator'
                  : 'account re-enabled by administrator',
              })
            }
          >
            {busy ? <Loader2 aria-hidden="true" className="size-3.5 animate-spin" /> : null}
            {user.isActive ? 'Disable' : 'Enable'}
          </Button>
        )}
      </td>
    </tr>
  );
}


/**
 * Add a member of staff.
 *
 * The temporary password is shown to the administrator once and the account is
 * forced to change it at first sign-in — so the administrator never keeps a
 * working credential for a colleague.
 */
function AddStaffForm({ onCancel, onCreated }: { onCancel: () => void; onCreated: () => void }) {
  const [staffNumber, setStaffNumber] = useState('');
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [role, setRole] = useState<string>('nurse');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const created = await createUser({ staffNumber, firstName, lastName, role, password });
      toast.success(
        `${created.firstName} ${created.lastName} added. Temporary password: ${password} — they must change it when they first sign in.`,
        { duration: 15000 },
      );
      onCreated();
    } catch (cause) {
      setError(friendlyError(cause));
      setBusy(false);
    }
  };

  return (
    <section className="rounded-md border border-border bg-surface p-4">
      <h2 className="text-sm font-medium text-text">New staff account</h2>
      <p className="mt-1 text-xs text-text-muted">
        You choose a temporary password. They will be asked to change it the first time they sign
        in.
      </p>

      {error ? (
        <p role="alert" className="mt-3 rounded-sm border border-danger/30 bg-danger-soft p-2 text-sm text-danger-fg">
          {error}
        </p>
      ) : null}

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <FormField label="Staff number" required>
          {(field) => (
            <Input
              id={field.id}
              value={staffNumber}
              onChange={(e) => setStaffNumber(e.target.value)}
              placeholder="DEMO/STF/0012"
            />
          )}
        </FormField>
        <FormField label="Role">
          {(field) => (
            <select
              id={field.id}
              value={role}
              onChange={(e) => setRole(e.target.value)}
              className="h-9 w-full rounded-sm border border-border-strong bg-surface px-3 text-md text-text"
            >
              {ALL_ROLES.map((r) => (
                <option key={r} value={r}>
                  {roleLabel(r)}
                </option>
              ))}
            </select>
          )}
        </FormField>
        <FormField label="First name" required>
          {(field) => (
            <Input id={field.id} value={firstName} onChange={(e) => setFirstName(e.target.value)} />
          )}
        </FormField>
        <FormField label="Surname" required>
          {(field) => (
            <Input id={field.id} value={lastName} onChange={(e) => setLastName(e.target.value)} />
          )}
        </FormField>
        <FormField label="Temporary password" required hint="At least 10 characters">
          {(field) => (
            <Input
              id={field.id}
              type="text"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="They change this at first sign-in"
            />
          )}
        </FormField>
      </div>

      <div className="mt-3 flex gap-2">
        <Button
          size="sm"
          onClick={() => void submit()}
          disabled={busy || !staffNumber.trim() || !firstName.trim() || password.length < 10}
        >
          {busy ? <Loader2 aria-hidden="true" className="size-4 animate-spin" /> : null}
          Create account
        </Button>
        <Button size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </section>
  );
}

/** Turns an API failure into something a clerk can act on. */
function friendlyError(cause: unknown): string {
  if (cause instanceof ApiError) {
    if (cause.status === 409) return cause.message;
    if (cause.status === 403) return 'Only an administrator can add staff.';
    if (cause.status === 0) return 'The server could not be reached. Check the connection and try again.';
    return cause.message;
  }
  return 'Something went wrong. Try again, and tell an administrator if it keeps happening.';
}
