package effects_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/stretchr/testify/assert"
)

func TestStreamEffect_MapFilterMerge(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, end := effects.WithStreamEffectHandler(ctx, 10)
	defer end()

	source := make(chan int)
	mapSink := make(chan string)
	filterSink := make(chan string)

	// Step 1: Map (int -> string)
	effects.StreamEffect[int, string](ctx, effects.MapStreamPayload[int, string]{
		Source: source,
		Sink:   mapSink,
		MapFn: func(v int) string {
			return "v=" + string(rune('0'+v))
		},
	})

	// Step 2: Filter (only even values)
	effects.StreamEffect[string, string](ctx, effects.FilterStreamPayload[string]{
		Source:    mapSink,
		Sink:      filterSink,
		Predicate: func(v string) bool { return v == "v=2" || v == "v=4" },
	})

	// Step 3: Consumer
	var results []string
	done := make(chan struct{})
	go func() {
		for v := range filterSink {
			results = append(results, v)
		}
		close(done)
	}()

	// Step 4: Put values into source
	go func() {
		defer close(source)
		for i := 1; i <= 5; i++ {
			source <- i
		}
	}()

	// Wait and verify
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for stream pipeline")
	}

	assert.ElementsMatch(t, []string{"v=2", "v=4"}, results)
}

func TestStreamEffect_ShutdownPropagation(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, end := effects.WithStreamEffectHandler(ctx, 10)
	defer end()

	source := make(chan int)
	mapSink := make(chan string)
	filterSink := make(chan string)

	done := make(chan string, 3) // map/filter/consumer

	// MapEffect
	effects.StreamEffect[int, string](ctx, effects.MapStreamPayload[int, string]{
		Source: source,
		Sink:   mapSink,
		MapFn: func(v int) string {
			return fmt.Sprintf("v=%d", v)
		},
	})

	// FilterEffect
	effects.StreamEffect[string, string](ctx, effects.FilterStreamPayload[string]{
		Source:    mapSink,
		Sink:      filterSink,
		Predicate: func(v string) bool { return strings.HasSuffix(v, "2") || strings.HasSuffix(v, "4") },
	})

	// Sink consumer (done iff filterSink is closed)
	go func() {
		for range filterSink {
			// consume
		}
		done <- "consumer"
	}()

	// watch for mapSink close
	go func() {
		for range mapSink {
			// pass-through
		}
		done <- "filter"
	}()

	// close source to trigger shutdown
	go func() {
		for i := 1; i <= 5; i++ {
			source <- i
		}
		close(source) // 🔥 trigger shutdown
		done <- "map"
	}()

	// Expect all 3 shutdowns
	expected := map[string]bool{
		"map":      false,
		"filter":   false,
		"consumer": false,
	}

	timeout := time.After(time.Second)
	for i := 0; i < 3; i++ {
		select {
		case label := <-done:
			expected[label] = true
		case <-timeout:
			t.Fatal("Timeout waiting for shutdown propagation")
		}
	}

	for label, seen := range expected {
		assert.True(t, seen, fmt.Sprintf("Expected %s to complete", label))
	}
}
