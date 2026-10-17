import { apiGet, apiPost } from '@/lib/api';

/**
 * Referral API.
 *
 * The ONLY clinical flow that talks to the server directly instead of through
 * Dexie. A referral is issued once, used elsewhere, and expires: there is no
 * value in replicating it to a device that may never see the patient again, and
 * its whole point is to reach a machine that has no database of ours.
 */
export interface IssuedReferral {
  id: string;
  referralNumber: string;
  toFacility: string;
  urgency: string;
  expiresAt: string;
}

export interface ReferralNote {
  v: number;
  ref: string;
  name: string;
  age?: number;
  sex?: string;
  from: string;
  to: string;
  reason: string;
  dx?: string;
  tx?: string;
  urgency: string;
  iat: string;
  exp: string;
}

export async function issueReferral(input: {
  patientId: string;
  encounterId: string;
  toFacility: string;
  reason: string;
  diagnosisSummary?: string;
  treatmentGiven?: string;
  urgency: 'routine' | 'urgent' | 'emergency';
}): Promise<{ referral: IssuedReferral; token: string }> {
  const { data } = await apiPost<{ referral: IssuedReferral; token: string }>('/referrals', input);
  return data;
}

/** Public: no session required, because the receiving facility has none. */
export async function readReferral(token: string): Promise<ReferralNote> {
  const { data } = await apiGet<ReferralNote>(`/referrals/${encodeURIComponent(token)}`, {
    skipAuth: true,
  });
  return data;
}
