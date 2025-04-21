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

	ctx, end := effects.WithStreamEffectHandler[int](ctx, 10)
	defer end()

	source := make(chan int)
	mapSink := make(chan string)
	filterSink := make(chan string)

	// Step 1: Map (int -> string)
	effects.StreamEffect[int](ctx, effects.MapStreamPayload[int, string]{
		Source: source,
		Sink:   mapSink,
		MapFn: func(v int) string {
			return "v=" + string(rune('0'+v))
		},
	})

	// Step 2: Filter (only even values)
	effects.StreamEffect[string](ctx, effects.FilterStreamPayload[string]{
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

	ctx, end := effects.WithStreamEffectHandler[int](ctx, 10)
	defer end()

	source := make(chan int)
	mapSink := make(chan string)
	filterSink := make(chan string)

	done := make(chan string, 3) // map/filter/consumer

	// MapEffect
	effects.StreamEffect[int](ctx, effects.MapStreamPayload[int, string]{
		Source: source,
		Sink:   mapSink,
		MapFn: func(v int) string {
			return fmt.Sprintf("v=%d", v)
		},
	})

	// FilterEffect
	effects.StreamEffect[string](ctx, effects.FilterStreamPayload[string]{
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

func TestSubscribeStreamPayload_OneSinkReceivesEvent(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	// Step 1: Setup stream system
	ctx, endOfStreamHandler := effects.WithStreamEffectHandler[int](ctx, 32)
	defer endOfStreamHandler()

	// Step 2: Setup source and sink
	source := make(chan int)
	sink := make(chan int)

	// Step 3: Subscribe
	effects.StreamEffect[int](ctx, effects.SubscribeStreamPayload[int]{
		Source: source,
		Sink:   sink,
	})

	// Step 4: Send data through source
	ready := make(chan struct{})
	go func() {
		close(ready)
		source <- 42
		close(source)
	}()
	<-ready // Wait for the source to be ready

	// Step 5: Assert sink receives data
	select {
	case v, ok := <-sink:
		if !ok {
			t.Fatal("sink channel was closed unexpectedly")
		}
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout: did not receive value on sink")
	}

	// Step 6: Ensure sink eventually closes after source close
	select {
	case _, ok := <-sink:
		if ok {
			t.Fatal("expected sink to be closed after source closed")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout: sink not closed after source close")
	}
}

func TestSubscribeStreamPayload_MultipleSinksSequentiallyReceiveEvent(t *testing.T) {
	ctx := context.Background()
	ctx, logEnd := WithTestLogEffectHandler(ctx)
	defer logEnd()

	// 1. Prepare stream system
	ctx, teardown := effects.WithStreamEffectHandler[int](ctx, 32)
	defer teardown()

	// 2. Prepare source & sink
	source := make(chan int)
	sink1 := make(chan int, 1)
	sink2 := make(chan int, 1)

	// 3. Subscribe sink1
	effects.StreamEffect[int](ctx, effects.SubscribeStreamPayload[int]{
		Source: source,
		Sink:   sink1,
	})

	time.Sleep(10 * time.Millisecond) // arbit startup 보장용

	// 4. Subscribe sink2
	effects.StreamEffect[int](ctx, effects.SubscribeStreamPayload[int]{
		Source: source,
		Sink:   sink2,
	})

	time.Sleep(50 * time.Millisecond) // time for registration

	// 5. Send data through source
	go func() {
		source <- 99
		close(source)
	}()

	// 6. should receive the same value in both sinks
	expectValue := func(ch <-chan int, who string) {
		select {
		case v, ok := <-ch:
			if !ok {
				t.Fatalf("%s: sink closed unexpectedly", who)
			}
			if v != 99 {
				t.Fatalf("%s: expected 99, got %d", who, v)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("%s: timeout waiting for value", who)
		}
	}

	expectValue(sink1, "sink1")
	expectValue(sink2, "sink2")

	// 7. both sinks should be closed after source close
	expectClosed := func(ch <-chan int, who string) {
		select {
		case _, ok := <-ch:
			if ok {
				t.Fatalf("%s: expected sink to be closed", who)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("%s: sink not closed", who)
		}
	}

	expectClosed(sink1, "sink1")
	expectClosed(sink2, "sink2")
}
