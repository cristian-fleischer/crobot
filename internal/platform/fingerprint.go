package platform

import "regexp"

// fingerprintRe matches the hidden CRoBot fingerprint in the markdown
// reference-link comment format: [//]: # "crobot:fp=VALUE"
// This format is invisible in rendered markdown across all major platforms.
var fingerprintRe = regexp.MustCompile(`\[//\]: # "crobot:fp=(.+?)"`)

// ExtractFingerprint extracts the CRoBot fingerprint from a comment body. It
// returns an empty string if no fingerprint marker is found.
func ExtractFingerprint(body string) string {
	matches := fingerprintRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}
