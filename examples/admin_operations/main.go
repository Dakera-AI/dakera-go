// Example: Dakera Go SDK — Admin Operations (Backups, Quotas, Maintenance, Cluster)
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	dakera "github.com/dakera-ai/dakera-go"
)

func dakeraURL() string {
	if u := os.Getenv("DAKERA_API_URL"); u != "" {
		return u
	}
	return "http://localhost:3300"
}

func dakeraAPIKey() string {
	if k := os.Getenv("DAKERA_API_KEY"); k != "" {
		return k
	}
	return "dk-mykey"
}

func main() {
	client := dakera.NewClientWithOptions(dakera.ClientOptions{
		BaseURL: dakeraURL(),
		APIKey:  dakeraAPIKey(),
	})
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// Cluster Status
	// -------------------------------------------------------------------------
	fmt.Println("--- Cluster Status ---")

	cluster, err := client.ClusterStatus(ctx)
	if err != nil {
		log.Fatalf("ClusterStatus failed: %v", err)
	}
	if cluster == nil {
		log.Fatalf("unexpected: cluster status is nil")
	}
	fmt.Printf("Status: %s, Nodes: %d, Healthy: %v, Version: %s\n",
		cluster.Status, cluster.Nodes, cluster.Healthy, cluster.Version)

	nodes, err := client.ClusterNodes(ctx)
	if err != nil {
		log.Fatalf("ClusterNodes failed: %v", err)
	}
	fmt.Printf("Cluster nodes: %d\n", len(nodes))

	// -------------------------------------------------------------------------
	// Backup: Create
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Create Backup ---")

	backupResp, err := client.AdminCreateBackup(ctx, dakera.CreateBackupRequest{
		Name:        "example-backup",
		BackupType:  "full",
		Compression: "zstd",
	})
	if err != nil {
		log.Fatalf("AdminCreateBackup failed: %v", err)
	}
	if backupResp.Backup.BackupID == "" {
		log.Fatalf("unexpected: backup ID is empty")
	}
	fmt.Printf("Created backup: %s (type: %s, status: %s)\n",
		backupResp.Backup.BackupID, backupResp.Backup.BackupType, backupResp.Backup.Status)

	backupID := backupResp.Backup.BackupID

	// -------------------------------------------------------------------------
	// Backup: List
	// -------------------------------------------------------------------------
	fmt.Println("\n--- List Backups ---")

	backups, err := client.AdminListBackups(ctx)
	if err != nil {
		log.Fatalf("AdminListBackups failed: %v", err)
	}
	if backups == nil {
		log.Fatalf("unexpected: backup list response is nil")
	}
	fmt.Printf("Total backups: %d\n", backups.Total)
	for _, b := range backups.Backups {
		fmt.Printf("  %s — %s (%s, %d bytes)\n", b.BackupID, b.Name, b.Status, b.SizeBytes)
	}

	// -------------------------------------------------------------------------
	// Backup: Restore
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Restore Backup ---")

	overwrite := true
	restoreResp, err := client.AdminRestoreBackup(ctx, dakera.RestoreBackupRequest{
		BackupID:  backupID,
		Overwrite: &overwrite,
	})
	if err != nil {
		log.Fatalf("AdminRestoreBackup failed: %v", err)
	}
	if restoreResp.RestoreID == "" {
		log.Fatalf("unexpected: restore ID is empty")
	}
	fmt.Printf("Restore started: %s (status: %s)\n", restoreResp.RestoreID, restoreResp.Status)

	// -------------------------------------------------------------------------
	// Quota Management
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Quota Management ---")

	quotas, err := client.AdminListQuotas(ctx)
	if err != nil {
		log.Fatalf("AdminListQuotas failed: %v", err)
	}
	if quotas == nil {
		log.Fatalf("unexpected: quota list response is nil")
	}
	fmt.Printf("Configured quotas: %d\n", quotas.Total)

	defaultQuota, err := client.AdminGetDefaultQuota(ctx)
	if err != nil {
		log.Fatalf("AdminGetDefaultQuota failed: %v", err)
	}
	fmt.Printf("Default quota config: %+v\n", defaultQuota.Config)

	// Set a quota for a namespace
	maxVec := uint64(100000)
	setResp, err := client.AdminSetQuota(ctx, "example-ns", dakera.SetQuotaRequest{
		Config: dakera.QuotaConfig{
			MaxVectors:  &maxVec,
			Enforcement: "hard",
		},
	})
	if err != nil {
		log.Fatalf("AdminSetQuota failed: %v", err)
	}
	fmt.Printf("Quota set for %s: max_vectors=%d, enforcement=%s\n",
		setResp.Namespace, *setResp.Config.MaxVectors, setResp.Config.Enforcement)

	// -------------------------------------------------------------------------
	// Maintenance Mode
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Maintenance Mode ---")

	durationMin := uint32(5)
	maintResp, err := client.AdminEnableMaintenance(ctx, dakera.EnableMaintenanceRequest{
		Reason:          "scheduled index rebuild",
		RejectRequests:  false,
		DurationMinutes: &durationMin,
	})
	if err != nil {
		log.Fatalf("AdminEnableMaintenance failed: %v", err)
	}
	fmt.Printf("Maintenance enabled: reason=%q, rejecting=%v\n",
		maintResp.Reason, maintResp.RejectingRequests)

	// Check maintenance status
	maintStatus, err := client.AdminMaintenanceStatus(ctx)
	if err != nil {
		log.Fatalf("AdminMaintenanceStatus failed: %v", err)
	}
	fmt.Printf("Maintenance active: %v\n", maintStatus.Enabled)

	// Disable maintenance
	disableResp, err := client.AdminDisableMaintenance(ctx, dakera.DisableMaintenanceRequest{})
	if err != nil {
		log.Fatalf("AdminDisableMaintenance failed: %v", err)
	}
	fmt.Printf("Maintenance disabled: enabled=%v\n", disableResp.Enabled)

	// -------------------------------------------------------------------------
	// Cleanup: delete the test backup
	// -------------------------------------------------------------------------
	fmt.Println("\n--- Cleanup ---")

	_, err = client.AdminDeleteBackup(ctx, backupID)
	if err != nil {
		log.Fatalf("AdminDeleteBackup failed: %v", err)
	}
	fmt.Printf("Deleted backup: %s\n", backupID)

	// Clean up the quota
	_, err = client.AdminDeleteQuota(ctx, "example-ns")
	if err != nil {
		log.Fatalf("AdminDeleteQuota failed: %v", err)
	}
	fmt.Println("Deleted quota for example-ns")

	fmt.Println("\nDone!")
}
