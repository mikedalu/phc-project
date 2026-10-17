package middleware_test

// Tests for the device-signature middleware.
//
// These use a Go-generated keypair rather than the fixed interop vector: the
// point here is the REQUEST FLOW — header handling, ordering, replay, revocation
// — not the signature encoding, which internal/platform/devicekey already tests
// against a real WebCrypto vector.
//
// Every case below corresponds to a row in the security checklist
// (docs/05-SECURITY.md §12), which is what makes this test suite worth having.

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/httpx"
	"github.com/mikedalu/phc-records/api/internal/http/middleware"
	"github.com/mikedalu/phc-records/api/internal/platform/devicekey"
	"github.com/mikedalu/phc-records/api/internal/platform/nonce"
	"github.com/mikedalu/phc-records/api/internal/platform/token"
)

const (
	testPath   = "/api/v1/sync/push"
	testBody   = `{"batchId":"018f3c2a-1111-7abc-8def-000000000001"}`
	testSecret = "test-secret-test-secret-test-secret"
)

type fixture struct {
	router  *gin.Engine
	issuer  *token.Issuer
	device  *domain.Device
	key     *ecdsa.PrivateKey
	token   string
	nonces  *nonce.Memory
	deviceI uuid.UUID
}

func newFixture(t *testing.T, mutate func(*domain.Device)) *fixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal spki: %v", err)
	}

	device := &domain.Device{
		ID:           uuid.New(),
		Name:         "Test workstation",
		PublicKey:    base64.RawURLEncoding.EncodeToString(spki),
		PublicKeyAlg: "ES256",
		Fingerprint:  devicekey.Fingerprint(spki),
	}
	if mutate != nil {
		mutate(device)
	}

	issuer, err := token.NewIssuer([]byte(testSecret), time.Minute)
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	issued, err := issuer.Issue(uuid.New(), domain.RoleNurse, device.ID.String(), uuid.NewString())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	mem := nonce.NewMemory()
	lookup := func(_ context.Context, id uuid.UUID) (*domain.Device, error) {
		if id == device.ID {
			return device, nil
		}
		return nil, domain.ErrNotFound
	}

	router := gin.New()
	router.Use(httpx.RequestID())
	group := router.Group("/api/v1")
	group.Use(middleware.RequireAuth(issuer))
	// Order is load-bearing: CaptureBody must precede DeviceSignature, and
	// RequireAuth must precede both.
	group.Use(httpx.CaptureBody(1 << 20))
	group.Use(middleware.DeviceSignature(lookup, mem, 5*time.Minute))
	group.POST("/sync/push", func(c *gin.Context) {
		device, ok := middleware.DeviceFrom(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "device missing from context"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"deviceId": device.ID.String()})
	})

	return &fixture{
		router: router, issuer: issuer, device: device, key: key,
		token: issued.Token, nonces: mem, deviceI: device.ID,
	}
}

// sign produces a base64url DER signature over the canonical string.
func (f *fixture) sign(t *testing.T, timestamp, nonceValue string, body []byte) string {
	t.Helper()
	canonical := devicekey.CanonicalString(http.MethodPost, testPath, f.device.ID.String(), timestamp, nonceValue, body)
	digest := sha256.Sum256([]byte(canonical))
	sig, err := ecdsa.SignASN1(rand.Reader, f.key, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(sig)
}

func (f *fixture) do(t *testing.T, body []byte, headers map[string]string, withAuth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, testPath, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if withAuth {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
	}
	return envelope.Error.Code
}

func TestDeviceSignatureAcceptsValidRequest(t *testing.T) {
	f := newFixture(t, nil)
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	nonceValue := "nonce-valid-1"
	body := []byte(testBody)

	rec := f.do(t, body, map[string]string{
		middleware.HeaderDeviceSignature: f.sign(t, ts, nonceValue, body),
		middleware.HeaderDeviceTimestamp: ts,
		middleware.HeaderDeviceNonce:     nonceValue,
	}, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestDeviceSignatureRejectsReplay is the whole reason the nonce cache exists.
func TestDeviceSignatureRejectsReplay(t *testing.T) {
	f := newFixture(t, nil)
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	nonceValue := "nonce-replay-1"
	body := []byte(testBody)
	sig := f.sign(t, ts, nonceValue, body)

	headers := map[string]string{
		middleware.HeaderDeviceSignature: sig,
		middleware.HeaderDeviceTimestamp: ts,
		middleware.HeaderDeviceNonce:     nonceValue,
	}

	if rec := f.do(t, body, headers, true); rec.Code != http.StatusOK {
		t.Fatalf("first request should succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	rec := f.do(t, body, headers, true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("replay should be 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "DEVICE_NONCE_REPLAYED" {
		t.Fatalf("expected DEVICE_NONCE_REPLAYED, got %q", code)
	}
}

// TestDeviceSignatureRejectsTamperedBody proves the signature covers the body,
// not just the headers.
func TestDeviceSignatureRejectsTamperedBody(t *testing.T) {
	f := newFixture(t, nil)
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	nonceValue := "nonce-tamper-1"
	signedBody := []byte(testBody)
	tamperedBody := []byte(strings.Replace(testBody, "1111", "9999", 1))

	rec := f.do(t, tamperedBody, map[string]string{
		middleware.HeaderDeviceSignature: f.sign(t, ts, nonceValue, signedBody),
		middleware.HeaderDeviceTimestamp: ts,
		middleware.HeaderDeviceNonce:     nonceValue,
	}, true)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "DEVICE_SIGNATURE_INVALID" {
		t.Fatalf("expected DEVICE_SIGNATURE_INVALID, got %q", code)
	}
}

func TestDeviceSignatureRejectsStaleTimestamp(t *testing.T) {
	f := newFixture(t, nil)
	// 10 minutes old: outside the 5-minute window.
	ts := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339Nano)
	nonceValue := "nonce-stale-1"
	body := []byte(testBody)

	rec := f.do(t, body, map[string]string{
		middleware.HeaderDeviceSignature: f.sign(t, ts, nonceValue, body),
		middleware.HeaderDeviceTimestamp: ts,
		middleware.HeaderDeviceNonce:     nonceValue,
	}, true)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "CLOCK_SKEW" {
		t.Fatalf("expected CLOCK_SKEW, got %q", code)
	}
}

func TestDeviceSignatureRejectsMissingHeaders(t *testing.T) {
	f := newFixture(t, nil)

	rec := f.do(t, []byte(testBody), nil, true)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "DEVICE_SIGNATURE_MISSING" {
		t.Fatalf("expected DEVICE_SIGNATURE_MISSING, got %q", code)
	}
}

func TestDeviceSignatureRejectsRevokedDevice(t *testing.T) {
	now := time.Now()
	f := newFixture(t, func(d *domain.Device) {
		d.RevokedAt = &now
		d.RevokedBy = &d.ID
		d.RevokedReason = strPtr("laptop stolen")
	})

	ts := time.Now().UTC().Format(time.RFC3339Nano)
	nonceValue := "nonce-revoked-1"
	body := []byte(testBody)

	rec := f.do(t, body, map[string]string{
		middleware.HeaderDeviceSignature: f.sign(t, ts, nonceValue, body),
		middleware.HeaderDeviceTimestamp: ts,
		middleware.HeaderDeviceNonce:     nonceValue,
	}, true)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "DEVICE_REVOKED" {
		t.Fatalf("expected DEVICE_REVOKED, got %q", code)
	}
	// Crucially, the nonce was NOT consumed: a revoked device must not be able to
	// poison the cache for the same nonce if it is later re-enabled.
	if f.nonces.Len() != 0 {
		t.Fatalf("nonce cache should be empty for a rejected request, got %d entries", f.nonces.Len())
	}
}

// TestDeviceSignatureRejectsUnregisteredKey covers the state every device is in
// between login and POST /devices.
func TestDeviceSignatureRejectsUnregisteredKey(t *testing.T) {
	f := newFixture(t, func(d *domain.Device) {
		d.PublicKey = ""
		d.Fingerprint = "pending:" + d.ID.String()
	})

	ts := time.Now().UTC().Format(time.RFC3339Nano)
	nonceValue := "nonce-nokey-1"
	body := []byte(testBody)

	rec := f.do(t, body, map[string]string{
		middleware.HeaderDeviceSignature: f.sign(t, ts, nonceValue, body),
		middleware.HeaderDeviceTimestamp: ts,
		middleware.HeaderDeviceNonce:     nonceValue,
	}, true)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "DEVICE_KEY_NOT_REGISTERED" {
		t.Fatalf("expected DEVICE_KEY_NOT_REGISTERED, got %q", code)
	}
}

// TestDeviceSignatureAcceptsP1363 covers runtimes whose WebCrypto emits raw r||s.
func TestDeviceSignatureAcceptsP1363(t *testing.T) {
	f := newFixture(t, nil)
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	nonceValue := "nonce-p1363-1"
	body := []byte(testBody)

	canonical := devicekey.CanonicalString(http.MethodPost, testPath, f.device.ID.String(), ts, nonceValue, body)
	digest := sha256.Sum256([]byte(canonical))
	r, s, err := ecdsa.Sign(rand.Reader, f.key, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	raw := make([]byte, 64)
	r.FillBytes(raw[:32])
	s.FillBytes(raw[32:])

	rec := f.do(t, body, map[string]string{
		middleware.HeaderDeviceSignature: base64.RawURLEncoding.EncodeToString(raw),
		middleware.HeaderDeviceTimestamp: ts,
		middleware.HeaderDeviceNonce:     nonceValue,
	}, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for P1363 signature, got %d: %s", rec.Code, rec.Body.String())
	}
}

func strPtr(s string) *string { return &s }
