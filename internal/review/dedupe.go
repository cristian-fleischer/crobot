package review

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/cristian-fleischer/crobot/internal/platform"
)

// DedupeFindings compares new findings against existing bot comments and returns
// only findings that haven't been posted yet (based on fingerprint matching).
// Findings that match an existing comment's fingerprint are returned as
// duplicates.
func DedupeFindings(findings []platform.ReviewFinding, existing []platform.Comment) (newFindings []platform.ReviewFinding, duplicates []platform.ReviewFinding) {
	// Collect all fingerprints from existing comments.
	existingFPs := make(map[string]bool, len(existing))
	for _, c := range existing {
		// Use the Fingerprint field populated by the platform layer.
		if c.Fingerprint != "" {
			existingFPs[c.Fingerprint] = true
		} else {
			// Fallback: extract from body if Fingerprint field wasn't populated.
			fp := platform.ExtractFingerprint(c.Body)
			if fp != "" {
				existingFPs[fp] = true
			}
		}
	}

	for i := range findings {
		// Ensure every finding has a fingerprint.
		if findings[i].Fingerprint == "" {
			findings[i].Fingerprint = GenerateFingerprint(&findings[i])
		}

		if existingFPs[findings[i].Fingerprint] {
			duplicates = append(duplicates, findings[i])
		} else {
			newFindings = append(newFindings, findings[i])
		}
	}

	return newFindings, duplicates
}

// GenerateFingerprint creates a fingerprint for a finding if one isn't provided.
// The message is normalized (trimmed) before hashing for consistent results.
// Format: path:side:line:first-8-chars-of-message-hash
func GenerateFingerprint(f *platform.ReviewFinding) string {
	normalized := strings.TrimSpace(f.Message)
	hash := sha256.Sum256([]byte(normalized))
	hashPrefix := fmt.Sprintf("%x", hash[:4]) // 4 bytes = 8 hex chars
	return fmt.Sprintf("%s:%s:%d:%s", f.Path, f.Side, f.Line, hashPrefix)
}
