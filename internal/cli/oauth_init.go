package cli

import (
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/storage"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/jtprogru/jtpost/internal/core/oauth_providers"
	"github.com/jtprogru/jtpost/internal/logger"
)

// buildOAuthService собирает OAuthService с зарегистрированными провайдерами
// из cfg.Auth.OAuthProviders. Возвращает nil если ни один провайдер не
// настроен (или Bundle.OAuthAccounts == nil).
func buildOAuthService(cfg *config.Config, bundle *storage.Bundle, log *logger.Logger) *core.OAuthService {
	if bundle == nil || bundle.OAuthAccounts == nil || bundle.Users == nil {
		return nil
	}
	providers := map[string]core.OAuthProvider{}
	if gh, ok := cfg.Auth.OAuthProviders["github"]; ok && gh.ClientID != "" && gh.ClientSecret != "" && gh.RedirectURL != "" {
		providers["github"] = oauth_providers.NewGitHubProvider(gh.ClientID, gh.ClientSecret, gh.RedirectURL)
	}
	if len(providers) == 0 {
		return nil
	}
	if log != nil {
		names := make([]string, 0, len(providers))
		for name := range providers {
			names = append(names, name)
		}
		log.Info("🔐 OAuth providers enabled: %v", names)
	}
	return core.NewOAuthService(
		providers,
		bundle.Users,
		bundle.OAuthAccounts,
		cfg.Auth.TenantDefault,
		core.RoleAuthor,
		core.SystemClock{},
	)
}
