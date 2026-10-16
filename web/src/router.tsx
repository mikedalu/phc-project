import { createRouter } from '@tanstack/react-router';
import { Route as rootRoute } from '@/routes/__root';
import { Route as indexRoute } from '@/routes/index';
import { Route as syncRoute } from '@/routes/sync';
import { PatientsRoute, QueueRoute } from '@/routes/placeholders';

/**
 * The assembled route tree.
 *
 * Adding a screen: create `src/routes/<name>.tsx` exporting `Route`, then import
 * it here and add it to `addChildren`. Keep children alphabetical by path so
 * merge conflicts stay trivial.
 */
const routeTree = rootRoute.addChildren([
  indexRoute,
  syncRoute,
  PatientsRoute,
  QueueRoute,
]);

export const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
  // Restore scroll position on back/forward, which matters on long patient lists.
  scrollRestoration: true,
});

/**
 * Registers the router type globally so `Link to="..."` and `useNavigate` are
 * typed against the real route tree. Without this, every route string is `string`
 * and a typo becomes a runtime 404 instead of a compile error.
 */
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
