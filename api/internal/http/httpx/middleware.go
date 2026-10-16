package httpx

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/mikedalu/phc-records/api/internal/config"
)

// CORS builds the CORS middleware from the configured allow-list.
//
// The SPA is served from a different origin (Cloudflare Pages) than the API
// (Cloud Run), so CORS is mandatory in every environment except a same-origin
// local setup.
//
// Deliberately NOT using AllowAllOrigins: browsers reject
// `Access-Control-Allow-Origin: *` on credentialed requests anyway, and an
// explicit allow-list means a mistyped deployment cannot silently expose the API
// to any web page.
func CORS(cfg *config.Config) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,

		// Bearer tokens live in the Authorization header, not in cookies. There
		// is therefore no cookie to allow, and no CSRF surface to worry about —
		// see docs/05-SECURITY.md T11.
		AllowCredentials: false,

		AllowMethods: []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},

		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			HeaderRequestID,
			// Sync-protocol headers (Sprint 2). Declared now so the browser
			// preflight does not reject the first sync push the day it lands.
			"X-Device-Id",
			"X-Device-Signature",
			"X-Device-Timestamp",
			"X-Device-Nonce",
			"Idempotency-Key",
		},

		// Without this the browser cannot read the request ID we echo back,
		// which would make client-side error reports untraceable.
		ExposeHeaders: []string{HeaderRequestID},

		MaxAge: 12 * time.Hour,
	})
}

// SecurityHeaders sets a small set of defence-in-depth response headers.
//
// This is an API, so the important ones are nosniff (stop a browser guessing a
// content type) and no-store on API responses (never let a shared machine cache
// patient data). The full CSP belongs on the SPA, not here.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")

		// HSTS only makes sense over TLS; Cloudflare terminates TLS in
		// production. Setting it locally over HTTP is ignored by browsers
		// anyway, but we keep the intent explicit.
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		c.Next()
	}
}
