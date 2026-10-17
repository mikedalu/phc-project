import { useEffect, useRef, useState } from 'react';
import { createRoute, Link, useNavigate } from '@tanstack/react-router';
import QRCode from 'qrcode';
import { ArrowLeft, Loader2, Printer } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { issueReferral } from '@/features/referrals/api';
import { useSession } from '@/features/auth/session';

/**
 * Issue a referral.
 *
 * The QR is drawn ON THIS DEVICE from the token, never from an image the server
 * produced: the token is the credential, and the code that carries it must
 * exist before the printer does.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/referrals/new',
  validateSearch: (search: Record<string, unknown>) => ({
    encounterId: String(search.encounterId ?? ''),
  }),
  component: NewReferralPage,
});

function NewReferralPage() {
  const { encounterId } = Route.useSearch();
  const navigate = useNavigate();
  const user = useSession((s) => s.user);

  const [encounter, setEncounter] = useState<{ patientId: string; visitNumber: string } | null>(null);
  const [toFacility, setToFacility] = useState('');
  const [reason, setReason] = useState('');
  const [diagnosis, setDiagnosis] = useState('');
  const [treatment, setTreatment] = useState('');
  const [urgency, setUrgency] = useState<'routine' | 'urgent' | 'emergency'>('routine');

  const [token, setToken] = useState<string | null>(null);
  const [issuedNumber, setIssuedNumber] = useState<string | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!encounterId) return;
    void db.encounters.get(encounterId).then((e) => {
      if (e) setEncounter({ patientId: e.patientId, visitNumber: e.visitNumber });
    });
  }, [encounterId]);

  useEffect(() => {
    if (!token || !canvasRef.current) return;
    // Wide margin and high error correction: this gets printed on a cheap
    // printer and read through a phone camera in poor light.
    void QRCode.toCanvas(canvasRef.current, token, {
      width: 280,
      margin: 2,
      errorCorrectionLevel: 'M',
    });
  }, [token]);

  const submit = async () => {
    if (!encounter) return;
    setBusy(true);
    try {
      const result = await issueReferral({
        patientId: encounter.patientId,
        encounterId,
        toFacility,
        reason,
        diagnosisSummary: diagnosis || undefined,
        treatmentGiven: treatment || undefined,
        urgency,
      });
      setToken(result.token);
      setIssuedNumber(result.referral.referralNumber);
      toast.success(`Referral ${result.referral.referralNumber} issued.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not issue the referral.');
    } finally {
      setBusy(false);
    }
  };

  if (!user) return <p className="text-sm text-text">Sign in to issue a referral.</p>;

  if (token) {
    const url = `${window.location.origin}/r/${token}`;
    return (
      <div className="space-y-5">
        <Link to="/queue" className="inline-flex items-center gap-1.5 text-sm text-text-muted hover:text-text">
          <ArrowLeft aria-hidden="true" className="size-3.5" />
          Queue
        </Link>

        <header>
          <h1 className="text-2xl font-semibold tracking-tight text-text">Referral issued</h1>
          <p className="mt-1 font-mono text-sm text-text-muted">{issuedNumber}</p>
          <p className="mt-1 text-sm text-text-muted">
            Valid for 48 hours. The receiving facility scans this code — no account needed.
          </p>
        </header>

        <div className="flex flex-col items-center gap-4 rounded-md border border-border bg-surface p-6 sm:flex-row sm:items-start">
          <canvas ref={canvasRef} className="rounded-sm bg-white p-2" aria-label="Referral QR code" />

          <div className="space-y-2 text-sm">
            <p className="font-medium text-text">{toFacility}</p>
            <p className="text-text-subtle">Reason: {reason}</p>
            <p className="text-text-subtle">
              Urgency: <span className="font-medium text-text">{urgency}</span>
            </p>
            <p className="break-all font-mono text-xs text-text-muted">{url}</p>
            <div className="flex gap-2 pt-2">
              <Button size="sm" onClick={() => window.print()}>
                <Printer aria-hidden="true" className="size-3.5" />
                Print
              </Button>
              <Button size="sm" variant="secondary" onClick={() => void navigate({ to: '/queue' })}>
                Done
              </Button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <Link to="/queue" className="inline-flex items-center gap-1.5 text-sm text-text-muted hover:text-text">
        <ArrowLeft aria-hidden="true" className="size-3.5" />
        Queue
      </Link>

      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Refer a patient</h1>
        <p className="mt-1 text-sm text-text-muted">
          {encounter ? `Visit ${encounter.visitNumber}` : 'Loading visit…'}
        </p>
      </header>

      <section className="max-w-2xl space-y-3 rounded-md border border-border bg-surface p-5">
        <FormField label="Refer to" required>
          {(field) => (
            <Input
              id={field.id}
              value={toFacility}
              onChange={(e) => setToFacility(e.target.value)}
              placeholder="General Hospital, Enugu"
            />
          )}
        </FormField>

        <FormField label="Reason" required>
          {(field) => (
            <Input
              id={field.id}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Needs surgical review"
            />
          )}
        </FormField>

        <div className="grid gap-3 sm:grid-cols-2">
          <FormField label="Diagnosis summary">
            {(field) => (
              <Input id={field.id} value={diagnosis} onChange={(e) => setDiagnosis(e.target.value)} />
            )}
          </FormField>
          <FormField label="Treatment given">
            {(field) => (
              <Input id={field.id} value={treatment} onChange={(e) => setTreatment(e.target.value)} />
            )}
          </FormField>
        </div>

        <FormField label="Urgency">
          {(field) => (
            <select
              id={field.id}
              value={urgency}
              onChange={(e) => setUrgency(e.target.value as typeof urgency)}
              className="h-9 w-full rounded-sm border border-border-strong bg-surface px-3 text-md text-text"
            >
              <option value="routine">Routine</option>
              <option value="urgent">Urgent</option>
              <option value="emergency">Emergency</option>
            </select>
          )}
        </FormField>

        <Button onClick={() => void submit()} disabled={busy || !toFacility.trim() || !reason.trim()}>
          {busy ? <Loader2 aria-hidden="true" className="size-4 animate-spin" /> : null}
          Issue referral
        </Button>
      </section>
    </div>
  );
}
