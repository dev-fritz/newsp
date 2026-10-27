package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/dev-fritz/newsp/api/internal/auth"
	"github.com/dev-fritz/newsp/api/internal/models"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/pkg/connectors"
)

func strPtr(s string) *string { return &s }

func TestCreateSourceSecret(t *testing.T) {
	t.Parallel()
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	ctx := context.Background()
	config := json.RawMessage(`{"limit":10}`)

	expectKind := func(d sourceDeps, mode connectors.SecretMode) {
		d.registry.EXPECT().Get("github").Return(d.connector, true)
		d.connector.EXPECT().Normalize(gomock.Any()).Return(config, nil)
		d.connector.EXPECT().Secret(config).Return(mode)
	}

	t.Run("sealed at rest and never stored in plain text", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		expectKind(d, connectors.SecretOptional)
		var stored models.Source
		d.sources.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s models.Source) error {
			stored = s
			return nil
		})
		_, err := svc.Create(ctx, admin, services.SourceInput{Kind: "github", Name: "GH", Config: config, Secret: strPtr("  ghp_token  ")})
		require.NoError(t, err)
		require.NotEmpty(t, stored.SecretEnc)
		assert.NotContains(t, string(stored.SecretEnc), "ghp_token")
		plain, err := d.box.Open(stored.SecretEnc)
		require.NoError(t, err)
		assert.Equal(t, "ghp_token", string(plain), "surrounding spaces are trimmed")
	})

	t.Run("required secret missing", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		expectKind(d, connectors.SecretRequired)
		_, err := svc.Create(ctx, admin, services.SourceInput{Kind: "github", Name: "GH", Config: config, Secret: strPtr("   ")})
		requireFieldError(t, err, "secret", "required")
	})

	t.Run("kind without credential refuses one", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		expectKind(d, connectors.SecretNone)
		_, err := svc.Create(ctx, admin, services.SourceInput{Kind: "github", Name: "GH", Config: config, Secret: strPtr("x")})
		requireFieldError(t, err, "secret", "unsupported")
	})

	t.Run("optional secret may be left out", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		expectKind(d, connectors.SecretOptional)
		d.sources.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s models.Source) error {
			assert.Nil(t, s.SecretEnc)
			return nil
		})
		_, err := svc.Create(ctx, admin, services.SourceInput{Kind: "github", Name: "GH", Config: config})
		require.NoError(t, err)
	})
}

func TestUpdateSourceSecret(t *testing.T) {
	t.Parallel()
	admin := auth.Identity{UserID: uuid.New(), Role: models.RoleAdmin}
	ctx := context.Background()

	withSecret := func(t *testing.T, d sourceDeps, plain string) models.Source {
		t.Helper()
		src := testSource(0)
		src.Kind = models.SourceGitHub
		sealed, err := d.box.Seal([]byte(plain))
		require.NoError(t, err)
		src.SecretEnc = sealed
		return src
	}
	expectSave := func(d sourceDeps, src models.Source, check func(models.Source)) {
		d.sources.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s models.Source) error {
			check(s)
			return nil
		})
		d.sources.EXPECT().GetWithStats(gomock.Any(), src.ID, gomock.Any()).Return(repositories.SourceStats{Source: src}, nil)
	}

	t.Run("omitted secret is kept without asking the connector", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		src := withSecret(t, d, "old")
		d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
		expectSave(d, src, func(s models.Source) { assert.Equal(t, src.SecretEnc, s.SecretEnc) })
		_, err := svc.Update(ctx, admin, src.ID, services.SourceChange{Name: strPtr("Renamed")})
		require.NoError(t, err)
	})

	t.Run("new secret replaces the old one", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		src := withSecret(t, d, "old")
		d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
		d.registry.EXPECT().Get("github").Return(d.connector, true)
		d.connector.EXPECT().Secret(src.Config).Return(connectors.SecretOptional)
		expectSave(d, src, func(s models.Source) {
			plain, err := d.box.Open(s.SecretEnc)
			require.NoError(t, err)
			assert.Equal(t, "new", string(plain))
		})
		_, err := svc.Update(ctx, admin, src.ID, services.SourceChange{Secret: strPtr("new")})
		require.NoError(t, err)
	})

	t.Run("empty secret removes an optional one", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		src := withSecret(t, d, "old")
		d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
		d.registry.EXPECT().Get("github").Return(d.connector, true)
		d.connector.EXPECT().Secret(src.Config).Return(connectors.SecretOptional)
		expectSave(d, src, func(s models.Source) { assert.Nil(t, s.SecretEnc) })
		_, err := svc.Update(ctx, admin, src.ID, services.SourceChange{Secret: strPtr("")})
		require.NoError(t, err)
	})

	t.Run("a required secret cannot be removed", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		src := withSecret(t, d, "old")
		d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
		d.registry.EXPECT().Get("github").Return(d.connector, true)
		d.connector.EXPECT().Secret(src.Config).Return(connectors.SecretRequired)
		_, err := svc.Update(ctx, admin, src.ID, services.SourceChange{Secret: strPtr("")})
		requireFieldError(t, err, "secret", "required")
	})

	t.Run("a config that starts requiring a secret needs one", func(t *testing.T) {
		t.Parallel()
		svc, d := newSourceService(t)
		src := testSource(0)
		src.Kind = models.SourceBluesky
		search := json.RawMessage(`{"mode":"search","query":"saas"}`)
		d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
		d.registry.EXPECT().Get("bluesky").Return(d.connector, true)
		d.connector.EXPECT().Normalize(gomock.Any()).Return(search, nil)
		d.connector.EXPECT().Secret(search).Return(connectors.SecretRequired)
		_, err := svc.Update(ctx, admin, src.ID, services.SourceChange{Config: search})
		requireFieldError(t, err, "secret", "required")
	})
}

func TestCollectSourcePassesTheSecret(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	src := testSource(0)
	sealed, err := d.box.Seal([]byte("token-123"))
	require.NoError(t, err)
	src.SecretEnc = sealed
	d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
	d.registry.EXPECT().Get("rss").Return(d.connector, true)
	d.connector.EXPECT().Fetch(gomock.Any(), src.Config, "token-123").Return(nil, nil)
	d.items.EXPECT().Upsert(gomock.Any(), src.ID, gomock.Any()).Return(0, nil)
	d.sources.EXPECT().MarkSuccess(gomock.Any(), src.ID, gomock.Any(), gomock.Any()).Return(nil)

	_, err = svc.CollectSource(context.Background(), collectJob(src.ID))
	require.NoError(t, err)
}

func TestCollectSourceUnreadableSecretIsRecordedWithoutDetails(t *testing.T) {
	t.Parallel()
	svc, d := newSourceService(t)
	src := testSource(0)
	src.SecretEnc = []byte("sealed with another key, tampered or truncated")
	d.sources.EXPECT().GetByID(gomock.Any(), src.ID).Return(src, nil)
	d.registry.EXPECT().Get("rss").Return(d.connector, true)
	d.sources.EXPECT().MarkFailure(gomock.Any(), src.ID, fixedNow, "the source credential cannot be decrypted; set it again", gomock.Any()).Return(nil)

	res, err := svc.CollectSource(context.Background(), collectJob(src.ID))
	require.NoError(t, err, "the source keeps its schedule; the admin fixes the credential")
	assert.Equal(t, 0, res.(map[string]any)["fetched"])
}
