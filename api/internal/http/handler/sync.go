package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/http/middleware"
	"github.com/mikedalu/phc-records/api/internal/service"
)

// Sync handles the replication endpoints.
type Sync struct {
	svc *service.Sync
}

// NewSync builds the handler.
func NewSync(svc *service.Sync) *Sync { return &Sync{svc: svc} }

type pushOperationRequest struct {
	OperationID      string         `json:"operationId"`
	Table            string         `json:"table"`
	RowID            string         `json:"rowId"`
	Operation        string         `json:"operation"`
	BaseServerRev    *int64         `json:"baseServerRev"`
	DeviceRecordedAt *string        `json:"deviceRecordedAt"`
	Payload          map[string]any `json:"payload"`
}

type pushRequest struct {
	BatchID       string `json:"batchId"`
	DeviceID      string `json:"deviceId"`
	ClientSentAt  string `json:"clientSentAt"`
	SchemaVersion int    `json:"schemaVersion"`
	// PendingCount is the client's own queue depth after this batch. Surfaced to
	// admins so a workstation with a stuck queue is visible before the user calls.
	PendingCount int                    `json:"pendingOperationCount"`
	Operations   []pushOperationRequest `json:"operations"`
}

type opResultDTO struct {
	OperationID string `json:"operationId"`
	Status      string `json:"status"`
	RowID       string `json:"rowId"`
	ServerRev   int64  `json:"serverRev,omitempty"`
	Reason      string `json:"reason,omitempty"`
	// Only present for conflict_lww.
	KeptServerVersion *bool          `json:"keptServerVersion,omitempty"`
	ConflictID        string         `json:"conflictId,omitempty"`
	ServerPayload     map[string]any `json:"serverPayload,omitempty"`
}

type pushSummaryDTO struct {
	Applied     int `json:"applied"`
	Duplicate   int `json:"duplicate"`
	ConflictLWW int `json:"conflictLww"`
	Rejected    int `json:"rejected"`
}

type pushResponseDTO struct {
	BatchID                string         `json:"batchId"`
	ServerTime             string         `json:"serverTime"`
	Results                []opResultDTO  `json:"results"`
	Summary                pushSummaryDTO `json:"summary"`
	NextCursor             int64          `json:"nextCursor"`
	ClientClockSkewSeconds int64          `json:"clientClockSkewSeconds"`
}

// Push handles POST /api/v1/sync/push.
//
// Mounted behind RequireAuth, CaptureBody and DeviceSignature. By the time this
// runs, the request is known to come from an authenticated user ON THE DEVICE
// THE TOKEN WAS ISSUED TO, and its body is exactly the bytes that were signed.
func (h *Sync) Push(c *gin.Context) {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication required.")
		return
	}

	var req pushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "MALFORMED_REQUEST", "Request body must be valid JSON.", nil)
		return
	}

	// The body's deviceId must agree with the signed device. A mismatch means a
	// client bug at best and a probe at worst; either way it is refused.
	if req.DeviceID != "" {
		claimed, err := uuid.Parse(req.DeviceID)
		if err != nil || claimed != actor.DeviceID {
			httpx.BadRequest(c, "VALIDATION_FAILED",
				"deviceId does not match the authenticated device.", nil)
			return
		}
	}

	var clientSentAt *time.Time
	if req.ClientSentAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, req.ClientSentAt); err == nil {
			clientSentAt = &parsed
		}
	}

	ops := make([]service.PushOperation, 0, len(req.Operations))
	for _, op := range req.Operations {
		var recorded *time.Time
		if op.DeviceRecordedAt != nil && *op.DeviceRecordedAt != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, *op.DeviceRecordedAt); err == nil {
				recorded = &parsed
			}
		}
		ops = append(ops, service.PushOperation{
			ID:               op.OperationID,
			Table:            op.Table,
			RowID:            op.RowID,
			Operation:        op.Operation,
			BaseServerRev:    op.BaseServerRev,
			DeviceRecordedAt: recorded,
			Payload:          op.Payload,
		})
	}

	result, err := h.svc.Push(c.Request.Context(), service.PushInput{
		UserID:       actor.UserID,
		DeviceID:     actor.DeviceID,
		BatchID:      req.BatchID,
		ClientSentAt: clientSentAt,
		PendingCount: req.PendingCount,
		Operations:   ops,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}

	var skew int64
	if clientSentAt != nil {
		skew = int64(clientSentAt.Sub(result.ServerTime).Seconds())
	}

	httpx.OK(c, pushResponseDTO{
		BatchID:                result.BatchID,
		ServerTime:             result.ServerTime.UTC().Format(time.RFC3339Nano),
		Results:                toOpResultDTOs(result.Results),
		Summary:                pushSummaryDTO{Applied: result.Applied, Duplicate: result.Duplicate, ConflictLWW: result.ConflictLWW, Rejected: result.Rejected},
		NextCursor:             result.NextCursor,
		ClientClockSkewSeconds: skew,
	})
}

func toOpResultDTOs(results []domain.OpResult) []opResultDTO {
	out := make([]opResultDTO, 0, len(results))
	for _, r := range results {
		dto := opResultDTO{
			OperationID: r.OperationID,
			Status:      string(r.Status),
			RowID:       r.RowID,
			ServerRev:   r.ServerRev,
			Reason:      r.Reason,
			ConflictID:  r.ConflictID,
		}
		if r.Status == domain.OpConflictLWW {
			kept := r.KeptServerVersion
			dto.KeptServerVersion = &kept
			dto.ServerPayload = r.ServerPayload
		}
		out = append(out, dto)
	}
	return out
}
