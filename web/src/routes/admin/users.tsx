import { useState } from 'react';
import { createRoute, redirect } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import { AlertTriangle, Loader2, ShieldCheck, UserX, X } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/cn';
import { ApiError } from '@/lib/api';
import { useSession } from '@/features/auth/session';
import { ALL_ROLES, roleLabel } from '@/features/auth/roles';
import { listUsers, updateUser, type AdminUser } from '@/features/users/api';

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
        {users ? (
          <p className="text-sm text-text-muted">
            {users.length} account{users.length === 1 ? '' : 's'}
          </p>
        ) : null}
      </header>

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
