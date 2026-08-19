package seed

import (
	"context"

	"github.com/portfolio/pf-reliability/apps/api/internal/incident"
)

const (
	CheckoutCode  = "checkout"
	InventoryCode = "inventory"
)

func Ensure(ctx context.Context, svc *incident.Service, integrationKey, webhookSecret string) error {
	checkout, err := svc.CreateMonitored(ctx, CheckoutCode, "Checkout API", "Demo checkout used in SRE drills. Metrics are virtual.")
	if err != nil {
		return err
	}
	inventory, err := svc.CreateMonitored(ctx, InventoryCode, "Inventory", "P06-shaped inventory. Alerts are ingested; no production commands run.")
	if err != nil {
		return err
	}
	_ = checkout
	_, err = svc.EnsureIntegration(ctx, inventory.ID, integrationKey, webhookSecret)
	return err
}
