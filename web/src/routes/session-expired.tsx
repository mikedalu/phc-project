import { createRoute, useRouter } from '@tanstack/react-router';
import { CalendarClock } from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { Button } from '@/components/ui/button';
import { useSession, signOut } from '@/features/auth/session';

/**
 * The offline window has closed and the deployment blocks writes.
 *
 * A dedicated screen rather than a redirect: the user is still holding a valid
 * local replica and can still READ it, so throwing them back to a login form
 * would imply the data is gone. It is not. They just cannot add to it until the
 * device reconnects.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/session-expired',
  component: SessionExpiredPage,
});

function SessionExpiredPage() {
  const router = useRouter();
  const offlineGraceExpiresAt = useSession((s) => s.offlineGraceExpiresAt);

  const handleReauthenticate = async () => {
    await signOut();
    await router.navigate({ to: '/login' });
  };

  return (
    <div className="mx-auto max-w-lg py-10">
      <div className="flex items-start gap-3 rounded-md border border-warning/30 bg-warning-soft p-5">
        <CalendarClock aria-hidden="true" className="mt-0.5 size-5 shrink-0 text-warning-fg" />
        <div>
          <h1 className="text-lg font-semibold text-warning-fg">Offline access has expired</h1>
          <p className="mt-2 text-sm text-text-subtle">
            This device has been offline since its last sign-in
            {offlineGraceExpiresAt
              ? ` and its access window ended ${new Date(offlineGraceExpiresAt).toLocaleString()}.`
              : '.'}
          </p>
          <p className="mt-2 text-sm text-text-subtle">
            Patient records already on this device remain readable. New records cannot be saved
            until the device connects and you sign in again, because an un-synced write with no
            valid session cannot be attributed to anyone.
          </p>

          <Button className="mt-4" onClick={() => void handleReauthenticate()}>
            Connect and sign in
          </Button>
        </div>
      </div>
    </div>
  );
}
