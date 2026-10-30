import { useEffect, useState, type ReactNode } from 'react';
import { KeyRound, LogOut, Search, ShieldAlert } from 'lucide-react';
import { Link, useRouter, useRouterState } from '@tanstack/react-router';
import { cn } from '@/lib/cn';
import { BrandMark } from '@/components/BrandMark';
import { CommandPalette } from '@/components/CommandPalette';
import { SyncStatusIndicator } from '@/components/SyncStatusIndicator';
import { Button } from '@/components/ui/button';
import { navForRoles } from '@/features/auth/nav';
import { useSession, signOut } from '@/features/auth/session';
import { graceMillisRemaining, offlinePolicy, writesBlocked } from '@/features/auth/offline';
import { roleLabel } from '@/features/auth/roles';
import { useCommandPalette } from '@/features/ui/commandPalette';
import { HOSPITAL, initials } from '@/lib/branding';

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
  const openPalette = useCommandPalette((state) => state.openPalette);
  const togglePalette = useCommandPalette((state) => state.togglePalette);
  const closePalette = useCommandPalette((state) => state.closePalette);

  const items = navForRoles(roles);
  const primaryRole = roleLabel(roles[0] ?? '');
  const mustChangePassword = useSession((s) => s.mustChangePassword);

  const handleSignOut = async () => {
    closePalette();
    await signOut();
    // Navigate explicitly: clearing the store does not re-run a route guard that
    // already passed. Without this the shell stays on screen with no session.
    await router.navigate({ to: '/login' });
  };

  // Ctrl/Cmd+K is captured at the window in the CAPTURE phase so a page-level
  // listener (the patient list also traditionally focuses its search box on
  // Ctrl+K) cannot open a second search surface underneath the palette.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target;
      const typing =
        target instanceof HTMLElement &&
        Boolean(target.closest('input, textarea, select, [contenteditable="true"]'));

      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        event.stopImmediatePropagation();
        togglePalette();
        return;
      }

      if (typing || !event.altKey) return;

      // Role workstations get a one-letter jump. These mirror the shortcut
      // legend on the dashboard; keep both lists in step when adding a screen.
      switch (event.key.toLowerCase()) {
        case 'n':
          void router.navigate({ to: '/patients/new' });
          break;
        case 't':
          void router.navigate({ to: '/queue' });
          break;
        case 'l':
          void router.navigate({ to: '/lab' });
          break;
        case 'd':
          void router.navigate({ to: '/pharmacy' });
          break;
        case 'p':
          void router.navigate({ to: '/billing' });
          break;
        case 'a':
          void router.navigate({ to: '/admin/activity' });
          break;
        default:
          return;
      }

      event.preventDefault();
    };

    window.addEventListener('keydown', onKeyDown, true);
    return () => window.removeEventListener('keydown', onKeyDown, true);
  }, [router, togglePalette]);

  // A palette opened on one screen should not survive a navigation triggered by
  // a link elsewhere in the shell.
  useEffect(() => {
    closePalette();
  }, [pathname, closePalette]);

  return (
    <div className="flex min-h-full bg-canvas">
      {/* ── Sidebar ─────────────────────────────────────────────── */}
      <aside
        data-print="hide"
        className="hidden w-60 shrink-0 border-r border-border bg-surface md:flex md:flex-col"
      >
        <div className="flex h-14 items-center gap-2.5 border-b border-border px-4">
          <BrandMark className="size-8" />
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold leading-tight text-text">{HOSPITAL.name}</p>
            <p className="truncate text-xs leading-tight text-text-muted">{HOSPITAL.tagline}</p>
          </div>
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

          <div className="mt-2 flex flex-wrap items-center gap-1">
            <Link
              to="/change-password"
              className="inline-flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs text-text-muted hover:bg-surface-sunken hover:text-text"
            >
              <KeyRound aria-hidden="true" className="size-3.5" />
              Change password
            </Link>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void handleSignOut()}
              className="h-7 px-2 text-xs"
            >
              <LogOut aria-hidden="true" className="size-3.5" />
              Sign out
            </Button>
          </div>
        </div>
      </aside>

      {/* ── Main column ─────────────────────────────────────────── */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header
          data-print="hide"
          className="sticky top-0 z-(--z-index-sticky) flex h-14 items-center justify-between gap-3 border-b border-border bg-surface/95 px-4 backdrop-blur-sm"
        >
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold text-text">{HOSPITAL.name}</p>
            <p className="hidden truncate text-xs text-text-muted sm:block">
              {HOSPITAL.tagline} · {HOSPITAL.systemName}
            </p>
          </div>

          <div className="flex shrink-0 items-center gap-2 sm:gap-3">
            <Button
              variant="secondary"
              size="sm"
              onClick={openPalette}
              aria-label="Global search (Ctrl K)"
              title="Global search (Ctrl K)"
              className="h-8 gap-2 px-2 sm:px-2.5"
            >
              <Search aria-hidden="true" className="size-3.5" />
              <span className="hidden lg:inline">Search</span>
              <kbd className="hidden rounded-xs border border-border bg-surface-sunken px-1 font-mono text-xs text-text-muted md:inline">
                Ctrl K
              </kbd>
            </Button>

            <SyncStatusIndicator />

            <div className="flex items-center gap-2 border-l border-border pl-2 sm:pl-3">
              <span className="hidden text-right lg:block">
                <span className="block max-w-40 truncate text-xs font-medium text-text">
                  {displayName}
                </span>
                <span className="block text-xs text-text-muted">{staffNumber}</span>
              </span>
              {primaryRole ? (
                <span className="hidden items-center rounded-full bg-spring-50 px-2 py-0.5 text-xs font-medium text-spring-900 ring-1 ring-spring-200 md:inline-flex">
                  {primaryRole}
                </span>
              ) : null}
              <span
                className="grid size-8 place-items-center rounded-full bg-spring-900 text-xs font-semibold text-white"
                title={displayName ? `${displayName} · ${primaryRole}` : undefined}
              >
                <span aria-hidden="true">{initials(displayName)}</span>
                <span className="sr-only">
                  Signed in as {displayName || 'staff'}
                  {primaryRole ? `, ${primaryRole}` : ''}
                </span>
              </span>
            </div>
          </div>
        </header>

        {mustChangePassword ? (
          <div
            role="status"
            className="flex flex-wrap items-center justify-between gap-2 border-b border-warning/30 bg-warning-soft px-4 py-2 text-sm text-warning-fg"
          >
            <span className="flex items-center gap-2">
              <KeyRound aria-hidden="true" className="size-4 shrink-0" />
              You are signed in with a temporary password.
            </span>
            <Link to="/change-password" className="font-medium underline underline-offset-2">
              Change it now
            </Link>
          </div>
        ) : null}

        <GraceBanner />

        <main className="mx-auto w-full max-w-[1600px] flex-1 p-6">{children}</main>
      </div>

      <CommandPalette />
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
