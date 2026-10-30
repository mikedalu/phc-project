import { useState, type FormEvent } from 'react';
import { createRoute, useNavigate } from '@tanstack/react-router';
import { KeyRound, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { changePassword } from '@/features/auth/api';
import { markPasswordChanged } from '@/features/auth/session';
import { humanError } from '@/lib/errors';

/**
 * Self-service password change.
 *
 * This is deliberately available to every staff member, not only admins: the
 * seeded demo accounts and every account created by an administrator carry a
 * temporary password until the person standing at the workstation replaces it.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/change-password',
  component: ChangePasswordPage,
});

function ChangePasswordPage() {
  const navigate = useNavigate();
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);

    if (next.length < 12) {
      setError('Use at least 12 characters. Length matters more than symbols.');
      return;
    }
    if (next !== confirm) {
      setError('The two new passwords do not match.');
      return;
    }

    setBusy(true);
    try {
      await changePassword({ currentPassword: current, newPassword: next });
      markPasswordChanged();
      toast.success('Password changed. Other devices were signed out.');
      await navigate({ to: '/' });
    } catch (err) {
      setError(humanError(err, 'Could not change the password.'));
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto max-w-lg space-y-5">
      <header>
        <p className="flex items-center gap-2 text-sm text-text-muted">
          <KeyRound aria-hidden="true" className="size-4" />
          Account security
        </p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-text">Change password</h1>
        <p className="mt-1 text-sm text-text-muted">
          The current workstation stays signed in. Every other session on this account is signed
          out.
        </p>
      </header>

      <form
        onSubmit={(event) => void submit(event)}
        className="space-y-4 rounded-md border border-border bg-surface p-5"
      >
        <FormField label="Current password" required>
          {(field) => (
            <Input
              id={field.id}
              type="password"
              autoComplete="current-password"
              value={current}
              onChange={(event) => setCurrent(event.target.value)}
              disabled={busy}
            />
          )}
        </FormField>

        <FormField label="New password" required hint="At least 12 characters">
          {(field) => (
            <Input
              id={field.id}
              type="password"
              autoComplete="new-password"
              value={next}
              onChange={(event) => setNext(event.target.value)}
              disabled={busy}
            />
          )}
        </FormField>

        <FormField label="Confirm new password" required>
          {(field) => (
            <Input
              id={field.id}
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={(event) => setConfirm(event.target.value)}
              disabled={busy}
            />
          )}
        </FormField>

        {error ? (
          <p role="alert" className="rounded-sm bg-danger-soft px-3 py-2 text-sm text-danger-fg">
            {error}
          </p>
        ) : null}

        <div className="flex items-center gap-2">
          <Button type="submit" disabled={busy || !current || !next || !confirm}>
            {busy ? <Loader2 aria-hidden="true" className="size-4 animate-spin" /> : null}
            Change password
          </Button>
          <Button
            type="button"
            variant="ghost"
            onClick={() => void navigate({ to: '/' })}
            disabled={busy}
          >
            Cancel
          </Button>
        </div>
      </form>
    </div>
  );
}
