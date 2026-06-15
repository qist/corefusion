package netutil

import (
	"net/url"
	"strings"
)

// githubHosts are the hostnames that belong to GitHub.
var githubHosts = map[string]bool{
	"github.com":                true,
	"api.github.com":            true,
	"raw.githubusercontent.com": true,
	"gist.github.com":           true,
	"objects.githubusercontent.com": true,
}

// IsGitHubURL checks whether rawURL points to a GitHub host.
func IsGitHubURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return githubHosts[host]
}

// AccelerateGitHubURL rewrites a GitHub URL to go through an acceleration
// proxy. Non-GitHub URLs are returned unchanged.
//
// Example:
//
//	AccelerateGitHubURL("https://api.github.com/repos/x/y", "https://ghproxy.com")
//	→ "https://ghproxy.com/https://api.github.com/repos/x/y"
func AccelerateGitHubURL(rawURL, accelDomain string) string {
	if accelDomain == "" {
		return rawURL
	}
	if !IsGitHubURL(rawURL) {
		return rawURL
	}
	// Trim trailing slash from accel domain
	accelDomain = strings.TrimRight(accelDomain, "/")
	return accelDomain + "/" + rawURL
}
