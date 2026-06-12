package storage

import (
	"path/filepath"
	"testing"
	"time"

	"pve-traffic-monitor/pkg/models"
)

func TestSQLiteStorageEndToEnd(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data", "pve_traffic.db")

	store, err := NewStorageFromConfig(&models.StorageConfig{
		Type:         "sqlite",
		DSN:          dbPath,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatalf("create sqlite storage: %v", err)
	}
	defer store.Close()

	baseTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	records := []models.TrafficRecord{
		{VMID: 101, Timestamp: baseTime, RXBytes: 1000, TXBytes: 500, TotalBytes: 1500},
		{VMID: 101, Timestamp: baseTime.Add(time.Minute), RXBytes: 1400, TXBytes: 800, TotalBytes: 2200},
		{VMID: 101, NetworkInterface: "net0", Timestamp: baseTime, RXBytes: 100, TXBytes: 50, TotalBytes: 150},
		{VMID: 101, NetworkInterface: "net0", Timestamp: baseTime.Add(time.Minute), RXBytes: 250, TXBytes: 125, TotalBytes: 375},
	}

	for _, record := range records {
		if err := store.SaveTrafficRecord(record); err != nil {
			t.Fatalf("save traffic record: %v", err)
		}
	}

	gotRecords, err := store.GetTrafficRecords(101, baseTime.Add(-time.Second), baseTime.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("get traffic records: %v", err)
	}
	if len(gotRecords) != 2 {
		t.Fatalf("record count = %d, want 2 all-interface records", len(gotRecords))
	}

	stats, err := store.CalculateTrafficStatsWithTimeRange(101, baseTime.Add(-time.Second), baseTime.Add(2*time.Minute), models.DirectionBoth)
	if err != nil {
		t.Fatalf("calculate traffic stats: %v", err)
	}
	if stats.RXBytes != 400 || stats.TXBytes != 300 || stats.TotalBytes != 700 {
		t.Fatalf("stats = rx:%d tx:%d total:%d, want rx:400 tx:300 total:700", stats.RXBytes, stats.TXBytes, stats.TotalBytes)
	}

	net0Stats, err := store.CalculateTrafficStatsWithTimeRangeAndInterface(101, "net0", baseTime.Add(-time.Second), baseTime.Add(2*time.Minute), models.DirectionBoth)
	if err != nil {
		t.Fatalf("calculate net0 traffic stats: %v", err)
	}
	if net0Stats.RXBytes != 150 || net0Stats.TXBytes != 75 || net0Stats.TotalBytes != 225 {
		t.Fatalf("net0 stats = rx:%d tx:%d total:%d, want rx:150 tx:75 total:225", net0Stats.RXBytes, net0Stats.TXBytes, net0Stats.TotalBytes)
	}

	actionLog := models.ActionLog{
		VMID:      101,
		RuleName:  "test-rule",
		Action:    models.ActionDisconnect,
		Reason:    "limit exceeded",
		Timestamp: baseTime,
		Success:   true,
	}
	if err := store.SaveActionLog(actionLog); err != nil {
		t.Fatalf("save action log: %v", err)
	}
	logs, err := store.GetActionLogs(baseTime.Add(-time.Second), baseTime.Add(time.Second))
	if err != nil {
		t.Fatalf("get action logs: %v", err)
	}
	if len(logs) != 1 || logs[0].RuleName != actionLog.RuleName {
		t.Fatalf("logs = %#v, want one test-rule log", logs)
	}

	state := map[string]interface{}{
		"status": "running",
		"seen":   true,
	}
	if err := store.SaveVMState(101, state); err != nil {
		t.Fatalf("save vm state: %v", err)
	}
	gotState, err := store.LoadVMState(101)
	if err != nil {
		t.Fatalf("load vm state: %v", err)
	}
	if gotState["status"] != "running" || gotState["seen"] != true {
		t.Fatalf("state = %#v, want saved state", gotState)
	}

	totalCount, err := store.GetTotalRecordCount()
	if err != nil {
		t.Fatalf("get total record count: %v", err)
	}
	if totalCount != 4 {
		t.Fatalf("total count = %d, want 4", totalCount)
	}

	deleted, err := store.DeleteRecordsInRange(101, baseTime, baseTime)
	if err != nil {
		t.Fatalf("delete records in range: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	rangeCount, err := store.CountRecordsInRange(101, baseTime.Add(-time.Second), baseTime.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("count records in range: %v", err)
	}
	if rangeCount != 2 {
		t.Fatalf("range count = %d, want 2", rangeCount)
	}
}
