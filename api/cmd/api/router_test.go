package main

// Route-level authorisation tests.
//
// ═══ WHY THIS TEST EXISTS ═══
//
// The RBAC rules live in the service layer and are tested there. This test
// answers a different, cheaper question: is every registered route actually
// behind the middleware it claims to be behind? A route added without
// `RequireAuth` (or an admin route without `RequireRole`) would pass every
// service test ever written, because the service would simply be reached by
// anyone.
//
// It walks `router.Routes()` rather than a hand-written list on purpose: a
// hand-written list is a second place to forget. Adding a route makes this test
// demand one of two things — a public allowlist entry with a reason, or a 401.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mikedalu/phc-records/api/internal/config"
	"github.com/mikedalu/phc-records/api/internal/domain"
	"github.com/mikedalu/phc-records/api/internal/http/handler"
	"github.com/mikedalu/phc-records/api/internal/platform/token"
)

// publicRoutes are reachable without a token, each for a stated reason.
var publicRoutes = map[string]string{
	"POST /api/v1/auth/login":          "how you obtain a token",
	"POST /api/v1/auth/refresh":        "how you renew a token",
	"POST /api/v1/auth/logout":         "must work with an expired access token",
	"POST /api/v1/auth/offline-verify": "cost parameters only; no secret in the response",
	"GET /api/v1/meta":                 "boot configuration; the client asks before it has a session",
	"GET /health":                      "Cloud Run liveness probe",
	"GET /ready":                       "Cloud Run readiness probe",
}

func testRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	issuer, err := token.NewIssuer([]byte("route-matrix-test-secret-32-bytes!!"), 15*time.Minute)
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}

	// Handlers are constructed with no dependencies. That is safe here: every
	// request in this test is REJECTED by middleware before a handler runs, so
	// nothing ever dereferences a nil service.
	// CORS needs at least one allowed origin or gin-contrib/cors panics with
	// "all origins disabled" — which is the correct default and worth keeping.
	cfg := &config.Config{AllowedOrigins: []string{"http://localhost:5173"}}
	return buildRouter(
		cfg,
		nil,
		handler.NewAuth(nil),
		handler.NewDevices(nil),
		handler.NewUsers(nil),
		handler.NewSync(nil),
		handler.NewPatients(nil),
		handler.NewHouseholds(nil),
		handler.NewEncounters(nil),
		handler.NewConsultation(nil),
		issuer,
	)
}

func routeKey(method, path string) string {
	return method + " " + path
}

// TestEveryRouteIsProtected is the table-driven assertion the sprint's
// Definition of Done asks for.
func TestEveryRouteIsProtected(t *testing.T) {
	router := testRouter(t)

	if len(router.Routes()) == 0 {
		t.Fatal("no routes registered — buildRouter changed shape")
	}

	for _, route := range router.Routes() {
		key := routeKey(route.Method, route.Path)

		if reason, isPublic := publicRoutes[key]; isPublic {
			if strings.TrimSpace(reason) == "" {
				t.Fatalf("%s is public but has no stated reason", key)
			}
			continue
		}

		t.Run(key, func(t *testing.T) {
			path := strings.ReplaceAll(route.Path, ":id", uuid.NewString())

			req := httptest.NewRequest(route.Method, path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s without a token: want 401, got %d (%s)", key, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestAdminRoutesRejectANonAdmin proves the role gate is wired, not just the
// auth gate. A nurse passes authentication and must still be refused.
func TestAdminRoutesRejectANonAdmin(t *testing.T) {
	router := testRouter(t)

	issuer, err := token.NewIssuer([]byte("route-matrix-test-secret-32-bytes!!"), 15*time.Minute)
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	issued, err := issuer.Issue(uuid.New(), domain.RoleNurse, uuid.NewString(), uuid.NewString())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	adminRoutes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodPatch, "/api/v1/admin/users/" + uuid.NewString()},
		{http.MethodPost, "/api/v1/admin/devices/" + uuid.NewString() + "/revoke"},
	}

	for _, route := range adminRoutes {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+issued.Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s as nurse: want 403, got %d (%s)",
				route.method, route.path, rec.Code, rec.Body.String())
		}
	}
}
