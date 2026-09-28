package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type updateAccountCredsRegularRepoStub struct {
	AccountRepository
	account     *Account
	updateCalls int
}

func (r *updateAccountCredsRegularRepoStub) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

func (r *updateAccountCredsRegularRepoStub) Update(_ context.Context, account *Account) error {
	r.updateCalls++
	r.account = account
	return nil
}

func TestUpdateAccountPreservesSensitiveCredsWhenIncomingOmits(t *testing.T) {
	accountID := int64(202)
	repo := &updateAccountCredsRegularRepoStub{
		account: &Account{
			ID:       accountID,
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Credentials: map[string]any{
				"refresh_token": "rt-existing",
				"access_token":  "at-existing",
				"id_token":      "id-existing",
				"kiro_api_key":  "ksk-existing",
				"base_url":      "https://old.example.com",
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{
			"base_url": "https://new.example.com",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, "rt-existing", repo.account.Credentials["refresh_token"])
	require.Equal(t, "at-existing", repo.account.Credentials["access_token"])
	require.Equal(t, "id-existing", repo.account.Credentials["id_token"])
	require.Equal(t, "ksk-existing", repo.account.Credentials["kiro_api_key"])
	require.Equal(t, "https://new.example.com", repo.account.Credentials["base_url"])
}

func TestUpdateAccountEmptyKiroAPIKeyDoesNotClearCredential(t *testing.T) {
	accountID := int64(203)
	repo := &updateAccountCredsRegularRepoStub{
		account: &Account{
			ID:       accountID,
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Credentials: map[string]any{
				"refresh_token": "rt-existing",
				"kiro_api_key":  "ksk-existing",
				"kiroApiKey":    "ksk-legacy-existing",
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{"kiro_api_key": nil},
	})

	require.NoError(t, err)
	require.Equal(t, "ksk-existing", repo.account.Credentials["kiro_api_key"])
	require.Equal(t, "ksk-legacy-existing", repo.account.Credentials["kiroApiKey"])
	require.Equal(t, "rt-existing", repo.account.Credentials["refresh_token"])
}

func TestUpdateAccountAllowsRoutineEditWithLegacyMalformedKiroRegion(t *testing.T) {
	accountID := int64(204)
	repo := &updateAccountCredsRegularRepoStub{account: &Account{
		ID: accountID, Platform: PlatformKiro, Type: AccountTypeOAuth, Status: StatusActive,
		Credentials: map[string]any{"refresh_token": "rt-existing", "api_region": "admin@sub2api.local"},
	}}
	svc := &adminServiceImpl{accountRepo: repo}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{"model_mapping": map[string]any{"from": "to"}},
	})
	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	// The malformed legacy override is allowed to disappear when a routine edit
	// submits a redacted full-object credentials map; runtime also ignores it.
	require.NotContains(t, repo.account.Credentials, "api_region")
}

func TestUpdateAccountRejectsInvalidKiroRuntimeFields(t *testing.T) {
	accountID := int64(205)
	repo := &updateAccountCredsRegularRepoStub{
		account: &Account{
			ID:       accountID,
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Credentials: map[string]any{
				"refresh_token": "rt-existing",
				"access_token":  "at-existing",
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{"api_region": "admin@sub2api.local"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "INVALID_KIRO_API_REGION")
	require.Equal(t, 0, repo.updateCalls)

	_, err = svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{"kiro_api_key": "admin@sub2api.local"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "INVALID_KIRO_CLI_KEY")
	require.Equal(t, 0, repo.updateCalls)
}
