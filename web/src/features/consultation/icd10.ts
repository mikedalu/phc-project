/**
 * A BUNDLED shortlist of ICD-10 codes for the conditions this clinic actually
 * sees.
 *
 * ═══ WHY NOT THE FULL ICD-10 SET ═══
 *
 * The full classification is ~14,000 codes behind a licence, and it is mostly
 * irrelevant here: a PHC in one district treats a few dozen things. A shortlist
 * the doctor can type three letters into is more useful than a complete list
 * they have to search, and `icd10_code` is free text on the server so a code
 * outside this list can still be entered by hand.
 */
export interface ICD10Entry {
  code: string;
  label: string;
}

export const ICD10_SHORTLIST: ICD10Entry[] = [
  { code: 'B54', label: 'Unspecified malaria' },
  { code: 'A09', label: 'Diarrhoea and gastroenteritis of presumed infectious origin' },
  { code: 'J06.9', label: 'Acute upper respiratory infection, unspecified' },
  { code: 'J18.9', label: 'Pneumonia, unspecified organism' },
  { code: 'A91', label: 'Dengue haemorrhagic fever' },
  { code: 'B50.9', label: 'Plasmodium falciparum malaria, unspecified' },
  { code: 'D50.9', label: 'Iron deficiency anaemia, unspecified' },
  { code: 'E11.9', label: 'Type 2 diabetes mellitus without complications' },
  { code: 'I10', label: 'Essential (primary) hypertension' },
  { code: 'K29.7', label: 'Gastritis, unspecified' },
  { code: 'N39.0', label: 'Urinary tract infection, site not specified' },
  { code: 'L30.9', label: 'Dermatitis, unspecified' },
  { code: 'H10.9', label: 'Conjunctivitis, unspecified' },
  { code: 'M54.5', label: 'Low back pain' },
  { code: 'R50.9', label: 'Fever, unspecified' },
  { code: 'R05', label: 'Cough' },
  { code: 'R10.4', label: 'Other and unspecified abdominal pain' },
  { code: 'O26.9', label: 'Pregnancy-related condition, unspecified' },
  { code: 'Z34.9', label: 'Supervision of normal pregnancy, unspecified' },
  { code: 'A15.0', label: 'Tuberculosis of lung, confirmed by sputum microscopy' },
];

/** Case-insensitive search over code and label. */
export function searchICD10(term: string, limit = 6): ICD10Entry[] {
  const needle = term.trim().toLowerCase();
  if (needle.length < 2) return [];
  return ICD10_SHORTLIST.filter(
    (entry) =>
      entry.code.toLowerCase().includes(needle) || entry.label.toLowerCase().includes(needle),
  ).slice(0, limit);
}
