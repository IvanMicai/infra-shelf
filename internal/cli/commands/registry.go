package commands

import (
	"github.com/spf13/cobra"

	"github.com/ivan/infra-shelf/internal/output"
	"github.com/ivan/infra-shelf/internal/registry"
)

func NewRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Registry maintenance",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "encrypt",
		Short: "Re-encrypt the registry in place using INFRA_SHELF_SECRET",
		Args:  cobra.NoArgs,
		RunE:  runRegistryEncrypt,
	})
	return cmd
}

func runRegistryEncrypt(_ *cobra.Command, _ []string) error {
	_, cfg, err := buildEngine()
	if err != nil {
		return err
	}
	store := registry.NewStore(cfg.RegistryPath)
	if err := store.EncryptInPlace(); err != nil {
		return err
	}
	output.Success("Registry encrypted.")
	return nil
}
