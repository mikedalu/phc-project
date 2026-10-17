import { useEffect, useState, type ReactNode } from 'react';
import { LogOut, ShieldAlert } from 'lucide-react';
import { Link, useRouter, useRouterState } from '@tanstack/react-router';
import { cn } from '@/lib/cn';
import { SyncStatusIndicator } from '@/components/SyncStatusIndicator';
import { Button } from '@/components/ui/button';
import { navForRoles } from '@/features/auth/nav';
import { useSession, signOut } from '@/features/auth/session';
import { graceMillisRemaining, offlinePolicy, writesBlocked } from '@/features/auth/offline';
import { roleLabel } from '@/features/auth/roles';

/**
 * The application shell: a fixed light sidebar, a top bar, and a content area.
 *
 * Light chrome throughout — there is no dark sidebar, because a dark sidebar is
 * dark mode wearing a hat, and it fails the glare test in a bright consulting
 * room (docs/03-UI-PATTERNS.md §1.4).
 *
 * ═══ NAVIGATION IS DERIVED, NOT DECLARED ═══
 *
 * The items come from `ROLE_NAV` filtered by the signed-in user's roles. This is
 * a usability decision: it keeps a cashier from clicking into a triage board
 * they cannot use. It is NOT a security boundary — with a replicated client an
 * attacker controls the frontend entirely, which is why every capability is
 * re-checked server-side and why route guards are a second convenience, not the
 * first line.
 */
export function AppShell({ children }: { children: ReactNode }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const roles = useSession((s) => s.user?.roles ?? []);
  const displayName = useSession((s) => s.user?.displayName ?? '');
  const staffNumber = useSession((s) => s.user?.staffNumber ?? '');
  const router = useRouter();

  const items = navForRoles(roles);

  const handleSignOut = async () => {
    await signOut();
    // Navigate explicitly: clearing the store does not re-run a route guard that
    // already passed. Without this the shell stays on screen with no session.
    await router.navigate({ to: '/login' });
  };

  return (
    <div className="flex min-h-full bg-canvas">
      {/* ── Sidebar ─────────────────────────────────────────────── */}
      <aside
        data-print="hide"
        className="hidden w-56 shrink-0 border-r border-border bg-surface md:flex md:flex-col"
      >
        <div className="flex h-14 items-center gap-2 border-b border-border px-4">
          <span className="grid size-7 place-items-center rounded-sm bg-primary text-xs font-bold text-text-inverse">
            PH
          </span>
          <span className="text-sm font-semibold text-text">PHC Records</span>
        </div>

        <nav className="flex-1 space-y-0.5 p-2" aria-label="Main">
          {items.map(({ to, label, icon: Icon }) => {
            const active = to === '/' ? pathname === '/' : pathname.startsWith(to);
            return (
              <Link
                key={to}
                to={to}
                aria-current={active ? 'page' : undefined}
                className={cn(
                  'flex items-center gap-2.5 rounded-sm px-2.5 py-2',
                  'text-sm transition-colors duration-100',
                  active
                    ? 'bg-primary-soft font-medium text-primary-fg'
                    : 'text-text-subtle hover:bg-surface-sunken hover:text-text',
                )}
              >
                <Icon aria-hidden="true" className="size-4 shrink-0" />
                {label}
              </Link>
            );
          })}
        </nav>

        <div className="border-t border-border p-3">
          <p className="truncate text-sm font-medium text-text">{displayName}</p>
          <p className="truncate text-xs text-text-muted">
            {staffNumber}
            {roles.length > 0 ? ` · ${roleLabel(roles[0] ?? '')}` : ''}
          </p>

          <Button
            variant="ghost"
            size="sm"
            onClick={() => void handleSignOut()}
            className="mt-2 -ml-2 h-7 px-2 text-xs"
          >
            <LogOut aria-hidden="true" className="size-3.5" />
            Sign out
          </Button>
        </div>
      </aside>

      {/* ── Main column ─────────────────────────────────────────── */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header
          data-print="hide"
          className="sticky top-0 z-(--z-index-sticky) flex h-14 items-center justify-between gap-4 border-b border-border bg-surface px-4"
        >
          <div className="min-w-0">
            <p className="truncate text-sm font-medium text-text">
              Demo Primary Health Centre
            </p>
            <p className="truncate text-xs text-text-muted">
              Offline-capable patient records
            </p>
          </div>

          <SyncStatusIndicator />
        </header>

        <GraceBanner />

        <main className="mx-auto w-full max-w-6xl flex-1 p-6">{children}</main>
      </div>
    </div>
  );
}

/**
 * Warns before, and blocks after, the offline window closes.
 *
 * The deadline is server-issued, so a device whose clock was rolled back only
 * sees this EARLIER than reality, never later. Two distinct messages because the
 * two states call for different behaviour: "two days left" is a nudge, "expired"
 * is an instruction.
 */
function GraceBanner() {
  const offlineGraceExpiresAt = useSession((s) => s.offlineGraceExpiresAt);
  const [, tick] = useState(0);

  // Re-render once a minute: the deadline passes while the tab sits open, and a
  // banner that only updates on the next navigation is a banner that lies.
  useEffect(() => {
    const timer = window.setInterval(() => tick((n) => n + 1), 60_000);
    return () => window.clearInterval(timer);
  }, []);

  if (!offlineGraceExpiresAt) return null;

  const remaining = graceMillisRemaining();
  const days = remaining / 86_400_000;

  // Above a day left there is nothing useful to say.
  if (days > 1) return null;

  const expired = remaining <= 0;
  const blocked = writesBlocked();

  if (expired) {
    return (
      <div
        role="alert"
        className="flex items-start gap-2 border-b border-danger/30 bg-danger-soft px-4 py-2.5 text-sm text-danger-fg"
      >
        <ShieldAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
        <span>
          {blocked
            ? 'Offline access has expired. Records are readable, but new entries are blocked. Connect this device to the internet and sign in again.'
            : `Offline access has expired. This deployment is set to ${offlinePolicy()}, so new entries are still allowed — connect and sign in to clear this warning.`}
        </span>
      </div>
    );
  }

  const hours = Math.max(1, Math.round(remaining / 3_600_000));

  return (
    <div
      role="status"
      className="flex items-start gap-2 border-b border-warning/30 bg-warning-soft px-4 py-2.5 text-sm text-warning-fg"
    >
      <ShieldAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <span>
        This device has {hours < 24 ? `${hours} hour${hours === 1 ? '' : 's'}` : 'one day'} of
        offline access left. Connect it to the internet and sign in to renew.
      </span>
    </div>
  );
}
