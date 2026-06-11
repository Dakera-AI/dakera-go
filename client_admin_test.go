package dakera

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Health Probes
// ===========================================================================

func TestHealthReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/health/ready", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ready":   true,
			"version": "0.11.89",
			"checks": map[string]interface{}{
				"storage": map[string]interface{}{"status": "ok"},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.HealthReady(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Ready)
	assert.Equal(t, "0.11.89", result.Version)
}

func TestHealthLive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/health/live", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"alive":          true,
			"version":        "0.11.89",
			"uptime_seconds": 86400,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.HealthLive(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Alive)
	assert.Equal(t, int64(86400), result.UptimeSeconds)
}

func TestGetIndexStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"namespace":    "test-ns",
			"vectorCount":  5000,
			"indexedCount": 4900,
			"dimensions":   384,
			"indexType":    "hnsw",
			"sizeBytes":    1048576,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetIndexStats(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, int64(5000), result.VectorCount)
	assert.Equal(t, 384, result.Dimensions)
}

// ===========================================================================
// Ops Operations
// ===========================================================================

func TestCompact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/compact", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.Compact(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Status)
}

func TestFlush(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/flush", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.Flush(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Status)
}

func TestOpsStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/ops/stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version":         "0.11.89",
			"total_vectors":   100000,
			"namespace_count": 5,
			"uptime_seconds":  3600,
			"timestamp":       1715900000,
			"state":           "running",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.OpsStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "0.11.89", result.Version)
	assert.Equal(t, int64(100000), result.TotalVectors)
	assert.Equal(t, "running", result.State)
}

func TestClusterStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/cluster/status", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "healthy",
			"nodes":         3,
			"healthy":       true,
			"version":       "0.11.89",
			"redis_healthy": true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ClusterStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "healthy", result.Status)
	assert.Equal(t, 3, result.Nodes)
	assert.True(t, result.Healthy)
}

func TestClusterNodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/cluster/nodes", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "node-1", "address": "10.0.0.1:3000", "status": "healthy", "role": "primary"},
			{"id": "node-2", "address": "10.0.0.2:3000", "status": "healthy", "role": "replica"},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ClusterNodes(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "node-1", result[0].ID)
	assert.Equal(t, "primary", result[0].Role)
}

func TestOptimizeNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/admin/namespaces/test-ns/optimize", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "optimized"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.OptimizeNamespace(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "optimized", result.Status)
}

func TestAdminIndexStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/namespaces/test-ns/index/stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"index_type":  "hnsw",
			"ef_search":   100,
			"connections": 16,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminIndexStats(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "hnsw", result["index_type"])
}

func TestRebuildIndexes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/admin/namespaces/test-ns/index/rebuild", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "rebuild_started"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.RebuildIndexes(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "rebuild_started", result.Status)
}

func TestCacheStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/cache/stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_entries": 1500,
			"hit_rate":      0.85,
			"memory_bytes":  2097152,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.CacheStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1500, result.TotalEntries)
	assert.InDelta(t, 0.85, result.HitRate, 0.001)
}

func TestCacheClear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/admin/cache/clear", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "cleared"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.CacheClear(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "cleared", result.Status)
}

func TestGetConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/config", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"max_connections": 100,
			"write_buffer":   4096,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetConfig(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result["max_connections"])
}

func TestUpdateConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/v1/admin/config", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"max_connections": 200,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.UpdateConfig(context.Background(), map[string]interface{}{"max_connections": 200})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGetQuotas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/quotas", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"max_vectors":  1000000,
			"max_storage":  10737418240,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.GetQuotas(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestUpdateQuotas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/v1/admin/quotas", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"max_vectors": 2000000})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.UpdateQuotas(context.Background(), map[string]interface{}{"max_vectors": 2000000})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlowQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/admin/slow-queries")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"query": "SELECT *", "duration_ms": 1500.0, "timestamp": "2026-05-17T00:00:00Z", "namespace": "test-ns"},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.SlowQueries(context.Background(), &SlowQueryOptions{Limit: 10, MinDurationMs: 100})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.InDelta(t, 1500.0, result[0].DurationMs, 0.1)
}

func TestCreateBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/admin/backups", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":           "backup-001",
			"created_at":   "2026-05-17T00:00:00Z",
			"size_bytes":   5242880,
			"status":       "completed",
			"include_data": true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.CreateBackup(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, "backup-001", result.ID)
	assert.Equal(t, "completed", result.Status)
}

func TestListBackups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/admin/backups", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "backup-001", "created_at": "2026-05-17T00:00:00Z", "size_bytes": 5242880, "status": "completed", "include_data": true},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ListBackups(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestRestoreBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/admin/backups/backup-001/restore", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "restoring"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.RestoreBackup(context.Background(), "backup-001")
	require.NoError(t, err)
	assert.Equal(t, "restoring", result.Status)
}

func TestDeleteBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/v1/admin/backups/backup-001", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "deleted"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	err := client.DeleteBackup(context.Background(), "backup-001")
	require.NoError(t, err)
}

func TestConfigureTTL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/admin/namespaces/test-ns/ttl", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"namespace":   "test-ns",
			"ttl_seconds": 86400,
			"strategy":    "sliding",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ConfigureTTL(context.Background(), "test-ns", 86400, "sliding")
	require.NoError(t, err)
	assert.Equal(t, "test-ns", result.Namespace)
	assert.Equal(t, 86400, result.TtlSeconds)
}

// ===========================================================================
// Ops Diagnostics & Jobs
// ===========================================================================

func TestOpsDiagnostics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/ops/diagnostics", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cpu_usage":    0.45,
			"memory_usage": 0.72,
			"disk_usage":   0.35,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.OpsDiagnostics(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result["cpu_usage"])
}

func TestOpsListJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/ops/jobs", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "job-1", "job_type": "compaction", "status": "running", "created_at": 1715900000, "progress": 50, "metadata": map[string]string{}},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.OpsListJobs(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "job-1", result[0].ID)
}

func TestOpsGetJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/ops/jobs/job-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "job-1", "job_type": "compaction", "status": "completed", "created_at": 1715900000, "progress": 100, "metadata": map[string]string{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.OpsGetJob(context.Background(), "job-1")
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
}

func TestOpsCompact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/ops/compact", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":  "compaction-123",
			"message": "compaction started",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.OpsCompact(context.Background(), CompactionRequest{Namespace: "test-ns", Force: true})
	require.NoError(t, err)
	assert.Equal(t, "compaction-123", result.JobID)
}

func TestOpsShutdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/ops/shutdown", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "shutting_down"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.OpsShutdown(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "shutting_down", result["status"])
}

// ===========================================================================
// Admin Cluster & Maintenance
// ===========================================================================

func TestAdminFulltextReindex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/fulltext/reindex", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"namespaces_processed": 3,
			"total_indexed":        150,
			"total_skipped":        50,
			"details":              []map[string]interface{}{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminFulltextReindex(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, 3, result.NamespacesProcessed)
	assert.Equal(t, 150, result.TotalIndexed)
}

func TestAdminClusterReplication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/cluster/replication", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replication_factor": 3,
			"healthy_replicas":  3,
			"total_nodes":       3,
			"replication_lag":   []map[string]interface{}{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminClusterReplication(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint32(3), result.ReplicationFactor)
}

func TestAdminListShards(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/cluster/shards", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"shards": []map[string]interface{}{
				{"shard_id": "shard-1", "namespace": "ns-1", "primary_node": "node-1", "replica_nodes": []string{"node-2"}, "state": "active", "vector_count": 5000, "size_bytes": 1048576},
			},
			"total": 1,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminListShards(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint32(1), result.Total)
	assert.Len(t, result.Shards, 1)
}

func TestAdminRebalanceShards(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/cluster/shards/rebalance", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"initiated":      true,
			"operation_id":   "rebal-001",
			"shards_affected": 2,
			"planned_moves":  []map[string]interface{}{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminRebalanceShards(context.Background(), ShardRebalanceRequest{DryRun: true})
	require.NoError(t, err)
	assert.True(t, result.Initiated)
	assert.Equal(t, "rebal-001", result.OperationID)
}

func TestAdminMaintenanceStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/cluster/maintenance", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":               false,
			"nodes_in_maintenance":  []string{},
			"rejecting_requests":    false,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminMaintenanceStatus(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Enabled)
}

func TestAdminEnableMaintenance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/cluster/maintenance/enable", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":              true,
			"reason":               "scheduled upgrade",
			"nodes_in_maintenance": []string{"node-1"},
			"rejecting_requests":   true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminEnableMaintenance(context.Background(), EnableMaintenanceRequest{
		Reason:         "scheduled upgrade",
		RejectRequests: true,
	})
	require.NoError(t, err)
	assert.True(t, result.Enabled)
}

func TestAdminDisableMaintenance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/cluster/maintenance/disable", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":              false,
			"nodes_in_maintenance": []string{},
			"rejecting_requests":   false,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminDisableMaintenance(context.Background(), DisableMaintenanceRequest{})
	require.NoError(t, err)
	assert.False(t, result.Enabled)
}

// ===========================================================================
// Admin Quotas
// ===========================================================================

func TestAdminListQuotas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/quotas", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"quotas": []map[string]interface{}{},
			"total":  0,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminListQuotas(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(0), result.Total)
}

func TestAdminGetDefaultQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/quotas/default", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"config": map[string]interface{}{"max_vectors": 1000000},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminGetDefaultQuota(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result.Config)
}

func TestAdminSetDefaultQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/admin/quotas/default", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"namespace": "",
			"config":    map[string]interface{}{"max_vectors": 2000000},
			"message":   "default quota updated",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	maxVec := uint64(2000000)
	result, err := client.AdminSetDefaultQuota(context.Background(), SetDefaultQuotaRequest{
		Config: &QuotaConfig{MaxVectors: &maxVec},
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestAdminGetQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/quotas/test-ns", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"namespace":   "test-ns",
			"config":      map[string]interface{}{},
			"usage":       map[string]interface{}{"vector_count": 500, "storage_bytes": 1024, "last_updated": 1715900000},
			"is_exceeded": false,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminGetQuota(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "test-ns", result.Namespace)
	assert.False(t, result.IsExceeded)
}

func TestAdminSetQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/admin/quotas/test-ns", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"namespace": "test-ns",
			"config":    map[string]interface{}{},
			"message":   "quota set",
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminSetQuota(context.Background(), "test-ns", SetQuotaRequest{Config: QuotaConfig{Enforcement: "hard"}})
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestAdminDeleteQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/admin/quotas/test-ns", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminDeleteQuota(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
}

func TestAdminCheckQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/quotas/test-ns/check", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allowed": true,
			"usage":   map[string]interface{}{"vector_count": 100, "storage_bytes": 512, "last_updated": 1715900000},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminCheckQuota(context.Background(), "test-ns", QuotaCheckRequest{VectorIDs: []string{"v1", "v2"}})
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

// ===========================================================================
// Admin Slow Queries
// ===========================================================================

func TestAdminListSlowQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/admin/slow-queries")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"query": "vector search", "duration_ms": 2500.0, "namespace": "ns-1"},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminListSlowQueries(context.Background(), "ns-1", "", 10)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestAdminSlowQuerySummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/slow-queries/summary", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_slow_queries": 15,
			"avg_duration_ms":    3200.0,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminSlowQuerySummary(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result["total_slow_queries"])
}

func TestAdminClearSlowQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/admin/slow-queries")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"cleared": 15})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminClearSlowQueries(context.Background(), "ns-1")
	require.NoError(t, err)
	assert.NotNil(t, result["cleared"])
}

func TestAdminUpdateSlowQueryConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/admin/slow-queries/config", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"threshold_ms": 500})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminUpdateSlowQueryConfig(context.Background(), map[string]interface{}{"threshold_ms": 500})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ===========================================================================
// Admin Backups (Phase 2)
// ===========================================================================

func TestAdminListBackups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/backups", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"backups": []map[string]interface{}{
				{"backup_id": "bk-1", "name": "daily", "backup_type": "full", "status": "completed", "namespaces": []string{"ns-1"}, "vector_count": 1000, "size_bytes": 4096, "created_at": 1715900000, "encrypted": false},
			},
			"total": 1,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminListBackups(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Total)
}

func TestAdminCreateBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/backups", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"backup": map[string]interface{}{
				"backup_id": "bk-2", "name": "manual", "backup_type": "full", "status": "in_progress",
				"namespaces": []string{}, "vector_count": 0, "size_bytes": 0, "created_at": 1715900000, "encrypted": true,
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminCreateBackup(context.Background(), CreateBackupRequest{Name: "manual", BackupType: "full"})
	require.NoError(t, err)
	assert.Equal(t, "bk-2", result.Backup.BackupID)
}

func TestAdminGetBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/backups/bk-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"backup_id": "bk-1", "name": "daily", "backup_type": "full", "status": "completed",
			"namespaces": []string{"ns-1"}, "vector_count": 1000, "size_bytes": 4096, "created_at": 1715900000, "encrypted": false,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminGetBackup(context.Background(), "bk-1")
	require.NoError(t, err)
	assert.Equal(t, "bk-1", result.BackupID)
}

func TestAdminDeleteBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/admin/backups/bk-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminDeleteBackup(context.Background(), "bk-1")
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
}

func TestAdminGetBackupSchedule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/backups/schedule", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":        true,
			"cron":           "0 2 * * *",
			"backup_type":    "incremental",
			"retention_days": 30,
			"max_backups":    10,
			"namespaces":     []string{},
			"encrypt":        true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminGetBackupSchedule(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "0 2 * * *", result.Cron)
}

func TestAdminUpdateBackupSchedule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/backups/schedule", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":        true,
			"cron":           "0 3 * * *",
			"backup_type":    "full",
			"retention_days": 14,
			"max_backups":    5,
			"namespaces":     []string{},
			"encrypt":        false,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	enabled := true
	result, err := client.AdminUpdateBackupSchedule(context.Background(), UpdateBackupScheduleRequest{
		Enabled: &enabled,
		Cron:    "0 3 * * *",
	})
	require.NoError(t, err)
	assert.Equal(t, "0 3 * * *", result.Cron)
}

func TestAdminRestoreBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/backups/restore", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"restore_id": "restore-001",
			"status":     "in_progress",
			"backup_id":  "bk-1",
			"namespaces": []string{"ns-1"},
			"started_at": 1715900000,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminRestoreBackup(context.Background(), RestoreBackupRequest{BackupID: "bk-1"})
	require.NoError(t, err)
	assert.Equal(t, "restore-001", result.RestoreID)
	assert.Equal(t, "in_progress", result.Status)
}

func TestAdminGetRestoreStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/backups/restore/restore-001", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"restore_id": "restore-001",
			"status":     "completed",
			"backup_id":  "bk-1",
			"namespaces": []string{"ns-1"},
			"started_at": 1715900000,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminGetRestoreStatus(context.Background(), "restore-001")
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
}

// ===========================================================================
// Phase 3: Fulltext, TTL, Route, Import, Storage, Background, Memory Types
// ===========================================================================

func TestFulltextStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/fulltext/stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"document_count": 500,
			"unique_terms":   12000,
			"avg_doc_length": 45.2,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.FulltextStats(context.Background(), "test-ns")
	require.NoError(t, err)
	assert.Equal(t, uint32(500), result.DocumentCount)
}

func TestFulltextDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/namespaces/test-ns/fulltext/delete", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"deleted_count": 3})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.FulltextDelete(context.Background(), "test-ns", []string{"doc-1", "doc-2", "doc-3"})
	require.NoError(t, err)
	assert.Equal(t, 3, result.DeletedCount)
}

func TestAdminTtlStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/ttl/stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"namespaces":    []map[string]interface{}{},
			"total_with_ttl": 100,
			"total_expired": 5,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminTtlStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(100), result.TotalWithTtl)
}

func TestRouteQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/route", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"routes": []map[string]interface{}{
				{"namespace": "ns-1", "similarity": 0.92, "description": "user memories"},
			},
			"model":             "minilm",
			"embedding_time_ms": 5,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.RouteQuery(context.Background(), RouteRequest{Query: "hello world", TopK: 3})
	require.NoError(t, err)
	assert.Len(t, result.Routes, 1)
	assert.Equal(t, "ns-1", result.Routes[0].Namespace)
}

func TestImportJobStatusMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/import/job-123/status", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":     "job-123",
			"status":     "completed",
			"format":     "jsonl",
			"total":      100,
			"imported":   98,
			"skipped":    2,
			"errors":     []string{},
			"started_at": 1715900000,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.ImportJobStatus(context.Background(), "job-123")
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 98, result.Imported)
}

func TestAdminDownloadBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/backups/bk-1/download", r.URL.Path)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("fake-backup-data"))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminDownloadBackup(context.Background(), "bk-1")
	require.NoError(t, err)
	assert.Equal(t, []byte("fake-backup-data"), result)
}

func TestAdminUploadBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/backups/upload", r.URL.Path)
		assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"backup_id": "bk-uploaded", "status": "uploaded"})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminUploadBackup(context.Background(), []byte("backup-data"))
	require.NoError(t, err)
	assert.Equal(t, "bk-uploaded", result["backup_id"])
}

func TestAdminStorageTierOverview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/storage/tiers", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tiers_enabled": true,
			"architecture":  []map[string]interface{}{},
			"config": map[string]interface{}{
				"hot_tier_capacity":           1000,
				"hot_to_warm_threshold_secs":  3600,
				"warm_to_cold_threshold_secs": 86400,
				"auto_tier_enabled":           true,
				"tier_check_interval_secs":    300,
			},
			"activity": map[string]interface{}{
				"promotions":       10,
				"demotions":        5,
				"cache_hit_rate":   0.9,
				"storage_backend":  "local",
				"promotions_to_hot": 10,
				"demotions_to_warm": 3,
				"demotions_to_cold": 2,
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminStorageTierOverview(context.Background())
	require.NoError(t, err)
	assert.True(t, result.TiersEnabled)
}

func TestAdminBackgroundActivity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/background-activity", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active_jobs": 2,
			"pending":     0,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminBackgroundActivity(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result["active_jobs"])
}

func TestAdminMemoryTypeStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/admin/memory-type-stats", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":            5000,
			"working":          100,
			"episodic":         3000,
			"semantic":         1500,
			"procedural":       400,
			"agent_namespaces": 10,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminMemoryTypeStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(5000), result.Total)
	assert.Equal(t, uint64(3000), result.Episodic)
}

func TestAdminMigrateNamespaceDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/namespaces/migrate-dimensions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"migrated":        2,
			"failed":          0,
			"already_current": 1,
			"results":         []map[string]interface{}{},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminMigrateNamespaceDimensions(context.Background(), MigrateNamespaceDimensionsRequest{
		TargetDimension: 1024,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Migrated)
}

// ===========================================================================
// Error Tests for Critical Admin Operations
// ===========================================================================

func TestAdminRebalanceShards_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "server error"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.AdminRebalanceShards(context.Background(), ShardRebalanceRequest{DryRun: false})
	require.Error(t, err)
}

func TestAdminRestoreBackup_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "restore failed"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.AdminRestoreBackup(context.Background(), RestoreBackupRequest{BackupID: "bad"})
	require.Error(t, err)
}

func TestOpsShutdown_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "forbidden"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.OpsShutdown(context.Background())
	require.Error(t, err)
}

// ===========================================================================
// AdminDrainReembed — POST /admin/reembed/drain (v0.11.82+, DAK-6326)
// ===========================================================================

func TestAdminDrainReembedFullDrain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/reembed/drain", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"processed":  1280,
			"remaining":  0,
			"elapsed_ms": 4210,
			"cycles":     3,
			"timed_out":  false,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	result, err := client.AdminDrainReembed(context.Background(), DrainReembedRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1280, result.Processed)
	assert.Equal(t, 0, result.Remaining)
	assert.Equal(t, 3, result.Cycles)
	assert.False(t, result.TimedOut)
}

func TestAdminDrainReembedForwardsParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/admin/reembed/drain", r.URL.Path)
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.EqualValues(t, 600, body["timeout_secs"])
		assert.EqualValues(t, 5000, body["batch_size"])
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"processed":  500,
			"remaining":  120,
			"elapsed_ms": 600000,
			"cycles":     50,
			"timed_out":  true,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	timeout := 600
	batch := 5000
	minImp := float32(0.5)
	result, err := client.AdminDrainReembed(context.Background(), DrainReembedRequest{
		TimeoutSecs:   &timeout,
		BatchSize:     &batch,
		MinImportance: &minImp,
	})
	require.NoError(t, err)
	assert.True(t, result.TimedOut)
	assert.Equal(t, 120, result.Remaining)
}

func TestAdminDrainReembedRequiresAdminScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "admin scope required"})
	}))
	defer server.Close()
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, MaxRetries: 1})
	_, err := client.AdminDrainReembed(context.Background(), DrainReembedRequest{})
	require.Error(t, err)
}
