//go:build unit

package service

// [mok] Fork-only tests for update_service_mok.go (see FORK.md).

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// mokReleaseClient records which repositories were asked for releases.
type mokReleaseClient struct {
	latest     *GitHubRelease
	latestErr  error
	latestRepo []string
	recentRepo []string
	downloads  int
}

func (c *mokReleaseClient) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	c.latestRepo = append(c.latestRepo, repo)
	return c.latest, c.latestErr
}

func (c *mokReleaseClient) FetchRecentReleases(_ context.Context, repo string, _ int) ([]*GitHubRelease, error) {
	c.recentRepo = append(c.recentRepo, repo)
	return []*GitHubRelease{{TagName: "v0.1.0"}}, nil
}

func (c *mokReleaseClient) DownloadFile(context.Context, string, string, int64) error {
	c.downloads++
	return errors.New("no downloads in tests")
}

func (c *mokReleaseClient) FetchChecksumFile(context.Context, string) ([]byte, error) {
	return nil, errors.New("no downloads in tests")
}

func TestMokVersionFromTag(t *testing.T) {
	for tag, want := range map[string]string{
		"mok-v0.2.21.4":  "0.2.21-mok.4",
		"mok-v1.10.0.12": "1.10.0-mok.12",
		" mok-v0.2.21.4": "0.2.21-mok.4",
	} {
		got, ok := mokVersionFromTag(tag)
		require.True(t, ok, tag)
		require.Equal(t, want, got, tag)
	}
	for _, tag := range []string{"v0.2.21", "mok-v0.2.21", "mok-v0.2.21.x", "mok-v0.2.21.4-rc1", ""} {
		_, ok := mokVersionFromTag(tag)
		require.False(t, ok, tag)
	}
}

func TestMokCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            int
	}{
		{"0.2.21-mok.4", "0.2.21-mok.5", -1},
		{"0.2.21-mok.9", "0.2.21-mok.10", -1},
		{"0.2.21-mok.5", "0.2.21-mok.5", 0},
		{"0.2.21-mok.5", "0.2.21-mok.4", 1},
		{"0.2.21-mok.7", "0.2.22-mok.1", -1},
		{"0.3.0-mok.1", "0.2.30-mok.9", 1},
		{"v0.2.21-mok.4", "0.2.21-mok.4", 0},
		// A plain upstream version counts as build 0 of that version.
		{"0.2.21", "0.2.21-mok.1", -1},
		{"0.2.21-mok.1", "0.2.21", 1},
		{"0.2.21-mok.1", "0.2.22", -1},
		// Without the fork marker upstream decides, ignoring other suffixes.
		{"0.2.21", "0.2.21-beta", 0},
		{"0.2.21", "0.2.22", -1},
	} {
		require.Equal(t, tc.want, compareVersions(tc.current, tc.latest), "%s vs %s", tc.current, tc.latest)
	}
}

func TestMokForkBuildChecksTheForkReleases(t *testing.T) {
	ctx := context.Background()
	client := &mokReleaseClient{latest: &GitHubRelease{
		TagName:     "mok-v0.2.21.5",
		Name:        "0.2.21-mok.5",
		Body:        "notes",
		PublishedAt: "2026-11-18T08:00:00Z",
		HTMLURL:     "https://github.com/mokself/sub2api/releases/tag/mok-v0.2.21.5",
		Assets:      []GitHubAsset{{Name: "sub2api_linux_amd64.tar.gz", BrowserDownloadURL: "https://github.com/x"}},
	}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-mok.4", "release")

	info, err := svc.CheckUpdate(ctx, true)

	require.NoError(t, err)
	require.Equal(t, []string{mokGitHubRepo}, client.latestRepo)
	require.Equal(t, "0.2.21-mok.4", info.CurrentVersion)
	require.Equal(t, "0.2.21-mok.5", info.LatestVersion)
	require.True(t, info.HasUpdate)
	require.Equal(t, "source", info.BuildType, "the admin UI offers the manual update path")
	require.Nil(t, info.ReleaseInfo, "no link to the release page and nothing to download")

	cached, err := svc.CheckUpdate(ctx, false)
	require.NoError(t, err)
	require.True(t, cached.Cached)
	require.True(t, cached.HasUpdate, "cached fork versions compare the same way")
	require.Equal(t, "source", cached.BuildType)
	require.Nil(t, cached.ReleaseInfo)

	upToDate := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-mok.5", "release")
	info, err = upToDate.CheckUpdate(ctx, true)
	require.NoError(t, err)
	require.False(t, info.HasUpdate)
}

func TestMokForkBuildReportsAReleaseThatIsNotAForkRelease(t *testing.T) {
	client := &mokReleaseClient{latest: &GitHubRelease{TagName: "v0.2.22"}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-mok.4", "release")

	info, err := svc.CheckUpdate(context.Background(), true)

	require.NoError(t, err, "upstream's CheckUpdate turns fetch errors into a warning")
	require.False(t, info.HasUpdate)
	require.Contains(t, info.Warning, "not a fork release")
}

func TestMokForkBuildRefusesOnlineUpdateAndRollback(t *testing.T) {
	ctx := context.Background()
	client := &mokReleaseClient{latest: &GitHubRelease{TagName: "mok-v0.2.21.5"}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21-mok.4", "release")

	require.ErrorIs(t, svc.PerformUpdate(ctx), ErrMokOnlineUpdateUnavailable)
	_, err := svc.ListRollbackVersions(ctx)
	require.ErrorIs(t, err, ErrMokOnlineUpdateUnavailable)
	require.ErrorIs(t, svc.RollbackToVersion(ctx, "0.1.0"), ErrMokOnlineUpdateUnavailable)

	require.Empty(t, client.latestRepo, "nothing is fetched")
	require.Empty(t, client.recentRepo)
	require.Zero(t, client.downloads)
}

func TestMokBuildsWithoutTheForkMarkerKeepUpstreamBehaviour(t *testing.T) {
	ctx := context.Background()
	client := &mokReleaseClient{latest: &GitHubRelease{TagName: "v0.2.22"}}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.2.21", "release")

	info, err := svc.CheckUpdate(ctx, true)
	require.NoError(t, err)
	require.Equal(t, []string{githubRepo}, client.latestRepo)
	require.Equal(t, "0.2.22", info.LatestVersion)
	require.True(t, info.HasUpdate)
	require.Equal(t, "release", info.BuildType)

	_, err = svc.ListRollbackVersions(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{githubRepo}, client.recentRepo)
}

func TestMokForkBuildIgnoresUpstreamResultsCachedBeforeTheUpgrade(t *testing.T) {
	ctx := context.Background()
	// What the previous image (checking upstream) stored in Redis.
	cache := &updateServiceCacheStub{}
	upstream := NewUpdateService(cache, &mokReleaseClient{latest: &GitHubRelease{
		TagName: "v0.2.22",
		HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.22",
	}}, "0.2.21", "release")
	_, err := upstream.CheckUpdate(ctx, true)
	require.NoError(t, err)
	require.NotEmpty(t, cache.data)

	client := &mokReleaseClient{latest: &GitHubRelease{TagName: "mok-v0.2.21.5"}}
	svc := NewUpdateService(cache, client, "0.2.21-mok.5", "release")
	info, err := svc.CheckUpdate(ctx, false)

	require.NoError(t, err)
	require.Equal(t, []string{mokGitHubRepo}, client.latestRepo, "the upstream result is not used")
	require.False(t, info.Cached)
	require.Equal(t, "0.2.21-mok.5", info.LatestVersion)
	require.False(t, info.HasUpdate)
	require.Nil(t, info.ReleaseInfo, "no link to upstream's release page")

	// The fork's own result is cached and used as usual.
	info, err = svc.CheckUpdate(ctx, false)
	require.NoError(t, err)
	require.True(t, info.Cached)
	require.Len(t, client.latestRepo, 1)
}
