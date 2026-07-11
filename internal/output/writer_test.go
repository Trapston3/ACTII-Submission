package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"devfleet-agent/internal/models"
)

func TestWriteResults_CreatesDirectoryAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "subdir", "results.json")

	results := []models.Result{
		{TaskID: "1", Answer: "42"},
		{TaskID: "2", Answer: "positive"},
	}

	if err := WriteResults(outPath, results); err != nil {
		t.Fatalf("WriteResults failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}

	var parsed []models.Result
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\ncontent: %s", err, data)
	}

	if len(parsed) != 2 {
		t.Errorf("got %d results, want 2", len(parsed))
	}
	if parsed[0].TaskID != "1" || parsed[0].Answer != "42" {
		t.Errorf("first result mismatch: got %+v", parsed[0])
	}
	if parsed[1].TaskID != "2" || parsed[1].Answer != "positive" {
		t.Errorf("second result mismatch: got %+v", parsed[1])
	}
}

func TestWriteResults_EmptySliceWritesJSONArray(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "results.json")

	if err := WriteResults(outPath, []models.Result{}); err != nil {
		t.Fatalf("WriteResults failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}

	// Must be a JSON array, not null
	if string(data) != "[]" {
		t.Errorf("expected empty JSON array '[]', got %q", string(data))
	}
}

func TestWriteResults_SchemaContract(t *testing.T) {
	// Verify output strictly matches [{"task_id":"...","answer":"..."}]
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "results.json")

	results := []models.Result{
		{TaskID: "task-001", Answer: "The answer is 42"},
	}

	if err := WriteResults(outPath, results); err != nil {
		t.Fatalf("WriteResults failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Unmarshal into generic map to verify field names exactly
	var raw []map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 result, got %d", len(raw))
	}
	entry := raw[0]
	if _, ok := entry["task_id"]; !ok {
		t.Error("output entry missing 'task_id' field")
	}
	if _, ok := entry["answer"]; !ok {
		t.Error("output entry missing 'answer' field")
	}
	// Ensure no extra fields
	if len(entry) != 2 {
		t.Errorf("expected exactly 2 fields, got %d: %v", len(entry), entry)
	}
}

func TestWriteResults_NoTempFileLeftBehind(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "results.json")

	if err := WriteResults(outPath, []models.Result{{TaskID: "1", Answer: "x"}}); err != nil {
		t.Fatalf("WriteResults failed: %v", err)
	}

	// The .tmp file must not exist after successful write
	tmpPath := outPath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("temp file should not exist after successful WriteResults")
	}
}

func TestWriteResults_OverwritesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "results.json")

	// Write initial content
	first := []models.Result{{TaskID: "old", Answer: "old-answer"}}
	if err := WriteResults(outPath, first); err != nil {
		t.Fatalf("first WriteResults failed: %v", err)
	}

	// Overwrite
	second := []models.Result{{TaskID: "new", Answer: "new-answer"}}
	if err := WriteResults(outPath, second); err != nil {
		t.Fatalf("second WriteResults failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	var parsed []models.Result
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if parsed[0].TaskID != "new" {
		t.Errorf("expected overwritten content, got TaskID=%q", parsed[0].TaskID)
	}
}
