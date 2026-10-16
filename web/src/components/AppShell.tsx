import type { ReactNode } from 'react';
import { Activity, LayoutDashboard, Stethoscope, Users } from 'lucide-react';
import { Link, useRouterState } from '@tanstack/react-router';
import { cn } from '@/lib/cn';
import { SyncStatusIndicator } from '@/components/SyncStatusIndicator';

/**
 * The application shell: a fixed light sidebar, a top bar, and a content area.
 *
 * Light chrome throughout — there is no dark sidebar, because a dark sidebar is
 * dark mode wearing a hat, and it fails the glare test in a bright consulting
 * room (docs/03-UI-PATTERNS.md §1.4).
 *
 * Navigation is filtered by role from Sprint 1. Until auth exists the list is
 * static, and the empty sections are labelled honestly rather than hidden.
 */

interface NavItem {
  to: string;
  label: string;
  Icon: typeof LayoutDashboard;
}

const NAV: NavItem[] = [
  { to: '/', label: 'Dashboard', Icon: LayoutDashboard },
  { to: '/patients', label: 'Patients', Icon: Users },
  { to: '/queue', label: 'Queue', Icon: Stethoscope },
  { to: '/sync', label: 'Sync status', Icon: Activity },
];

export function AppShell({ children }: { children: ReactNode }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });

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
          {NAV.map(({ to, label, Icon }) => {
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
          <p className="text-xs text-text-muted">
            Single-tenant deployment
            <br />
            <span className="font-mono">v0.1.0 · Sprint 0</span>
          </p>
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

        <main className="mx-auto w-full max-w-6xl flex-1 p-6">{children}</main>
      </div>
    </div>
  );
}
