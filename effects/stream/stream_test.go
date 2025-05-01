package stream_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/stream"
	"github.com/stretchr/testify/assert"
)

func TestStreamEffect_MapFilterMerge(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, end := stream.WithStreamEffectHandler[int](ctx, 10)
	defer end()

	source := make(chan int)
	mapSink := make(chan string)
	filterSink := make(chan string)

	// Step 1: Map (int -> string)
	stream.StreamEffect[int](ctx, stream.MapStreamPayload[int, string]{
		Source: source,
		Sink:   mapSink,
		MapFn: func(v int) string {
			return "v=" + string(rune('0'+v))
		},
	})

	// Step 2: Filter (only even values)
	stream.StreamEffect[string](ctx, stream.FilterStreamPayload[string]{
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
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, end := stream.WithStreamEffectHandler[int](ctx, 10)
	defer end()

	source := make(chan int)
	mapSink := make(chan string)
	filterSink := make(chan string)

	done := make(chan string, 3) // map/filter/consumer

	// MapEffect
	stream.StreamEffect[int](ctx, stream.MapStreamPayload[int, string]{
		Source: source,
		Sink:   mapSink,
		MapFn: func(v int) string {
			return fmt.Sprintf("v=%d", v)
		},
	})

	// FilterEffect
	stream.StreamEffect[string](ctx, stream.FilterStreamPayload[string]{
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
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, endOfStreamHandler := stream.WithStreamEffectHandler[int](ctx, 32)
	defer endOfStreamHandler()

	source := make(chan int, 1)
	sink := make(chan int, 1)
	dropped := make(chan int, stream.MinCapacityOfDroppedChannel)

	// 1. Subscribe sink
	stream.StreamEffect[int](ctx, stream.SubscribeStreamPayload[int]{
		Source: source,
		Target: stream.NewSinkDropPair(
			sink,
			chan<- int(dropped),
		),
	})

	// 2. Allow registration to stabilize
	time.Sleep(300 * time.Millisecond)

	// 3. Send a value into source
outerloop:
	for {
		source <- 42
		time.Sleep(50 * time.Millisecond)
		select {
		case v := <-sink:
			assert.Equal(t, 42, v)
			break outerloop
		case d := <-dropped:
			log.LogEff(ctx, log.LogWarn, "dropped", map[string]interface{}{
				"dropped": d,
			})
			assert.Equal(t, 42, d)
		}
	}

	// 5. Close source after sending
	close(source)

	// 6: Ensure sink eventually closes after source close
	_, ok := <-sink
	assert.False(t, ok)

}

func TestSubscribeStreamPayload_MultipleSinksSequentiallyReceiveEvent(t *testing.T) {
	ctx := context.Background()
	ctx, logEnd := log.WithTestLogEffectHandler(ctx)
	defer logEnd()

	ctx, teardown := stream.WithStreamEffectHandler[int](ctx, 32)
	defer teardown()

	source := make(chan int, 2)
	sink1 := make(chan int, stream.MinCapacityOfDroppedChannel)
	sink2 := make(chan int, stream.MinCapacityOfDroppedChannel)
	dropped := make(chan int, stream.MinCapacityOfDroppedChannel)

	// 1. Subscribe sinks
	stream.StreamEffect[int](ctx, stream.SubscribeStreamPayload[int]{
		Source: source,
		Target: stream.NewSinkDropPair(
			sink1,
			dropped,
		),
	})

	stream.StreamEffect[int](ctx, stream.SubscribeStreamPayload[int]{
		Source: source,
		Target: stream.NewSinkDropPair(
			sink2,
			dropped,
		),
	})

	// 2. Allow registration to stabilize
	time.Sleep(300 * time.Millisecond)

	received := make(chan int, 2)
	ready := make(chan struct{})

	go func() {
		ready <- struct{}{}
		for v := range sink1 {
			received <- v
		}
	}()
	go func() {
		ready <- struct{}{}
		for v := range sink2 {
			received <- v
		}
	}()
	<-ready
	<-ready

	// 3. Send a value into source
	// 먼저 source로 쏜다
	source <- 99

	// 그 다음 received에서 2개를 받는다
	receivedCount := 0

	for receivedCount < 2 {
		select {
		case v := <-received:
			assert.Equal(t, 99, v)
			receivedCount++
		case d := <-dropped:
			assert.Equal(t, 99, d)
			receivedCount++
		case <-time.After(2 * time.Second): // 타임아웃 늘려줌
			t.Fatal("timeout waiting for sinks or dropped")
		}
	}

	// 5. Close source after sending
	close(source)

	// 6. Ensure sinks are closed afterwards
	_, ok := <-sink1
	assert.False(t, ok)
	_, ok = <-sink2
	assert.False(t, ok)
}

func TestStreamEffect_OrderByStreamPayload_SortsCorrectly(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, teardown := stream.WithStreamEffectHandler[int](ctx, 32)
	defer teardown()

	source := make(chan int)
	sink := make(chan int)

	// Source will push out-of-order values
	go func() {
		defer close(source)
		source <- 42
		source <- 7
		source <- 19
		source <- 3
	}()

	stream.StreamEffect[int](ctx, stream.OrderByStreamPayload[int]{
		WindowSize: 5,
		Source:     source,
		Sink:       sink,
		CmpFn: func(a, b int) int {
			return a - b
		},
	})

	var results []int
	for v := range sink {
		results = append(results, v)
	}

	expected := []int{3, 7, 19, 42}
	if !slices.Equal(results, expected) {
		t.Errorf("Expected sorted output %v, got %v", expected, results)
	}
}

func TestStreamEffect_MergeStreamPayload_DoubleClose(t *testing.T) {
	ctx := context.Background()
	ctx, endOfLogHandler := log.WithTestLogEffectHandler(ctx)
	defer endOfLogHandler()

	ctx, teardown := stream.WithStreamEffectHandler[int](ctx, 32)
	defer teardown()

	// Two source channels
	source1 := make(chan int)
	source2 := make(chan int)
	// One shared sink channel
	sink := make(chan int)

	// Merge both sources into one sink by MergeStreamPayload
	stream.StreamEffect[int](ctx, stream.MergeStreamPayload[int]{
		Sources: []<-chan int{source1, source2},
		Sink:    sink,
	})

	// Produce and close from both sources
	go func() {
		source1 <- 1
		close(source1)
	}()
	go func() {
		source2 <- 2
		close(source2)
	}()

	var results []int
	for v := range sink {
		results = append(results, v)
	}

	assert.ElementsMatch(t, []int{1, 2}, results)
}
