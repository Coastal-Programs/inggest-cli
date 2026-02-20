package commands

import (
	"github.com/jakeschepis/zeus-cli/internal/auth"
	"github.com/jakeschepis/zeus-cli/internal/common/config"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

func NewAuthCmd(format *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth",
		Aliases: []string{"a"},
		Short:   "Authenticate with Xero (OAuth 2.0 PKCE)",
	}
	cmd.AddCommand(
		newAuthLoginCmd(format),
		newAuthLogoutCmd(format),
		newAuthStatusCmd(format),
		newAuthRefreshCmd(format),
	)
	return cmd
}

func newAuthLoginCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate via browser (OAuth 2.0 PKCE)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := auth.Login(cfg); err != nil {
				return err
			}
			tenant, _ := cfg.ActiveTenant()
			result := map[string]any{
				"status":  "authenticated",
				"tenants": len(cfg.Tenants),
			}
			if tenant != nil {
				result["active_org"] = tenant.TenantName
				result["active_tenant_id"] = tenant.TenantID
			}
			return output.Print(result, output.Format(*format))
		},
	}
}

func newAuthLogoutCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear stored tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.AccessToken = ""
			cfg.RefreshToken = ""
			cfg.TokenExpiry = 0
			cfg.ActiveTenantID = ""
			cfg.Tenants = nil
			if err := cfg.Save(); err != nil {
				return err
			}
			return output.Print(map[string]string{"status": "logged_out"}, output.Format(*format))
		},
	}
}

func newAuthStatusCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			status := "unauthenticated"
			if cfg.IsAuthenticated() {
				status = "authenticated"
			}
			activeName := ""
			if t, err := cfg.ActiveTenant(); err == nil {
				activeName = t.TenantName
			}
			return output.Print(map[string]any{
				"status":           status,
				"active_org":       activeName,
				"active_tenant_id": cfg.ActiveTenantID,
				"connected_orgs":   len(cfg.Tenants),
				"has_client_id":    cfg.ClientID != "",
			}, output.Format(*format))
		},
	}
}

func newAuthRefreshCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Manually refresh the access token",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := auth.Refresh(cfg); err != nil {
				return err
			}
			return output.Print(map[string]string{"status": "refreshed"}, output.Format(*format))
		},
	}
}

