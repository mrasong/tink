package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Manage registered macOS client devices",
	Long:  `List or remove registered client devices stored in the tink.db database (supports online & offline mode).`,
}

var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered client devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}
		defer client.Close()

		devices, err := client.ListDevices()
		if err != nil {
			return fmt.Errorf("failed to list devices: %w", err)
		}

		if len(devices) == 0 {
			fmt.Println("No registered devices found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "DEVICE ID\tNAME\tCREATED AT\tLAST CONNECTED\tLAST DISCONNECTED")
		for _, d := range devices {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				d.ID,
				d.Name,
				time.UnixMilli(d.CreatedAt).Format("2006-01-02 15:04:05"),
				time.UnixMilli(d.LastConnectedAt).Format("2006-01-02 15:04:05"),
				time.UnixMilli(d.LastDisconnectedAt).Format("2006-01-02 15:04:05"),
			)
		}
		return w.Flush()
	},
}

var deviceDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a registered client device",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		deviceID := args[0]
		client, err := getClient()
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.DeleteDevice(deviceID); err != nil {
			return fmt.Errorf("failed to delete device: %w", err)
		}
		fmt.Printf("✓ Device %s has been deleted.\n", deviceID)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceDeleteCmd)
}
