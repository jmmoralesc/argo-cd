package versions

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/Masterminds/semver/v3"
)

// MaxVersion takes a revision and a list of tags.
// If the revision is a version, it returns that version, even if it is not in the list of tags.
// If the revision is a constraint, it iterates over the list of tags to find the "maximum" tag which satisfies that
// constraint.
func MaxVersion(revision string, tags []string) (*semver.Version, error) {
	if v, err := semver.NewVersion(revision); err == nil {
		// If the revision is a valid version, then we know it isn't a constraint; it's just a pin.
		// In which case, we should use standard tag resolution mechanisms.
		// For example, the following are considered valid versions, and therefore should match an exact tag:
		// - "v1.0.0"/"1.0.0"
		// - "v1.0"/"1.0"
		return v, nil
	}

	constraints, err := semver.NewConstraint(revision)
	if err != nil {
		log.Debugf("Revision '%s' is not a valid semver constraint, skipping semver resolution.", revision)
		return nil, fmt.Errorf("failed to determine semver constraint: %w", err)
	}

	var maxVersion *semver.Version
	for _, tag := range tags {
		v, err := semver.NewVersion(tag)

		// Invalid semantic version ignored
		if errors.Is(err, semver.ErrInvalidSemVer) {
			log.Debugf("Invalid semantic version: %s", tag)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("invalid semver version in tags: %w", err)
		}
		if constraints.Check(v) {
			if maxVersion == nil || v.GreaterThan(maxVersion) {
				maxVersion = v
			}
		}
	}
	if maxVersion == nil {
		return nil, fmt.Errorf("version matching constraint not found in %d tags", len(tags))
	}

	log.Debugf("Semver constraint '%s' resolved to version '%s'", constraints.String(), maxVersion.Original())
	return maxVersion, nil
}
