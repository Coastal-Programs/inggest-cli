package commands

import (
	"fmt"

	"github.com/jakeschepis/zeus-cli/internal/auth"
	"github.com/jakeschepis/zeus-cli/internal/common/config"
	"github.com/jakeschepis/zeus-cli/internal/xero"
	"github.com/jakeschepis/zeus-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewOrgsCmd returns the orgs command group.
func NewOrgsCmd(format *string, org *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orgs",
		Short: "Manage connected Xero organisations",
	}
	cmd.AddCommand(
		newOrgsListCmd(format),
		newOrgsUseCmd(format),
		newOrgsSyncCmd(format),
	)
	return cmd
}

func newOrgsListCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all connected Xero organisations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			type orgRow struct {
				TenantID   string `json:"tenant_id"`
				TenantName string `json:"tenant_name"`
				Active     bool   `json:"active"`
			}

			rows := make([]orgRow, len(cfg.Tenants))
			for i, t := range cfg.Tenants {
				rows[i] = orgRow{
					TenantID:   t.TenantID,
					TenantName: t.TenantName,
					Active:     t.TenantID == cfg.ActiveTenantID,
				}
			}
			return output.Print(rows, output.Format(*format))
		},
	}
}

func newOrgsUseCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name-or-id>",
		Short: "Set the active organisation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			tenant, err := cfg.ResolveTenant(args[0])
			if err != nil {
				return err
			}
			cfg.ActiveTenantID = tenant.TenantID
			if err := cfg.Save(); err != nil {
				return err
			}
			return output.Print(map[string]string{
				"status":      "active",
				"tenant_id":   tenant.TenantID,
				"tenant_name": tenant.TenantName,
			}, output.Format(*format))
		},
	}
}

func newOrgsSyncCmd(format *string) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Re-fetch the list of connected orgs from Xero",
		Long:  "Run this after granting access to new Xero organisations in the Xero app settings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := auth.EnsureValidToken(cfg); err != nil {
				return err
			}
			tenants, err := auth.FetchTenants(cfg.AccessToken)
			if err != nil {
				return err
			}
			cfg.Tenants = make([]config.Tenant, len(tenants))
			for i, t := range tenants {
				cfg.Tenants[i] = config.Tenant{
					TenantID:   t.TenantID,
					TenantName: t.TenantName,
				}
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			return output.Print(cfg.Tenants, output.Format(*format))
		},
	}
}

// getClientForOrg returns a Xero client targeting the resolved org.
// If orgQuery is empty, uses the active org.
func getClientForOrg(orgQuery string) (*xero.Client, *config.Tenant, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	if err := auth.EnsureValidToken(cfg); err != nil {
		return nil, nil, err
	}
	tenant, err := cfg.ResolveTenant(orgQuery)
	if err != nil {
		return nil, nil, err
	}
	client, err := xero.NewForTenant(cfg, tenant.TenantID)
	if err != nil {
		return nil, nil, err
	}
	return client, tenant, nil
}

// getAllClients returns a client for every connected org.
func getAllClients() ([]*xero.Client, []config.Tenant, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	if err := auth.EnsureValidToken(cfg); err != nil {
		return nil, nil, err
	}
	if len(cfg.Tenants) == 0 {
		return nil, nil, fmt.Errorf("no orgs connected — run: xero auth login")
	}
	clients := make([]*xero.Client, 0, len(cfg.Tenants))
	tenants := make([]config.Tenant, 0, len(cfg.Tenants))
	for _, t := range cfg.Tenants {
		c, err := xero.NewForTenant(cfg, t.TenantID)
		if err != nil {
			return nil, nil, fmt.Errorf("creating client for %s: %w", t.TenantName, err)
		}
		clients = append(clients, c)
		tenants = append(tenants, t)
	}
	return clients, tenants, nil
}
