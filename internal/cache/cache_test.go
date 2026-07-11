package cache

import (
	"fmt"
	"sync"
	"testing"
)

func TestCache_GetSet(t *testing.T) {
	c := New()

	// Simple get/set
	c.Set("Hello World", "result-1")
	val, ok := c.Get("Hello World")
	if !ok || val != "result-1" {
		t.Errorf("Get returned unexpected: val=%q, ok=%v", val, ok)
	}

	// Cache miss
	_, ok = c.Get("Unseen prompt")
	if ok {
		t.Error("expected ok=false for cache miss")
	}
}

func TestCache_Normalization(t *testing.T) {
	c := New()

	c.Set("  Calculate   15 + 27  \n", "42")

	// Test case-insensitive and whitespace-insensitive matching
	variants := []string{
		"calculate 15 + 27",
		"  calculate   15 + 27  ",
		"CALCULATE 15 + 27\n",
		"calculate\n15 + 27", // newline normalized to space
	}

	for _, variant := range variants {
		t.Run(variant, func(t *testing.T) {
			val, ok := c.Get(variant)
			if !ok || val != "42" {
				t.Errorf("failed to get cached result for variant %q: val=%q, ok=%v", variant, val, ok)
			}
		})
	}
}

func TestCache_Concurrency(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	workers := 20
	iterations := 100

	// Concurrent writes
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				prompt := fmt.Sprintf("prompt-%d", j)
				c.Set(prompt, fmt.Sprintf("answer-%d", j))
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				prompt := fmt.Sprintf("prompt-%d", j)
				_, _ = c.Get(prompt)
			}
		}(i)
	}

	wg.Wait()

	// Final verification
	for j := 0; j < iterations; j++ {
		prompt := fmt.Sprintf("prompt-%d", j)
		val, ok := c.Get(prompt)
		if !ok || val != fmt.Sprintf("answer-%d", j) {
			t.Errorf("concurrent cache corrupted: prompt=%q, got=%q, ok=%v", prompt, val, ok)
		}
	}
}
