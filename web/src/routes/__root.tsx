import { createRootRoute, Outlet, redirect, useRouterState } from '@tanstack/react-router';
import { AppShell } from '@/components/AppShell';
import { useSession } from '@/features/auth/session';

/**
 * The root route.
 *
 * Deliberately NOT using TanStack Router's file-based generation: the route tree
 * is small and explicit here, which keeps the wiring readable and avoids a
 * generated file that must be regenerated before the project will typecheck.
 *
 * ═══ THE GUARD ═══
 *
 * Every route needs a session except /login. Reading the store synchronously is
 * safe because bootstrap awaits `hydrateSession()` BEFORE rendering the router —
 * so `status` is never 'loading' here. If that ordering is ever broken, the
 * symptom is a user being bounced back to /login on every reload.
 */
export const Route = createRootRoute({
  beforeLoad: ({ location }) => {
    if (location.pathname === '/login') return;

    if (useSession.getState().status !== 'authenticated') {
      throw redirect({ to: '/login' });
    }
  },
  component: RootLayout,
});

function RootLayout() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  // The login screen is a full-page canvas. A sidebar full of clinical
  // navigation next to a sign-in form invites clicking things that require the
  // session you do not have yet.
  if (pathname === '/login') {
    return <Outlet />;
  }

  return (
    <AppShell>
      {/* Renders the matched child route. Every route file exports `Route`,
          and router.tsx assembles them into the tree. */}
      <Outlet />
    </AppShell>
  );
}
