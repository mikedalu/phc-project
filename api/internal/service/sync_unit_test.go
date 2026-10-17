package service

// Unit tests for the pure parts of the sync engine.
//
// The database parts are covered by integration tests; these pin the decisions
// that are easy to change by accident and expensive to get wrong in production.

import (
	"testing"
	"time"

	"github.com/mikedalu/phc-records/api/internal/domain"
)

func TestSaneClientTimeRejectsAbsurdClocks(t *testing.T) {
	now := time.Date(2026, 2, 14, 9, 0, 0, 0, time.UTC)

	good := now.Add(-time.Hour)
	future := now.Add(48 * time.Hour)
	ancient := now.Add(-30 * 24 * time.Hour)
	nilTime := (*time.Time)(nil)

	cases := []struct {
		name string
		in   *time.Time
		want time.Time
	}{
		{"nil falls back", nilTime, now},
		{"recent is kept", &good, good},
		// A device clock rolled forward must not make its edit "newest for ever".
		{"far future is clamped", &future, now},
		// A clock stuck in the past must not make its edit permanently lose LWW.
		{"far past is clamped", &ancient, now},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := saneClientTime(tc.in, now); !got.Equal(tc.want) {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSummariseCountsEveryStatus(t *testing.T) {
	now := time.Now()
	results := []domain.OpResult{
		{Status: domain.OpApplied},
		{Status: domain.OpApplied},
		{Status: domain.OpDuplicate},
		{Status: domain.OpConflictLWW},
		{Status: domain.OpRejected},
	}

	got := summarise("batch-1", now, results, 42)

	if got.Applied != 2 || got.Duplicate != 1 || got.ConflictLWW != 1 || got.Rejected != 1 {
		t.Fatalf("counts wrong: %+v", got)
	}
	if got.NextCursor != 42 {
		t.Fatalf("cursor lost: %d", got.NextCursor)
	}
	// The sum must equal the number of operations, or the client cannot tell
	// whether the server silently dropped one.
	if got.Applied+got.Duplicate+got.ConflictLWW+got.Rejected != len(results) {
		t.Fatalf("counts do not add up: %+v", got)
	}
}

func TestPayloadStringValidation(t *testing.T) {
	cases := map[string]map[string]any{
		"missing":    {},
		"empty":      {"title": ""},
		"whitespace": {"title": "   "},
		"wrong type": {"title": 42},
	}
	for name, payload := range cases {
		if _, err := payloadString(payload, "title"); err == nil {
			t.Fatalf("%s: expected a validation error", name)
		}
	}

	got, err := payloadString(map[string]any{"title": "  hello  "}, "title")
	if err != nil || got != "hello" {
		t.Fatalf("expected trimmed value, got %q err=%v", got, err)
	}
}

func TestPayloadTimeIgnoresUnparseable(t *testing.T) {
	if got := payloadTime(map[string]any{"updatedAt": "not a time"}, "updatedAt"); got != nil {
		t.Fatal("an unparseable timestamp must fall back to server time, not crash the push")
	}

	ts := "2026-02-14T09:31:07.221Z"
	got := payloadTime(map[string]any{"updatedAt": ts}, "updatedAt")
	if got == nil || got.UTC().Format(time.RFC3339Nano) != ts {
		t.Fatalf("expected %s, got %v", ts, got)
	}
}

func TestScopeRoleChecks(t *testing.T) {
	scope := &domain.SyncScope{
		TableName: "sync_probe",
		PullRoles: []string{"*"},
		PushRoles: []string{"admin", "nurse"},
	}

	if !scope.AllowsPull(domain.RoleCashier) {
		t.Fatal("'{*}' must allow every role")
	}
	if !scope.AllowsPush(domain.RoleNurse) {
		t.Fatal("nurse is listed")
	}
	if scope.AllowsPush(domain.RoleCashier) {
		t.Fatal("cashier is not listed and must be refused")
	}
}

func TestCamelKey(t *testing.T) {
	cases := map[string]string{
		"server_rev":           "serverRev",
		"first_name":           "firstName",
		"id":                   "id",
		"created_by":           "createdBy",
		"currency_minor_units": "currencyMinorUnits",
	}
	for in, want := range cases {
		if got := camelKey(in); got != want {
			t.Fatalf("camelKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactRowsStripsAndCamelises(t *testing.T) {
	rows := []map[string]any{{
		"id":                 "u1",
		"first_name":         "Amina",
		"password_hash":      "$argon2id$secret",
		"failed_login_count": int64(2),
		"locked_until":       "2026-02-14T09:00:00Z",
		"server_rev":         int64(7),
	}}

	got := redactRows("users", rows)
	if len(got) != 1 {
		t.Fatalf("expected one row, got %d", len(got))
	}
	row := got[0]

	// The wire shape is camelCase.
	if row["firstName"] != "Amina" {
		t.Fatalf("firstName missing: %v", row)
	}
	if row["serverRev"] != int64(7) {
		t.Fatalf("serverRev missing or wrong type: %v", row["serverRev"])
	}

	// Nothing credential-shaped survives, under EITHER naming convention.
	for _, banned := range []string{"password_hash", "passwordHash", "failed_login_count",
		"failedLoginCount", "locked_until", "lockedUntil"} {
		if _, present := row[banned]; present {
			t.Fatalf("%s leaked into the pull response", banned)
		}
	}
}

func TestRedactRowsHasNoBannedColumnsForUnknownTables(t *testing.T) {
	// A table with no deny list must still be camelised and passed through.
	got := redactRows("sync_probe", []map[string]any{{"title": "hello", "server_rev": int64(1)}})
	if got[0]["title"] != "hello" || got[0]["serverRev"] != int64(1) {
		t.Fatalf("unexpected passthrough: %v", got[0])
	}
}
