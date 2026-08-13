package leadpush

import "time"

const (
	// SDKName is the name sent to the Leadpush API for this SDK.
	SDKName = "leadpush-go"

	// SDKVersion is the version of this SDK.
	SDKVersion = "1.0.0"

	// APIVersion is the Leadpush API version used by this SDK.
	APIVersion = "v1"

	// DefaultBaseURL is the production Leadpush API base URL.
	DefaultBaseURL = "https://api.leadpush.io/" + APIVersion

	// DefaultUserAgent is sent with API requests unless overridden.
	DefaultUserAgent = SDKName + "/" + SDKVersion + " (api=" + APIVersion + ")"
)

// DefaultTimeout is the default maximum duration of an API request.
const DefaultTimeout = 30 * time.Second
