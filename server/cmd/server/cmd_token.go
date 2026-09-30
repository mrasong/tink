package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	keyNameFlag string
	keyRoleFlag string
)

var keyCmd = &cobra.Command{
	Use:     "key",
	Aliases: []string{"token"},
	Short:   "Manage authentication Secret Keys and Admin Keys",
	Long:    `Manage API Secret Keys stored in the tink.db database (supports online & offline mode).`,
}

var keyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secret keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}
		defer client.Close()

		keys, err := client.ListKeys()
		if err != nil {
			return fmt.Errorf("failed to list keys: %w", err)
		}

		if len(keys) == 0 {
			fmt.Println("No secret keys found in database.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tROLE\tENABLED\tCREATED AT\tLAST USED")
		for _, k := range keys {
			enabledStr := "Enabled (1)"
			if k.Enabled == 0 {
				enabledStr = "Disabled (0)"
			}
			lastUsedStr := "-"
			if k.LastUsed > 0 {
				lastUsedStr = time.UnixMilli(k.LastUsed).Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				k.ID,
				k.Name,
				k.Role,
				enabledStr,
				time.UnixMilli(k.CreatedAt).Format("2006-01-02 15:04:05"),
				lastUsedStr,
			)
		}
		return w.Flush()
	},
}

var keyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Secret Key (64-char alphanumeric)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}
		defer client.Close()

		name := keyNameFlag
		if name == "" {
			name = "CLI Generated"
		}
		role := keyRoleFlag
		if role != "admin" {
			role = "user"
		}

		rawToken, key, err := client.CreateKey(name, role)
		if err != nil {
			return fmt.Errorf("failed to create secret key: %w", err)
		}

		fmt.Println("✓ Secret Key successfully created!")
		fmt.Printf("ID:         %s\n", key.ID)
		fmt.Printf("Name:       %s\n", key.Name)
		fmt.Printf("Role:       %s\n", key.Role)
		fmt.Printf("Secret Key: %s\n", rawToken)
		fmt.Println("\nNote: Please copy this Secret Key now. It cannot be retrieved again!")
		return nil
	},
}

var keyEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable a disabled Secret Key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyID := args[0]
		client, err := getClient()
		if err != nil {
			return err
		}
		defer client.Close()

		var enabled uint8 = 1
		key, err := client.UpdateKey(keyID, nil, &enabled, nil)
		if err != nil {
			return fmt.Errorf("failed to enable secret key: %w", err)
		}
		fmt.Printf("✓ Secret Key %s (%s) has been enabled.\n", key.ID, key.Name)
		return nil
	},
}

var keyDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable an active Secret Key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyID := args[0]
		client, err := getClient()
		if err != nil {
			return err
		}
		defer client.Close()

		var enabled uint8 = 0
		key, err := client.UpdateKey(keyID, nil, &enabled, nil)
		if err != nil {
			return fmt.Errorf("failed to disable secret key: %w", err)
		}
		fmt.Printf("✓ Secret Key %s (%s) has been disabled.\n", key.ID, key.Name)
		return nil
	},
}

var keyDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a Secret Key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyID := args[0]
		client, err := getClient()
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.DeleteKey(keyID); err != nil {
			return fmt.Errorf("failed to delete secret key: %w", err)
		}
		fmt.Printf("✓ Secret Key %s has been deleted.\n", keyID)
		return nil
	},
}

func init() {
	keyCreateCmd.Flags().StringVarP(&keyNameFlag, "name", "n", "", "Optional name/description for this Secret Key")
	keyCreateCmd.Flags().StringVarP(&keyRoleFlag, "role", "r", "user", "Role for this Secret Key ('admin' or 'user')")

	keyCmd.AddCommand(keyListCmd)
	keyCmd.AddCommand(keyCreateCmd)
	keyCmd.AddCommand(keyEnableCmd)
	keyCmd.AddCommand(keyDisableCmd)
	keyCmd.AddCommand(keyDeleteCmd)
}
