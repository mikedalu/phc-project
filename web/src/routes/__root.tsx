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

    // A scanned referral is read by a STRANGER at another facility. They have no
    // session and never will; the encrypted token is the credential. Guarding
    // this path would make the feature fail exactly when it is needed.
    if (location.pathname.startsWith('/r/')) return;

    if (useSession.getState().status !== 'authenticated') {
      throw redirect({ to: '/login' });
    }
  },
  component: RootLayout,
});

function RootLayout() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  // The login screen and the scanned referral are full-page canvases. A sidebar
  // full of clinical navigation next to either one invites clicking things that
  // require a session the viewer does not have.
  if (pathname === '/login' || pathname.startsWith('/r/')) {
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
