package hubapi

import "github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"

// Shared codes: spelled, and meaning, exactly what httpx means (contract §4.2).
// They are httpx's constants, not copies, so the two can never drift.
const (
	CodeInvalidJSON     = httpx.CodeInvalidJSON     // 400
	CodeBadRequest      = httpx.CodeBadRequest      // 400
	CodeForbiddenOrigin = httpx.CodeForbiddenOrigin // 403
	CodeConflict        = httpx.CodeConflict        // 409
	CodeInternal        = httpx.CodeInternal        // 500
)

// The Hub's own codes, contract §4.2, in its table order. project_not_found is
// spelled like httpx.CodeProjectNotFound on purpose: the relay passes a Hub 404
// to the desktop unchanged, and "no such project that you can see" reads the
// same from either process.
const (
	CodeUnauthenticated       = "unauthenticated"         // 401, details.reason
	CodeTenantNotEnrolled     = "tenant_not_enrolled"     // 403, details.tenantId
	CodeForbidden             = "forbidden"               // 403, details.required
	CodeNotReserver           = "not_reserver"            // 403, details.releaseId
	CodeTeamNotFound          = "team_not_found"          // 404
	CodeProjectNotFound       = "project_not_found"       // 404
	CodeBriefNotFound         = "brief_not_found"         // 404
	CodeMemberNotFound        = "member_not_found"        // 404
	CodeInviteNotFound        = "invite_not_found"        // 404
	CodeApprovalNotFound      = "approval_not_found"      // 404
	CodeReleaseNotFound       = "release_not_found"       // 404
	CodeBlobNotFound          = "blob_not_found"          // 404
	CodeRouteNotFound         = "route_not_found"         // 404
	CodeMethodNotAllowed      = "method_not_allowed"      // 405
	CodeRevisionConflict      = "revision_conflict"       // 409
	CodeBriefExists           = "brief_exists"            // 409
	CodeBriefArchived         = "brief_archived"          // 409
	CodeDesignAlreadyApproved = "design_already_approved" // 409
	CodeRepoAlreadyLinked     = "repo_already_linked"     // 409
	CodeLastAdmin             = "last_admin"              // 409
	CodeReleaseState          = "release_state"           // 409
	CodeReleaseExpired        = "release_expired"         // 409
	CodeQuotaExceeded         = "quota_exceeded"          // 409
	CodeBlobTooLarge          = "blob_too_large"          // 413
	CodeBriefIncomplete       = "brief_incomplete"        // 422
	CodeKeyUnresolvable       = "key_unresolvable"        // 422
	CodeDigestMismatch        = "digest_mismatch"         // 422
	CodeBlobMissing           = "blob_missing"            // 422
	CodeDesignNotApproved     = "design_not_approved"     // 422
	CodeRepoNotLinked         = "repo_not_linked"         // 422
	CodeIdempotencyMismatch   = "idempotency_mismatch"    // 422
	CodeRateLimited           = "rate_limited"            // 429, details.scope
	CodeUnavailable           = "unavailable"             // 503
)

// Reasons carried in details.reason on a 401 unauthenticated (contract §4.2).
const (
	ReasonMissing     = "missing"
	ReasonMalformed   = "malformed"
	ReasonSignature   = "signature"
	ReasonExpired     = "expired"
	ReasonNotYetValid = "not_yet_valid"
	ReasonIssuer      = "issuer"
	ReasonAudience    = "audience"
	ReasonScope       = "scope"
	ReasonClient      = "client"
	ReasonVersion     = "version"
)
