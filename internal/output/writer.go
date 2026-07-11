// Package output handles writing results to the filesystem.
// It uses an atomic temp-file rename strategy so the judge's automation layer
// never reads a partially-written file even if the process is interrupted.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"devfleet-agent/internal/models"
)

// WriteResults serializes results to JSON and writes them atomically to path.
//
// Strategy:
//  1. Marshal to JSON in memory
//  2. Write to a sibling temp file (path + ".tmp")
//  3. os.Rename the temp file to the final path (atomic on Linux/macOS)
//  4. If rename fails (cross-device link), fall back to direct os.WriteFile
//
// The output directory is created if it does not exist.
// An empty results slice writes "[]" — never "null".
func WriteResults(path string, results []models.Result) error {
	// Guarantee parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output directory %q: %w", dir, err)
	}

	// Ensure empty slice marshals as "[]" not "null"
	if results == nil {
		results = []models.Result{}
	}

	data, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("marshal results to JSON: %w", err)
	}

	// Write to temp file first — protects against partial writes
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp file %q: %w", tmpPath, err)
	}

	// Atomic rename (same filesystem)
	if err := os.Rename(tmpPath, path); err != nil {
		// Cross-device or permission edge-case: fall back to direct write
		_ = os.Remove(tmpPath) // best-effort cleanup of temp file
		if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
			return fmt.Errorf("fallback write to %q failed: %w (rename error: %v)", path, writeErr, err)
		}
	}

	return nil
}
