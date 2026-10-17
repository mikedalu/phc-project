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

// PullChange is one entry of the ordered pull manifest.
type PullChange struct {
	Seq       int64
	Table     string
	RowID     string
	Operation string
	ServerRev int64
}

// PullResult is one page of changes plus the current state of every row named.
//
// `Changes` and `Rows` are deliberately separate: the manifest is ordered and
// advances the cursor, while a row appears in `Rows` once however many times it
// changed in the window.
type PullResult struct {
	Changes []PullChange
	Rows    map[string][]map[string]any

	// NextCursor is the seq of the LAST change READ, not max(seq) of the table.
	// Using max(seq) skips anything committed between the read and the query.
	NextCursor int64
	HasMore    bool

	FullResyncRequired bool
	Reason             string
	OldestAvailableSeq int64

	ServerTime time.Time
}

// SnapshotResult is one keyset page of a table's current state.
type SnapshotResult struct {
	Table         string
	Rows          []map[string]any
	HasMore       bool
	CursorAtStart int64
}
