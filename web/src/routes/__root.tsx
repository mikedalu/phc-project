import { createRootRoute, Outlet } from '@tanstack/react-router';
import { AppShell } from '@/components/AppShell';

/**
 * The root route.
 *
 * Deliberately NOT using TanStack Router's file-based generation: the route
 * tree is small and explicit here, which keeps the wiring readable and avoids a
 * generated file that must be regenerated before the project will typecheck.
 *
 * `createRootRoute` (not `createRootRouteWithContext`) because there is no
 * router context yet — auth supplies it in Sprint 1.
 */
export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <AppShell>
      {/* Renders the matched child route. Every route file exports `Route`,
          and router.tsx assembles them into the tree. */}
      <Outlet />
    </AppShell>
  );
}
