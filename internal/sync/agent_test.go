package sync_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pietroperona/night-agent/internal/audit"
	"github.com/pietroperona/night-agent/internal/cloudconfig"
	cloudsync "github.com/pietroperona/night-agent/internal/sync"
)

// writeEvents scrive eventi nel file JSONL e restituisce il path.
func writeEvents(t *testing.T, dir string, events []audit.Event) string {
	t.Helper()
	path := filepath.Join(dir, "audit.jsonl")
	logger, err := audit.NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	for _, e := range events {
		if err := logger.Write(e); err != nil {
			t.Fatalf("Write event: %v", err)
		}
	}
	logger.Close()
	return path
}

func TestSyncOnce_SendsBatch(t *testing.T) {
	var received []cloudsync.IngestRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ingest" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req cloudsync.IngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		received = append(received, req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cloudsync.IngestResponse{
			Received: len(req.Batch),
			Cursor:   req.Batch[len(req.Batch)-1].ID,
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	events := []audit.Event{
		{ID: "e1", Command: "git status", Decision: "allow"},
		{ID: "e2", Command: "sudo su", Decision: "block"},
	}
	logPath := writeEvents(t, dir, events)
	cfgPath := filepath.Join(dir, "cloud.yaml")

	cfg := &cloudconfig.Config{
		Token:     "test-token",
		Endpoint:  srv.URL,
		MachineID: "machine-abc",
		Connected: true,
	}
	cloudconfig.Save(cfgPath, cfg)

	agent := cloudsync.NewAgent(cfgPath, logPath)
	if err := agent.SyncOnce(); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("attese 1 richiesta, ricevute %d", len(received))
	}
	if len(received[0].Batch) != 2 {
		t.Errorf("attesi 2 eventi nel batch, ricevuti %d", len(received[0].Batch))
	}
	if received[0].MachineID != "machine-abc" {
		t.Errorf("machine_id atteso 'machine-abc', ricevuto '%s'", received[0].MachineID)
	}
}

func TestSyncOnce_RespectsCursor(t *testing.T) {
	var batches [][]audit.Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req cloudsync.IngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		batches = append(batches, req.Batch)
		json.NewEncoder(w).Encode(cloudsync.IngestResponse{
			Received: len(req.Batch),
			Cursor:   req.Batch[len(req.Batch)-1].ID,
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	events := []audit.Event{
		{ID: "e1", Command: "git status", Decision: "allow"},
		{ID: "e2", Command: "git diff", Decision: "allow"},
		{ID: "e3", Command: "sudo su", Decision: "block"},
	}
	logPath := writeEvents(t, dir, events)
	cfgPath := filepath.Join(dir, "cloud.yaml")

	// cursore già su e1 → deve inviare solo e2 e e3
	cfg := &cloudconfig.Config{
		Token:     "tok",
		Endpoint:  srv.URL,
		MachineID: "m1",
		Cursor:    "e1",
		Connected: true,
	}
	cloudconfig.Save(cfgPath, cfg)

	agent := cloudsync.NewAgent(cfgPath, logPath)
	if err := agent.SyncOnce(); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	if len(batches) != 1 {
		t.Fatalf("atteso 1 batch, ricevuti %d", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("attesi 2 eventi (post-cursor), ricevuti %d", len(batches[0]))
	}
	if batches[0][0].ID != "e2" {
		t.Errorf("primo evento atteso e2, ricevuto %s", batches[0][0].ID)
	}
}

func TestSyncOnce_NothingToSync(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	os.WriteFile(logPath, []byte{}, 0600)

	cfgPath := filepath.Join(dir, "cloud.yaml")
	cfg := &cloudconfig.Config{
		Token:     "tok",
		Endpoint:  srv.URL,
		MachineID: "m1",
		Connected: true,
	}
	cloudconfig.Save(cfgPath, cfg)

	agent := cloudsync.NewAgent(cfgPath, logPath)
	if err := agent.SyncOnce(); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if called {
		t.Error("non doveva chiamare l'API con log vuoto")
	}
}

func TestSyncOnce_401_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	dir := t.TempDir()
	events := []audit.Event{{ID: "e1", Command: "ls", Decision: "allow"}}
	logPath := writeEvents(t, dir, events)
	cfgPath := filepath.Join(dir, "cloud.yaml")

	cfg := &cloudconfig.Config{
		Token:     "bad-token",
		Endpoint:  srv.URL,
		MachineID: "m1",
		Connected: true,
	}
	cloudconfig.Save(cfgPath, cfg)

	agent := cloudsync.NewAgent(cfgPath, logPath)
	err := agent.SyncOnce()
	if err == nil {
		t.Fatal("atteso errore su 401, ricevuto nil")
	}
}

func TestSyncOnce_BatchLimit(t *testing.T) {
	var batches []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req cloudsync.IngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		batches = append(batches, len(req.Batch))
		last := req.Batch[len(req.Batch)-1]
		json.NewEncoder(w).Encode(cloudsync.IngestResponse{
			Received: len(req.Batch),
			Cursor:   last.ID,
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	// crea 150 eventi → deve fare 2 batch (100 + 50)
	var events []audit.Event
	for i := 0; i < 150; i++ {
		events = append(events, audit.Event{
			ID:        fmt.Sprintf("e%d", i+1),
			Command:   "ls",
			Decision:  "allow",
			Timestamp: time.Now().UTC(),
		})
	}
	logPath := writeEvents(t, dir, events)
	cfgPath := filepath.Join(dir, "cloud.yaml")

	cfg := &cloudconfig.Config{
		Token:     "tok",
		Endpoint:  srv.URL,
		MachineID: "m1",
		Connected: true,
	}
	cloudconfig.Save(cfgPath, cfg)

	agent := cloudsync.NewAgent(cfgPath, logPath)
	if err := agent.SyncOnce(); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	total := 0
	for _, b := range batches {
		if b > 100 {
			t.Errorf("batch size %d supera limite 100", b)
		}
		total += b
	}
	if total != 150 {
		t.Errorf("attesi 150 eventi totali, ricevuti %d", total)
	}
}

func TestSyncOnce_UpdatesCursorAfterSync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req cloudsync.IngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(cloudsync.IngestResponse{
			Received: len(req.Batch),
			Cursor:   req.Batch[len(req.Batch)-1].ID,
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	events := []audit.Event{
		{ID: "x1", Command: "ls", Decision: "allow"},
		{ID: "x2", Command: "cat .env", Decision: "sandbox"},
	}
	logPath := writeEvents(t, dir, events)
	cfgPath := filepath.Join(dir, "cloud.yaml")

	cfg := &cloudconfig.Config{
		Token:     "tok",
		Endpoint:  srv.URL,
		MachineID: "m1",
		Connected: true,
	}
	cloudconfig.Save(cfgPath, cfg)

	agent := cloudsync.NewAgent(cfgPath, logPath)
	if err := agent.SyncOnce(); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	// verifica che il cursore sia stato aggiornato
	updated, err := cloudconfig.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	if updated.Cursor != "x2" {
		t.Errorf("cursore atteso 'x2', ricevuto '%s'", updated.Cursor)
	}
	if updated.LastSync.IsZero() {
		t.Error("LastSync non aggiornato dopo sync riuscito")
	}
}
