import { createRouter } from '@tanstack/react-router';
import { Route as rootRoute } from '@/routes/__root';
import { Route as indexRoute } from '@/routes/index';
import { Route as loginRoute } from '@/routes/login';
import { Route as sessionExpiredRoute } from '@/routes/session-expired';
import { Route as adminUsersRoute } from '@/routes/admin/users';
import { Route as syncProbeRoute } from '@/routes/dev/sync';
import { Route as syncRoute } from '@/routes/sync';
import { Route as QueueRoute } from '@/routes/queue/index';
import { Route as PatientListRoute } from '@/routes/patients/index';
import { Route as PatientNewRoute } from '@/routes/patients/new';
import { Route as PatientProfileRoute } from '@/routes/patients/$id';
import { Route as FamilyRoute } from '@/routes/patients/family';
import { Route as HouseholdListRoute } from '@/routes/households/index';
import { Route as HouseholdRoute } from '@/routes/households/$id';
import { Route as ConsultationRoute } from '@/routes/consultations/$encounterId';
import { Route as LabRoute } from '@/routes/lab/index';
import { Route as BillingRoute } from '@/routes/billing/index';
import { Route as PharmacyRoute } from '@/routes/pharmacy/index';
import { Route as WardRoute } from '@/routes/ward/index';
import { Route as NewReferralRoute } from '@/routes/referrals/new';
import { Route as PublicReferralRoute } from '@/routes/r/$token';
import { Route as AdminActivityRoute } from '@/routes/admin/activity';

/**
 * The assembled route tree.
 *
 * Adding a screen: create `src/routes/<name>.tsx` exporting `Route`, then import
 * it here and add it to `addChildren`. Keep children alphabetical by path so
 * merge conflicts stay trivial.
 */
const routeTree = rootRoute.addChildren([
  loginRoute,
  sessionExpiredRoute,
  indexRoute,
  syncRoute,
  QueueRoute,
  PatientListRoute,
  PatientNewRoute,
  PatientProfileRoute,
  FamilyRoute,
  HouseholdListRoute,
  HouseholdRoute,
  ConsultationRoute,
  LabRoute,
  BillingRoute,
  PharmacyRoute,
  WardRoute,
  NewReferralRoute,
  PublicReferralRoute,
  AdminActivityRoute,
  adminUsersRoute,
  syncProbeRoute,
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
