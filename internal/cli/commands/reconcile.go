package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewReconcileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile",
		Short: "Re-apply per-app resources (vhosts, users, perms) from the registry",
		Long: `Reconcile walks the apps registry and re-applies every provisioned
per-app resource against the live containers. Recovery hatch for the case
where a data container's volume was wiped — the registry remembers the
credentials, reconcile pushes them back in. Idempotent and fail-soft.`,
		Args: cobra.NoArgs,
		RunE: runReconcile,
	}
}

func runReconcile(cmd *cobra.Command, _ []string) error {
	engine, _, err := buildEngine()
	if err != nil {
		return err
	}
	results, err := engine.Reconcile(cmd.Context())
	if err != nil {
		return err
	}

	totalFail := 0
	for _, r := range results {
		totalFail += len(r.Failures)
	}
	if totalFail > 0 {
		return fmt.Errorf("reconcile finished with %d failure(s)", totalFail)
	}
	return nil
}
