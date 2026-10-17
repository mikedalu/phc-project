import { useEffect, useState } from 'react';
import { createRoute, redirect, useRouter } from '@tanstack/react-router';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { AlertCircle, Loader2, LockKeyhole } from 'lucide-react';
import { z } from 'zod';
import { Route as rootRoute } from '@/routes/__root';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { ApiError } from '@/lib/api';
import { useSession, signIn, signInOffline, OfflineSignInError } from '@/features/auth/session';
import { readOfflineCredential } from '@/features/auth/offlineCredentials';
import { describeDevice } from '@/features/auth/device-label';

/**
 * Sign-in screen.
 *
 * Three things this screen must do that a generic login form would not:
 *
 *  1. Tell the truth when the device is offline. Signing in requires the server,
 *     so an offline user gets a specific message — not "invalid credentials",
 *     which would send them to an admin who cannot help.
 *  2. Survive a slow server. Cloud Run cold starts take seconds; the button
 *     stays in a loading state and the form stays disabled, so a double click
 *     cannot fire two logins.
 *  3. Show WHICH workstation this is. The device label and key state appear
 *     under the form, because "am I on the right machine?" is a question front
 *     desk staff actually ask when a shared PC is moved.
 */

const loginSchema = z.object({
  staffNumber: z.string().trim().min(1, 'Enter your staff number.'),
  password: z.string().min(1, 'Enter your password.'),
});

type LoginValues = z.infer<typeof loginSchema>;

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  beforeLoad: () => {
    // Already signed in: the login screen has nothing to offer. Redirect rather
    // than render a form that would create a second session on the same device.
    if (useSession.getState().status === 'authenticated') {
      throw redirect({ to: '/' });
    }
  },
  component: LoginPage,
});

function LoginPage() {
  const router = useRouter();
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [offline, setOffline] = useState(!navigator.onLine);
  const [enrolledStaffNumber, setEnrolledStaffNumber] = useState<string | null>(null);

  useEffect(() => {
    const onOnline = () => setOffline(false);
    const onOffline = () => setOffline(true);
    window.addEventListener('online', onOnline);
    window.addEventListener('offline', onOffline);

    void readOfflineCredential().then((credential) => {
      setEnrolledStaffNumber(credential?.staffNumber ?? null);
    });

    return () => {
      window.removeEventListener('online', onOnline);
      window.removeEventListener('offline', onOffline);
    };
  }, []);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { staffNumber: '', password: '' },
  });

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null);
    setSubmitting(true);

    try {
      if (!navigator.onLine) {
        await signInOffline({ staffNumber: values.staffNumber, password: values.password });
      } else {
        try {
          await signIn({ staffNumber: values.staffNumber, password: values.password });
        } catch (error) {
          // A dropped connection is not a rejected password. Fall back to the
          // device verifier rather than telling the user their credentials are
          // wrong while the wifi is out.
          if (error instanceof ApiError && error.isOffline) {
            await signInOffline({ staffNumber: values.staffNumber, password: values.password });
            setOffline(true);
          } else {
            throw error;
          }
        }
      }

      await router.navigate({ to: '/' });
    } catch (error) {
      setFormError(describeLoginError(error));
      setSubmitting(false);
    }
  });

  return (
    <div className="flex min-h-full items-center justify-center bg-canvas p-6">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex items-center gap-2.5">
          <span className="grid size-9 place-items-center rounded-sm bg-primary text-sm font-bold text-text-inverse">
            PH
          </span>
          <div>
            <h1 className="text-lg font-semibold text-text">PHC Records</h1>
            <p className="text-sm text-text-muted">Demo Primary Health Centre</p>
          </div>
        </div>

        <form
          onSubmit={onSubmit}
          noValidate
          aria-busy={submitting}
          className="flex flex-col gap-5 rounded-md border border-border bg-surface p-6"
        >
          <div className="flex items-center gap-2">
            <LockKeyhole aria-hidden="true" className="size-4 text-text-muted" />
            <h2 className="text-md font-medium text-text">Sign in</h2>
          </div>

          {offline ? (
            <div
              role="status"
              className="flex items-start gap-2 rounded-sm border border-warning/30 bg-warning-soft p-3 text-sm text-warning-fg"
            >
              <AlertCircle aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
              <span>
                No connection. Signing in with this device&rsquo;s stored verifier
                {enrolledStaffNumber ? ` as ${enrolledStaffNumber}` : ''}. Work you do is kept
                locally and syncs when the connection returns.
              </span>
            </div>
          ) : null}

          {!offline && enrolledStaffNumber ? (
            <p className="text-xs text-text-muted">
              Offline sign-in on this device is set up for{' '}
              <span className="font-mono">{enrolledStaffNumber}</span>.
            </p>
          ) : null}

          {formError ? (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-sm border border-danger/30 bg-danger-soft p-3 text-sm text-danger-fg"
            >
              <AlertCircle aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
              <span>{formError}</span>
            </div>
          ) : null}

          <FormField
            label="Staff number"
            required
            hint="For example DEMO/STF/0001"
            error={errors.staffNumber?.message}
          >
            {(field) => (
              <Input
                {...register('staffNumber')}
                id={field.id}
                aria-describedby={field.describedBy}
                aria-invalid={field.invalid}
                autoComplete="username"
                autoCapitalize="characters"
                spellCheck={false}
                autoFocus
                disabled={submitting}
              />
            )}
          </FormField>

          <FormField label="Password" required error={errors.password?.message}>
            {(field) => (
              <Input
                {...register('password')}
                id={field.id}
                type="password"
                aria-describedby={field.describedBy}
                aria-invalid={field.invalid}
                autoComplete="current-password"
                disabled={submitting}
              />
            )}
          </FormField>

          <Button type="submit" size="lg" disabled={submitting} className="w-full">
            {submitting ? (
              <>
                <Loader2 aria-hidden="true" className="size-4 animate-spin" />
                Signing in…
              </>
            ) : (
              'Sign in'
            )}
          </Button>
        </form>

        <p className="mt-4 text-center text-xs text-text-muted">
          This workstation: <span className="text-text-subtle">{describeDevice()}</span>
        </p>
      </div>
    </div>
  );
}

/**
 * Turns an API failure into something a clinician can act on.
 *
 * Every branch says what to DO, not just what went wrong. "Account locked" with
 * no instruction is a support call; "try again in a few minutes" is not.
 */
function describeLoginError(error: unknown): string {
  if (error instanceof OfflineSignInError) {
    switch (error.code) {
      case 'NO_CREDENTIAL':
        return 'This device has not been set up for offline sign-in. Connect to the internet and sign in once, then it will work offline.';
      case 'LOCKED':
        return 'Too many failed offline attempts. Wait 15 minutes, or connect and sign in online.';
      case 'WRONG_STAFF_NUMBER':
        return error.message;
      default:
        return 'That password does not match this device.';
    }
  }

  if (error instanceof ApiError) {
    switch (error.code) {
      case 'ACCOUNT_LOCKED':
        return 'Too many failed attempts. Wait a few minutes, or ask an administrator to unlock the account.';
      case 'UNAUTHENTICATED':
        return 'That staff number and password do not match. Check both and try again.';
      case 'NETWORK_ERROR':
      case 'REQUEST_TIMEOUT':
        return 'Could not reach the server. Signing in needs a connection; once you are signed in, the app keeps working offline.';
      case 'DEVICE_REVOKED':
        return 'This workstation has been disabled. Ask an administrator to re-enable it.';
      case 'VALIDATION_FAILED':
        return 'Check the form and try again.';
      default:
        return error.message;
    }
  }

  if (error instanceof Error) {
    // A WebCrypto failure lands here: no secure context, or a browser that
    // cannot hold the device key. Say that plainly instead of "unknown error".
    return error.message;
  }

  return 'Something went wrong. Try again.';
}
