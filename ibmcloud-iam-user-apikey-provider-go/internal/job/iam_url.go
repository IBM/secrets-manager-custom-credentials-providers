package job

import (
	"fmt"
	"net/url"
	"strings"
)

// allowedIAMURLs is the hardcoded allowlist of IAM endpoints that the SMIN_URL parameter may point to.
//
// SECURITY: the IBM Cloud API key read from the secret referenced by SMIN_APIKEY_SECRET_ID is sent to the
// configured IAM endpoint. SMIN_URL is supplied by whoever creates the secret, so it must never be trusted
// as-is. It is only accepted if it matches one of the entries below.
//
// If you need to override the IAM URL you MUST first add
// the allowed value(s) to this list, and only then build and deploy the provider.
var allowedIAMURLs = []string{
	"https://iam.cloud.ibm.com",
}

// resolveIAMURL validates the user supplied SMIN_URL value against allowedIAMURLs and returns the IAM URL to use.
//
// An empty value is returned as is (no validation needed): the IAM SDK then applies its own default endpoint.
// A single trailing slash is tolerated ("https://iam.cloud.ibm.com/").
// Anything else besides the exact scheme and host of an allowlisted entry is rejected: userinfo, paths,
// queries, fragments, opaque URLs, whitespace and backslashes.
//
// The returned value is always the allowlist entry itself, never the raw user input.
func resolveIAMURL(userURL string) (string, error) {
	if userURL == "" {
		return "", nil
	}

	canonical, err := canonicalOrigin(userURL)
	if err != nil {
		return "", fmt.Errorf("the value of SMIN_URL (%q) is not a valid IAM URL: %s", userURL, err.Error())
	}

	for _, allowed := range allowedIAMURLs {
		if canonical == strings.ToLower(strings.TrimSuffix(allowed, "/")) {
			return allowed, nil
		}
	}

	return "", fmt.Errorf("the value of SMIN_URL (%q) is not an allowed IAM URL", userURL)
}

// canonicalOrigin strictly parses rawURL and returns "https://<lowercase host>".
// It fails unless rawURL consists solely of the https scheme, a host and (optionally) a single trailing slash.
func canonicalOrigin(rawURL string) (string, error) {
	if strings.ContainsAny(rawURL, " \t\r\n\\@?#") {
		return "", fmt.Errorf("it must not contain whitespace, backslashes, userinfo, a query or a fragment")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("it cannot be parsed as a URL")
	}

	switch {
	case u.Scheme != "https":
		return "", fmt.Errorf("the scheme must be https")
	case u.Opaque != "":
		return "", fmt.Errorf("it must not be an opaque URL")
	case u.User != nil:
		return "", fmt.Errorf("it must not contain userinfo")
	case u.RawQuery != "" || u.ForceQuery:
		return "", fmt.Errorf("it must not contain a query")
	case u.Fragment != "" || u.RawFragment != "":
		return "", fmt.Errorf("it must not contain a fragment")
	case u.Host == "":
		return "", fmt.Errorf("it must contain a host")
	case u.Host != u.Hostname():
		return "", fmt.Errorf("it must not contain a port")
	case u.RawPath != "" || (u.Path != "" && u.Path != "/"):
		return "", fmt.Errorf("it must not contain a path")
	}

	return "https://" + strings.ToLower(u.Hostname()), nil
}
