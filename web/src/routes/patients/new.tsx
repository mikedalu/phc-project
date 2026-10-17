import { useState } from 'react';
import { createRoute, useNavigate, Link } from '@tanstack/react-router';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { AlertTriangle, ArrowLeft, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { registerPatient, findLocalDuplicates } from '@/features/patients/write';
import type { PatientRow } from '@/db/types';
import { humanError } from '@/lib/errors';

/**
 * Registration form.
 *
 * ═══ THE AGE/DOB TOGGLE IS NOT A UI PREFERENCE ═══
 *
 * A clerk often knows "about eight" and not a birth date. Forcing a date makes
 * them invent one, and an invented date is worse than an admitted estimate —
 * it looks exact for ever. So the form asks which one they have, and the service
 * derives the other.
 *
 * Duplicates are a WARNING here, exactly as on the server: the clerk sees the
 * match, can open it, and chooses. Blocking would teach them to work around it.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/patients/new',
  component: NewPatientPage,
});

const schema = z
  .object({
    firstName: z.string().trim().min(1, 'First name is required.'),
    middleName: z.string().trim().optional(),
    surname: z.string().trim().min(1, 'Surname is required.'),
    gender: z.enum(['male', 'female', 'other', 'unknown']),
    ageMode: z.enum(['dob', 'age']),
    dateOfBirth: z.string().optional(),
    ageYears: z.string().optional(),
    ageMonths: z.string().optional(),
    maritalStatus: z.string().optional(),
    phone: z.string().optional(),
    address: z.string().optional(),
    addressLandmark: z.string().optional(),
    tribe: z.string().optional(),
    occupation: z.string().optional(),
    bloodGroup: z.string().optional(),
    genotype: z.string().optional(),
    allergies: z.string().optional(),
  })
  .superRefine((values, ctx) => {
    if (values.ageMode === 'dob' && !values.dateOfBirth) {
      ctx.addIssue({ code: 'custom', path: ['dateOfBirth'], message: 'Enter a date of birth.' });
    }
    if (values.ageMode === 'age' && !values.ageYears && !values.ageMonths) {
      ctx.addIssue({
        code: 'custom',
        path: ['ageYears'],
        message: 'Enter an age in years or months.',
      });
    }
  });

type Values = z.infer<typeof schema>;

function NewPatientPage() {
  const navigate = useNavigate();
  const [saving, setSaving] = useState(false);
  const [duplicates, setDuplicates] = useState<PatientRow[] | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      firstName: '',
      middleName: '',
      surname: '',
      gender: 'female',
      ageMode: 'dob',
      dateOfBirth: '',
      ageYears: '',
      ageMonths: '',
      maritalStatus: '',
      phone: '',
      address: '',
      addressLandmark: '',
      tribe: '',
      occupation: '',
      bloodGroup: '',
      genotype: '',
      allergies: '',
    },
  });

  const ageMode = watch('ageMode');

  const save = async (values: Values) => {
    setSaving(true);
    try {
      const row = await registerPatient({
        firstName: values.firstName,
        middleName: values.middleName || null,
        surname: values.surname,
        gender: values.gender,
        dateOfBirth: values.ageMode === 'dob' ? values.dateOfBirth || null : null,
        ageYears: values.ageMode === 'age' && values.ageYears ? Number(values.ageYears) : null,
        ageMonths: values.ageMode === 'age' && values.ageMonths ? Number(values.ageMonths) : null,
        maritalStatus: values.maritalStatus || null,
        phone: values.phone || null,
        altPhone: null,
        tribe: values.tribe || null,
        occupation: values.occupation || null,
        address: values.address || null,
        addressLandmark: values.addressLandmark || null,
        bloodGroup: values.bloodGroup || null,
        genotype: values.genotype || null,
        allergies: values.allergies || null,
      });

      toast.success(
        navigator.onLine
          ? `Registered ${row.patientNumber}`
          : `Saved on this device as ${row.patientNumber}. It will sync when the connection returns.`,
      );
      await navigate({ to: '/patients/$id', params: { id: row.id } });
    } catch (error) {
      toast.error(humanError(error, 'Could not register the patient.'));
      setSaving(false);
    }
  };

  /** Check duplicates first, then confirm. */
  const onSubmit = handleSubmit(async (values) => {
    const matches = await findLocalDuplicates({
      firstName: values.firstName,
      surname: values.surname,
      dateOfBirth: values.ageMode === 'dob' ? values.dateOfBirth || null : null,
      phone: values.phone || null,
    });

    if (matches.length > 0 && pendingId === null) {
      setDuplicates(matches);
      return;
    }

    await save(values);
  });

  return (
    <div className="space-y-5">
      <Link to="/patients" className="inline-flex items-center gap-1.5 text-sm text-text-muted hover:text-text">
        <ArrowLeft aria-hidden="true" className="size-3.5" />
        Patients
      </Link>

      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Register a patient</h1>
        <p className="mt-1 text-sm text-text-muted">
          Saved on this device first. The record syncs when a connection is available.
        </p>
      </header>

      {duplicates ? (
        <section
          role="alert"
          className="rounded-md border border-warning/30 bg-warning-soft p-4"
        >
          <div className="flex items-start gap-2">
            <AlertTriangle aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning-fg" />
            <div>
              <p className="text-sm font-medium text-warning-fg">
                {duplicates.length} patient{duplicates.length === 1 ? '' : 's'} already match
              </p>
              <ul className="mt-2 space-y-1">
                {duplicates.map((match) => (
                  <li key={match.id} className="text-sm text-text-subtle">
                    <Link
                      to="/patients/$id"
                      params={{ id: match.id }}
                      className="font-medium text-primary hover:underline"
                    >
                      {match.firstName} {match.surname}
                    </Link>{' '}
                    · {match.patientNumber} · {match.dateOfBirth}
                  </li>
                ))}
              </ul>
              <div className="mt-3 flex gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => {
                    setDuplicates(null);
                    setPendingId('confirmed');
                  }}
                >
                  Register anyway
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setDuplicates(null)}>
                  Cancel
                </Button>
              </div>
            </div>
          </div>
        </section>
      ) : null}

      <form
        onSubmit={onSubmit}
        noValidate
        aria-busy={saving}
        className="grid max-w-3xl gap-5 rounded-md border border-border bg-surface p-6 sm:grid-cols-2"
      >
        <FormField label="First name" required error={errors.firstName?.message}>
          {(field) => (
            <Input {...register('firstName')} id={field.id} aria-invalid={field.invalid} autoFocus />
          )}
        </FormField>

        <FormField label="Surname" required error={errors.surname?.message}>
          {(field) => (
            <Input {...register('surname')} id={field.id} aria-invalid={field.invalid} />
          )}
        </FormField>

        <FormField label="Middle name" error={errors.middleName?.message}>
          {(field) => <Input {...register('middleName')} id={field.id} />}
        </FormField>

        <FormField label="Sex" required error={errors.gender?.message}>
          {(field) => (
            <select
              {...register('gender')}
              id={field.id}
              className="h-9 w-full rounded-sm border border-border-strong bg-surface px-3 text-md text-text"
            >
              <option value="female">Female</option>
              <option value="male">Male</option>
              <option value="other">Other</option>
              <option value="unknown">Unknown</option>
            </select>
          )}
        </FormField>

        <fieldset className="sm:col-span-2">
          <legend className="text-base font-medium text-text">Age</legend>
          <p className="mt-1 text-sm text-text-muted">
            A date of birth is better when you have it. An estimate is honest; an invented date is not.
          </p>

          <div className="mt-2 flex gap-4">
            <label className="flex items-center gap-2 text-sm text-text">
              <input type="radio" value="dob" {...register('ageMode')} /> Date of birth
            </label>
            <label className="flex items-center gap-2 text-sm text-text">
              <input type="radio" value="age" {...register('ageMode')} /> Age only
            </label>
          </div>

          <div className="mt-3 grid gap-4 sm:grid-cols-3">
            {ageMode === 'dob' ? (
              <FormField label="Date of birth" required error={errors.dateOfBirth?.message}>
                {(field) => (
                  <Input type="date" max={new Date().toISOString().slice(0, 10)} {...register('dateOfBirth')} id={field.id} aria-invalid={field.invalid} />
                )}
              </FormField>
            ) : (
              <>
                <FormField label="Years" error={errors.ageYears?.message}>
                  {(field) => (
                    <Input inputMode="numeric" {...register('ageYears')} id={field.id} aria-invalid={field.invalid} />
                  )}
                </FormField>
                <FormField label="Months" hint="Under 2 years">
                  {(field) => <Input inputMode="numeric" {...register('ageMonths')} id={field.id} />}
                </FormField>
              </>
            )}
          </div>
        </fieldset>

        <FormField label="Phone">
          {(field) => <Input inputMode="tel" {...register('phone')} id={field.id} />}
        </FormField>

        <FormField label="Marital status">
          {(field) => (
            <select
              {...register('maritalStatus')}
              id={field.id}
              className="h-9 w-full rounded-sm border border-border-strong bg-surface px-3 text-md text-text"
            >
              <option value="">—</option>
              <option value="single">Single</option>
              <option value="married">Married</option>
              <option value="divorced">Divorced</option>
              <option value="widowed">Widowed</option>
              <option value="separated">Separated</option>
            </select>
          )}
        </FormField>

        <FormField label="Address">
          {(field) => <Input {...register('address')} id={field.id} />}
        </FormField>

        <FormField label="Landmark" hint="“Opposite the market gate”">
          {(field) => <Input {...register('addressLandmark')} id={field.id} />}
        </FormField>

        <FormField label="Tribe">
          {(field) => <Input {...register('tribe')} id={field.id} list="tribes" />}
        </FormField>

        <FormField label="Occupation">
          {(field) => <Input {...register('occupation')} id={field.id} />}
        </FormField>

        <FormField label="Blood group">
          {(field) => (
            <Input {...register('bloodGroup')} id={field.id} placeholder="A+ / O−" />
          )}
        </FormField>

        <FormField label="Genotype">
          {(field) => <Input {...register('genotype')} id={field.id} placeholder="AA / AS" />}
        </FormField>

        <FormField label="Allergies" className="sm:col-span-2">
          {(field) => <Input {...register('allergies')} id={field.id} />}
        </FormField>

        <datalist id="tribes">
          <option value="Hausa" />
          <option value="Igbo" />
          <option value="Yoruba" />
          <option value="Fulani" />
          <option value="Tiv" />
          <option value="Kanuri" />
        </datalist>

        <div className="sm:col-span-2">
          <Button type="submit" size="lg" disabled={saving}>
            {saving ? (
              <>
                <Loader2 aria-hidden="true" className="size-4 animate-spin" />
                Saving…
              </>
            ) : (
              'Register patient'
            )}
          </Button>
        </div>
      </form>
    </div>
  );
}
