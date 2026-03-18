package version

import (
	"regexp"
	"testing"
)

// semverRE is a loose semver pattern that accepts optional pre-release labels
// like "-alpha", "-beta.1", etc.
var semverRE = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`)

func TestVersion_NonEmpty(t *testing.T) {
	t.Parallel()

	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}

func TestVersion_SemverFormat(t *testing.T) {
	t.Parallel()

	if !semverRE.MatchString(Version) {
		t.Errorf("Version %q does not match semver format (e.g. 1.2.3 or 1.2.3-alpha)", Version)
	}
}
