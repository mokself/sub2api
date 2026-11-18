package service

// [mok] Fork-only file (see FORK.md): the admin version badge checks this fork's releases.
//
// Upstream's UpdateService looks for new versions in Wei-Shaw/sub2api and can replace the
// running binary with an upstream release, which would drop every fork feature. The fork
// is released as a Docker image instead: pushing a tag mok-v<upstream version>.<build>
// builds ghcr.io/mokself/sub2api:<upstream version>-mok.<build> and publishes a GitHub
// release for the tag (.github/workflows/mok-release.yml).
//
// A build whose version carries the fork marker (0.2.21-mok.4, as the fork's images do)
// is a fork build. Through [mok] seams in update_service.go, for a fork build the update
// check reads the fork's releases and reports only the newer version number (no link to a
// release page and no word of upstream's releases), the admin UI shows the manual update
// path, and online update and rollback refuse to run. Versions such as 0.2.21-mok.4 compare
// correctly everywhere. Builds without the marker (a local `go run`, upstream's own tests)
// keep upstream's behaviour.

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// mokGitHubRepo is the repository whose releases the update check reads.
	mokGitHubRepo = "mokself/sub2api"
	// mokVersionMarker separates the upstream version from the fork build number.
	mokVersionMarker = "-mok."
	// mokUpdateBuildType makes the admin UI offer the manual update path (a link to the
	// release and a hint) instead of the online update and rollback buttons.
	mokUpdateBuildType = "source"
)

// ErrMokOnlineUpdateUnavailable is returned instead of downloading upstream binaries.
var ErrMokOnlineUpdateUnavailable = infraerrors.BadRequest(
	"MOK_ONLINE_UPDATE_UNAVAILABLE",
	"online update and rollback are not available for this fork; pull the new Docker image instead",
)

// mokReleaseTag matches the fork's release tags, e.g. mok-v0.2.21.4.
var mokReleaseTag = regexp.MustCompile(`^mok-v(\d+\.\d+\.\d+)\.(\d+)$`)

// mokVersionFromTag turns a release tag such as mok-v0.2.21.4 into the version the image
// reports, 0.2.21-mok.4.
func mokVersionFromTag(tag string) (string, bool) {
	match := mokReleaseTag.FindStringSubmatch(strings.TrimSpace(tag))
	if match == nil {
		return "", false
	}
	return match[1] + mokVersionMarker + match[2], true
}

// mokForkBuild reports whether the running version is one of the fork's builds.
func mokForkBuild(version string) bool {
	return strings.Contains(version, mokVersionMarker)
}

// mokUpdateServiceBuildType replaces the build type in NewUpdateService through a [mok] seam.
func mokUpdateServiceBuildType(version, buildType string) string {
	if mokForkBuild(version) {
		return mokUpdateBuildType
	}
	return buildType
}

// mokFetchLatestRelease replaces fetchLatestRelease in CheckUpdate through a [mok] seam: a
// fork build reads the latest release of the fork, which has no binaries to download. Only
// the version number is passed on: without release info the admin UI shows no link to the
// release page.
func (s *UpdateService) mokFetchLatestRelease(ctx context.Context) (*UpdateInfo, error) {
	if !mokForkBuild(s.currentVersion) {
		return s.fetchLatestRelease(ctx)
	}
	release, err := s.githubClient.FetchLatestRelease(ctx, mokGitHubRepo)
	if err != nil {
		return nil, err
	}
	latestVersion, ok := mokVersionFromTag(release.TagName)
	if !ok {
		return nil, fmt.Errorf("latest release %q of %s is not a fork release", release.TagName, mokGitHubRepo)
	}
	return &UpdateInfo{
		CurrentVersion: s.currentVersion,
		LatestVersion:  latestVersion,
		HasUpdate:      compareVersions(s.currentVersion, latestVersion) < 0,
		BuildType:      s.buildType,
	}, nil
}

// mokCheckCachedRelease is called by getFromCache through a [mok] seam. The cache outlives
// an upgrade (it is in Redis), so a fork build can find a result an upstream-checking build
// stored; such a result is treated as a cache miss and fetched again.
func mokCheckCachedRelease(currentVersion, cachedLatest string) error {
	if mokForkBuild(currentVersion) && !mokForkBuild(cachedLatest) {
		return fmt.Errorf("cached release %q is not a fork release", cachedLatest)
	}
	return nil
}

// mokCompareVersions is called first by compareVersions through a [mok] seam. When either
// version carries a fork build number (0.2.21-mok.4) it compares the upstream version and
// then the build number, where a plain upstream version counts as build 0. Otherwise it
// leaves the comparison to upstream (ok is false).
func mokCompareVersions(current, latest string) (result int, ok bool) {
	if !strings.Contains(current, mokVersionMarker) && !strings.Contains(latest, mokVersionMarker) {
		return 0, false
	}
	currentBase, currentBuild := mokSplitVersion(current)
	latestBase, latestBuild := mokSplitVersion(latest)
	currentParts, latestParts := parseVersion(currentBase), parseVersion(latestBase)
	for i := range currentParts {
		if currentParts[i] != latestParts[i] {
			if currentParts[i] < latestParts[i] {
				return -1, true
			}
			return 1, true
		}
	}
	switch {
	case currentBuild < latestBuild:
		return -1, true
	case currentBuild > latestBuild:
		return 1, true
	}
	return 0, true
}

func mokSplitVersion(version string) (base string, build int) {
	base, rest, found := strings.Cut(strings.TrimPrefix(strings.TrimSpace(version), "v"), mokVersionMarker)
	if !found {
		return base, 0
	}
	digits := rest
	if end := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' }); end >= 0 {
		digits = rest[:end]
	}
	build, _ = strconv.Atoi(digits)
	return base, build
}

// mokRejectOnlineUpdate guards PerformUpdate and fetchRollbackCandidates through [mok]
// seams: for a fork build both would install upstream binaries over the fork.
func mokRejectOnlineUpdate(version string) error {
	if mokForkBuild(version) {
		return ErrMokOnlineUpdateUnavailable
	}
	return nil
}
