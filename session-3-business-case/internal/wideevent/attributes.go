package wideevent

import (
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Attributes is the canonical mapping from an Event to the attributes that
// reach Honeycomb. It lives here rather than in the seeder so the width of the
// event — the whole premise of the Arbitrary Question Test — is testable
// without standing up an exporter.
//
// Naming follows the book's Chapter 6 tables where it has an opinion, and OTel
// semantic conventions otherwise. See the package comment for the specifics,
// including why geo.* is written as plain strings.
func Attributes(e Event) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		// Request and execution flow (Table 6-8, 6-11)
		semconv.HTTPRequestMethodPost,
		semconv.HTTPRoute(e.Route),
		semconv.URLPath(e.URLPath),
		semconv.HTTPResponseStatusCode(e.StatusCode),
		attribute.Int64("db.query.duration_ms", e.DurationDB),
		attribute.Int("request.local_hour", e.LocalHour),

		// User and business context (Table 6-15)
		attribute.String("user.id", e.UserID),
		attribute.String("user.type", e.UserType),
		attribute.String("user.org.id", e.UserOrgID),
		attribute.String("user.auth_method", e.AuthMethod),

		// Client (Tables 6-9, 6-10). "mobile users" in the book's question
		// means user_agent.device == "phone"; there is no "mobile" value.
		attribute.String("user_agent.device", e.Device),
		attribute.String("user_agent.OS", e.OSName),

		// Geography. OTel semconv names, Development stability; California is
		// US-CA in ISO 3166-2.
		attribute.String("geo.country.iso_code", e.CountryISO),
		attribute.String("geo.region.iso_code", e.RegionISO),

		// Localization (Table 6-18)
		attribute.String("localization.language", e.Language),
		attribute.String("localization.currency", e.Currency),

		// Service and code context (Tables 6-1, 6-5, 6-6)
		attribute.String("service.version", e.ServiceVersion),
		attribute.String("service.environment", e.ServiceEnv),
		attribute.String("service.build.git_hash", e.BuildGitHash),
		attribute.Int64("service.build.deployment.age_minutes", e.DeployAgeMin),
		attribute.Bool("feature_flag.new_checkout_flow", e.FeatureFlagNew),

		// Rate limits (Table 6-16)
		attribute.Int64("ratelimit.limit", e.RateLimitLimit),
		attribute.Int64("ratelimit.remaining", e.RateLimitRemaining),

		// Async request summaries (Table 6-13)
		attribute.Int64("stats.postgres_query_count", e.PostgresQueryCount),
		attribute.Int64("stats.redis_query_count", e.RedisQueryCount),

		// Infrastructure (Table 6-2, 6-3)
		attribute.String("instance.type", e.InstanceType),
		attribute.String("cloud.region", e.CloudRegion),
	}

	// Only mobile clients report an app and version, so these are appended
	// conditionally rather than emitted empty.
	if e.App != "" {
		attrs = append(attrs,
			attribute.String("user_agent.app", e.App),
			attribute.String("user_agent.app_version", e.AppVersion),
		)
	}

	// Errors (Table 6-14). error is a literal boolean because that is what the
	// queries filter on.
	if e.Errored {
		attrs = append(attrs,
			attribute.Bool("error", true),
			attribute.String("error.type", e.ErrorType),
			attribute.String("exception.slug", e.ErrorSlug),
		)
	}

	return attrs
}
