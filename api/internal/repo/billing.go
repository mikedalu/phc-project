package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

// Billing holds the invoice, charge and payment SQL.
type Billing struct{}

// NewBilling builds the repository.
func NewBilling() *Billing { return &Billing{} }

// InsertInvoice writes the bill header.
func (r *Billing) InsertInvoice(ctx context.Context, q Querier, inv *domain.Invoice) error {
	const sql = `
        INSERT INTO invoices (
            id, invoice_number, patient_id, household_id, encounter_id,
            invoice_type, status, total_minor, issued_at, issued_by, notes,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)`

	if _, err := q.Exec(ctx, sql,
		inv.ID, inv.InvoiceNumber, inv.PatientID, inv.HouseholdID, inv.EncounterID,
		inv.InvoiceType, inv.Status, inv.TotalMinor, inv.IssuedAt, inv.IssuedBy, inv.Notes,
		inv.CreatedBy,
	); err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: invoice number already exists", domain.ErrConflict)
		}
		return fmt.Errorf("insert invoice: %w", err)
	}
	return nil
}

// InsertCharge writes one line, snapping the price it was created with.
func (r *Billing) InsertCharge(ctx context.Context, q Querier, c *domain.Charge) error {
	const sql = `
        INSERT INTO charges (
            id, invoice_id, patient_id, household_id, charge_type, tariff_id,
            description, reference_id, reference_type, quantity,
            unit_price_minor, amount_minor, status, posted_at,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15)`

	if _, err := q.Exec(ctx, sql,
		c.ID, c.InvoiceID, c.PatientID, c.HouseholdID, c.ChargeType, c.TariffID,
		c.Description, c.ReferenceID, c.ReferenceType, c.Quantity,
		c.UnitPriceMinor, c.AmountMinor, c.Status, c.PostedAt,
		c.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert charge: %w", err)
	}
	return nil
}

// FindInvoice loads one invoice.
func (r *Billing) FindInvoice(ctx context.Context, q Querier, id uuid.UUID) (*domain.Invoice, error) {
	const sql = `
        SELECT id, invoice_number, patient_id, household_id, encounter_id,
               invoice_type, status, total_minor, issued_at, issued_by,
               voided_at, voided_by, void_reason, notes,
               created_at, updated_at, created_by, updated_by
          FROM invoices
         WHERE id = $1 AND deleted_at IS NULL`

	var inv domain.Invoice
	err := q.QueryRow(ctx, sql, id).Scan(
		&inv.ID, &inv.InvoiceNumber, &inv.PatientID, &inv.HouseholdID, &inv.EncounterID,
		&inv.InvoiceType, &inv.Status, &inv.TotalMinor, &inv.IssuedAt, &inv.IssuedBy,
		&inv.VoidedAt, &inv.VoidedBy, &inv.VoidReason, &inv.Notes,
		&inv.CreatedAt, &inv.UpdatedAt, &inv.CreatedBy, &inv.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find invoice: %w", err)
	}
	return &inv, nil
}

// InsertPayment writes money received.
//
// The UNIQUE constraint on operation_id is the idempotency guard: a retried
// push gets a duplicate error, which the caller reports as "already applied"
// rather than charging the patient twice.
func (r *Billing) InsertPayment(ctx context.Context, q Querier, p *domain.Payment) (bool, error) {
	const sql = `
        INSERT INTO payments (
            id, operation_id, receipt_number, payer_patient_id, household_id,
            amount_minor, currency_code, payment_method, payment_reference,
            status, paid_at, received_by, reversal_of_payment_id, notes,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15)
        ON CONFLICT (operation_id) DO NOTHING`

	tag, err := q.Exec(ctx, sql,
		p.ID, p.OperationID, p.ReceiptNumber, p.PayerPatientID, p.HouseholdID,
		p.AmountMinor, p.CurrencyCode, p.Method, p.Reference,
		p.Status, p.PaidAt, p.ReceivedBy, p.ReversalOf, p.Notes,
		p.CreatedBy,
	)
	if err != nil {
		if isUniqueViolation(err) {
			// Same operation_id OR the same receipt number. Both mean "already
			// recorded", which is what the caller needs to know.
			return false, nil
		}
		return false, fmt.Errorf("insert payment: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Allocate applies part of a payment to a charge.
func (r *Billing) Allocate(ctx context.Context, q Querier, a *domain.Allocation) error {
	const sql = `
        INSERT INTO payment_allocations (
            id, payment_id, charge_id, amount_minor, allocated_at, allocated_by,
            created_by, updated_by
        ) VALUES ($1,$2,$3,$4,$5,$6,$6,$6)
        ON CONFLICT (payment_id, charge_id) DO NOTHING`

	if _, err := q.Exec(ctx, sql,
		a.ID, a.PaymentID, a.ChargeID, a.AmountMinor, a.AllocatedAt, a.AllocatedBy,
	); err != nil {
		return fmt.Errorf("insert allocation: %w", err)
	}
	return nil
}

// InvoiceBalance DERIVES what is owed, in one aggregate.
func (r *Billing) InvoiceBalance(ctx context.Context, q Querier, invoiceID uuid.UUID) (*domain.InvoiceBalance, error) {
	const sql = `
        SELECT i.id, i.total_minor, i.status,
               COALESCE((
                   SELECT sum(pa.amount_minor)
                     FROM payment_allocations pa
                     JOIN charges c ON c.id = pa.charge_id
                    WHERE c.invoice_id = i.id
                      AND pa.deleted_at IS NULL
               ), 0) AS paid_minor
          FROM invoices i
         WHERE i.id = $1 AND i.deleted_at IS NULL`

	var b domain.InvoiceBalance
	if err := q.QueryRow(ctx, sql, invoiceID).Scan(&b.InvoiceID, &b.TotalMinor, &b.Status, &b.PaidMinor); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("invoice balance: %w", err)
	}
	b.BalanceMinor = b.TotalMinor - b.PaidMinor
	return &b, nil
}

// ActiveCharges lists the live lines of an invoice.
func (r *Billing) ActiveCharges(ctx context.Context, q Querier, invoiceID uuid.UUID) ([]*domain.Charge, error) {
	const sql = `
        SELECT id, invoice_id, patient_id, household_id, charge_type, tariff_id,
               description, reference_id, reference_type, quantity,
               unit_price_minor, amount_minor, status, posted_at,
               created_at, updated_at, created_by, updated_by
          FROM charges
         WHERE invoice_id = $1 AND status = 'active' AND deleted_at IS NULL
         ORDER BY posted_at`

	rows, err := q.Query(ctx, sql, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("list charges: %w", err)
	}
	defer rows.Close()

	var out []*domain.Charge
	for rows.Next() {
		var c domain.Charge
		if err := rows.Scan(
			&c.ID, &c.InvoiceID, &c.PatientID, &c.HouseholdID, &c.ChargeType, &c.TariffID,
			&c.Description, &c.ReferenceID, &c.ReferenceType, &c.Quantity,
			&c.UnitPriceMinor, &c.AmountMinor, &c.Status, &c.PostedAt,
			&c.CreatedAt, &c.UpdatedAt, &c.CreatedBy, &c.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan charge: %w", err)
		}
		out = append(out, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate charges: %w", err)
	}
	return out, nil
}

// SetInvoiceStatus updates the derived status label on the bill.
func (r *Billing) SetInvoiceStatus(ctx context.Context, q Querier, invoiceID uuid.UUID, status string, actor uuid.UUID) error {
	const sql = `
        UPDATE invoices
           SET status = $2, updated_by = $3, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND deleted_at IS NULL`

	if _, err := q.Exec(ctx, sql, invoiceID, status, actor); err != nil {
		return fmt.Errorf("set invoice status: %w", err)
	}
	return nil
}

// Tariff is a price-list entry.
type Tariff struct {
	ID             uuid.UUID
	Code           string
	Name           string
	Category       string
	UnitPriceMinor int64
	Unit           *string
}

// FindActiveTariff returns the currently effective price for a code.
func (r *Billing) FindActiveTariff(ctx context.Context, q Querier, code string) (*Tariff, error) {
	const sql = `
        SELECT id, code, name, category, unit_price_minor, unit
          FROM tariffs
         WHERE code = $1
           AND is_active
           AND deleted_at IS NULL
           AND effective_from <= CURRENT_DATE
           AND (effective_to IS NULL OR effective_to >= CURRENT_DATE)
         ORDER BY effective_from DESC
         LIMIT 1`

	var t Tariff
	if err := q.QueryRow(ctx, sql, code).Scan(&t.ID, &t.Code, &t.Name, &t.Category, &t.UnitPriceMinor, &t.Unit); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find tariff: %w", err)
	}
	return &t, nil
}

// LinkLabOrderToInvoice records which bill a test belongs to.
func (r *Billing) LinkLabOrderToInvoice(ctx context.Context, q Querier, labOrderID, invoiceID, chargeID uuid.UUID) error {
	const sql = `
        UPDATE lab_orders
           SET invoice_id = $2, charge_id = $3, updated_at = now(),
               revision = revision + 1, server_rev = server_rev + 1
         WHERE id = $1 AND deleted_at IS NULL`

	if _, err := q.Exec(ctx, sql, labOrderID, invoiceID, chargeID); err != nil {
		return fmt.Errorf("link lab order to invoice: %w", err)
	}
	return nil
}
