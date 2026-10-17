package domain

import "time"

// Sync domain types.
//
// These mirror docs/02-SYNC-PROTOCOL.md §5 exactly. Kept in domain rather than
// the HTTP layer so the service can be tested without a router, and so the
// handler cannot invent a status the service does not know.

// SyncOperation is one queued local mutation being pushed.
type SyncOperation struct {
	ID               string
	Table            string
	RowID            string
	Operation        string // insert | update | delete
	BaseServerRev    *int64
	DeviceRecordedAt *time.Time
	Payload          map[string]any
}

// OpStatus is the per-operation result the client must handle.
//
// There are four, not two, and `rejected` is the one that gets forgotten:
// retrying a permanently rejected operation forever produces a device that
// hammers the server and never drains; dropping it silently loses a nurse's
// record with no trace.
type OpStatus string

const (
	OpApplied     OpStatus = "applied"
	OpDuplicate   OpStatus = "duplicate"
	OpConflictLWW OpStatus = "conflict_lww"
	OpRejected    OpStatus = "rejected"
)

// OpResult is one row of the push response.
type OpResult struct {
	OperationID string
	Status      OpStatus
	RowID       string
	ServerRev   int64
	Reason      string

	// Set only for conflict_lww: which side the server kept, and the payload it
	// kept, so the client can overwrite its local copy without a second pull.
	KeptServerVersion bool
	ConflictID        string
	ServerPayload     map[string]any
}

// SyncScope is one row of sync_table_scopes: the engine's per-table policy.
type SyncScope struct {
	TableName      string
	SyncEnabled    bool
	PullRoles      []string
	PushRoles      []string
	ConflictPolicy string // append_only | lww
	RetentionDays  *int
	Notes          *string
}

// AllowsPush reports whether a role may write this table.
//
// '{*}' means every role. The check is on the token's role, but the service
// re-reads it from the database before pushing — a role revoked fifteen minutes
// ago must not keep writing.
func (s *SyncScope) AllowsPush(role Role) bool {
	return containsRole(s.PushRoles, role)
}

// AllowsPull reports whether a role may receive this table.
func (s *SyncScope) AllowsPull(role Role) bool {
	return containsRole(s.PullRoles, role)
}

func containsRole(list []string, role Role) bool {
	for _, entry := range list {
		if entry == "*" || entry == role.String() {
			return true
		}
	}
	return false
}

// BatchResult is the whole push response.
type BatchResult struct {
	BatchID     string
	ServerTime  time.Time
	Results     []OpResult
	NextCursor  int64
	Applied     int
	Duplicate   int
	ConflictLWW int
	Rejected    int
}
