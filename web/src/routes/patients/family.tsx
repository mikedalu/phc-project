import { useState } from 'react';
import { createRoute, Link, useNavigate } from '@tanstack/react-router';
import { ArrowLeft, Loader2, Plus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { registerFamily } from '@/features/patients/family';

/**
 * Family registration.
 *
 * A household is the BILLING party — the reason a family is a first-class thing
 * rather than a label on a patient. Registering the whole family in one sitting
 * is the flow the front desk asked for, and it is done locally, in one
 * transaction, so a failure cannot leave half a family behind.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/patients/family',
  component: FamilyPage,
});

interface MemberDraft {
  key: string;
  firstName: string;
  surname: string;
  gender: 'male' | 'female' | 'other' | 'unknown';
  ageMode: 'dob' | 'age';
  dateOfBirth: string;
  ageYears: string;
  ageMonths: string;
  isHead: boolean;
}

function emptyMember(isHead = false): MemberDraft {
  return {
    key: crypto.randomUUID(),
    firstName: '',
    surname: '',
    gender: 'female',
    ageMode: 'dob',
    dateOfBirth: '',
    ageYears: '',
    ageMonths: '',
    isHead,
  };
}

function FamilyPage() {
  const navigate = useNavigate();
  const [familyName, setFamilyName] = useState('');
  const [phone, setPhone] = useState('');
  const [address, setAddress] = useState('');
  const [members, setMembers] = useState<MemberDraft[]>([emptyMember(true)]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const update = (key: string, patch: Partial<MemberDraft>) => {
    setMembers((current) => current.map((m) => (m.key === key ? { ...m, ...patch } : m)));
  };

  const submit = async () => {
    setError(null);

    if (!familyName.trim()) {
      setError('A family name is required.');
      return;
    }

    for (const [index, member] of members.entries()) {
      if (!member.firstName.trim()) {
        setError(`Member ${index + 1} needs a first name.`);
        return;
      }
      if (member.ageMode === 'dob' && !member.dateOfBirth) {
        setError(`Member ${index + 1} needs a date of birth, or an age.`);
        return;
      }
      if (member.ageMode === 'age' && !member.ageYears && !member.ageMonths) {
        setError(`Member ${index + 1} needs an age in years or months.`);
        return;
      }
    }

    setSaving(true);
    try {
      const result = await registerFamily({
        familyName,
        phone: phone || null,
        address: address || null,
        notes: null,
        members: members.map((member) => ({
          isHead: member.isHead,
          firstName: member.firstName,
          middleName: null,
          surname: member.surname || familyName,
          gender: member.gender,
          dateOfBirth: member.ageMode === 'dob' ? member.dateOfBirth : null,
          ageYears: member.ageMode === 'age' && member.ageYears ? Number(member.ageYears) : null,
          ageMonths: member.ageMode === 'age' && member.ageMonths ? Number(member.ageMonths) : null,
          maritalStatus: null,
          phone: phone || null,
          altPhone: null,
          tribe: null,
          occupation: null,
          address: address || null,
          addressLandmark: null,
          bloodGroup: null,
          genotype: null,
          allergies: null,
        })),
      });

      toast.success(
        navigator.onLine
          ? 'Family registered.'
          : 'Family saved on this device. It will sync when the connection returns.',
      );
      await navigate({ to: '/households/$id', params: { id: result.householdId } });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not register the family.');
      setSaving(false);
    }
  };

  return (
    <div className="space-y-5">
      <Link to="/patients" className="inline-flex items-center gap-1.5 text-sm text-text-muted hover:text-text">
        <ArrowLeft aria-hidden="true" className="size-3.5" />
        Patients
      </Link>

      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Register a family</h1>
        <p className="mt-1 text-sm text-text-muted">
          All members are saved together. The household becomes the billing party for the family.
        </p>
      </header>

      {error ? (
        <div role="alert" className="rounded-sm border border-danger/30 bg-danger-soft p-3 text-sm text-danger-fg">
          {error}
        </div>
      ) : null}

      <section className="grid max-w-3xl gap-4 rounded-md border border-border bg-surface p-6 sm:grid-cols-2">
        <FormField label="Family name" required>
          {(field) => (
            <Input id={field.id} value={familyName} onChange={(e) => setFamilyName(e.target.value)} autoFocus />
          )}
        </FormField>
        <FormField label="Phone">
          {(field) => (
            <Input id={field.id} inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} />
          )}
        </FormField>
        <FormField label="Address" className="sm:col-span-2">
          {(field) => <Input id={field.id} value={address} onChange={(e) => setAddress(e.target.value)} />}
        </FormField>
      </section>

      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-md font-medium text-text">Members ({members.length})</h2>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setMembers((current) => [...current, emptyMember()])}
          >
            <Plus aria-hidden="true" className="size-3.5" />
            Add member
          </Button>
        </div>

        {members.map((member, index) => (
          <div key={member.key} className="rounded-md border border-border bg-surface p-4">
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium text-text">Member {index + 1}</p>
              <div className="flex items-center gap-3">
                <label className="flex items-center gap-1.5 text-xs text-text-subtle">
                  <input
                    type="radio"
                    name="head"
                    checked={member.isHead}
                    onChange={() =>
                      setMembers((current) =>
                        current.map((m) => ({ ...m, isHead: m.key === member.key })),
                      )
                    }
                  />
                  Head of household
                </label>
                {members.length > 1 ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    aria-label={`Remove member ${index + 1}`}
                    onClick={() => setMembers((current) => current.filter((m) => m.key !== member.key))}
                  >
                    <Trash2 aria-hidden="true" className="size-3.5" />
                  </Button>
                ) : null}
              </div>
            </div>

            <div className="mt-3 grid gap-3 sm:grid-cols-4">
              <FormField label="First name" required>
                {(field) => (
                  <Input
                    id={field.id}
                    value={member.firstName}
                    onChange={(e) => update(member.key, { firstName: e.target.value })}
                  />
                )}
              </FormField>
              <FormField label="Surname" hint="Defaults to the family name">
                {(field) => (
                  <Input
                    id={field.id}
                    value={member.surname}
                    onChange={(e) => update(member.key, { surname: e.target.value })}
                  />
                )}
              </FormField>
              <FormField label="Sex">
                {(field) => (
                  <select
                    id={field.id}
                    value={member.gender}
                    onChange={(e) => update(member.key, { gender: e.target.value as MemberDraft['gender'] })}
                    className="h-9 w-full rounded-sm border border-border-strong bg-surface px-3 text-md text-text"
                  >
                    <option value="female">Female</option>
                    <option value="male">Male</option>
                    <option value="other">Other</option>
                    <option value="unknown">Unknown</option>
                  </select>
                )}
              </FormField>

              {member.ageMode === 'dob' ? (
                <FormField label="Date of birth" required>
                  {(field) => (
                    <Input
                      id={field.id}
                      type="date"
                      max={new Date().toISOString().slice(0, 10)}
                      value={member.dateOfBirth}
                      onChange={(e) => update(member.key, { dateOfBirth: e.target.value })}
                    />
                  )}
                </FormField>
              ) : (
                <div className="grid grid-cols-2 gap-2">
                  <FormField label="Years">
                    {(field) => (
                      <Input
                        id={field.id}
                        inputMode="numeric"
                        value={member.ageYears}
                        onChange={(e) => update(member.key, { ageYears: e.target.value })}
                      />
                    )}
                  </FormField>
                  <FormField label="Months">
                    {(field) => (
                      <Input
                        id={field.id}
                        inputMode="numeric"
                        value={member.ageMonths}
                        onChange={(e) => update(member.key, { ageMonths: e.target.value })}
                      />
                    )}
                  </FormField>
                </div>
              )}
            </div>

            <button
              type="button"
              onClick={() =>
                update(member.key, { ageMode: member.ageMode === 'dob' ? 'age' : 'dob' })
              }
              className="mt-2 text-xs text-primary hover:underline"
            >
              {member.ageMode === 'dob' ? 'No birth date? Enter an age instead' : 'Have a birth date? Enter it instead'}
            </button>
          </div>
        ))}
      </section>

      <Button size="lg" onClick={() => void submit()} disabled={saving}>
        {saving ? (
          <>
            <Loader2 aria-hidden="true" className="size-4 animate-spin" />
            Saving…
          </>
        ) : (
          'Register family'
        )}
      </Button>
    </div>
  );
}
