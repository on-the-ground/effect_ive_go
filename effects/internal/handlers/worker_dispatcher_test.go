package handlers_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
)

// dummyMessage implements Partitionable for testing partitioned dispatching.
type dummyMessage struct {
	id    int
	group string
}

func (d dummyMessage) PartitionKey() string {
	return d.group
}

// Test that SingleQueue calls the handler with messages sent to it.
func TestSingleQueue_DispatchesToHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu     sync.Mutex
		called []int
		wg     sync.WaitGroup
	)
	wg.Add(2)

	handleFn := func(_ context.Context, msg int) {
		defer wg.Done()
		mu.Lock()
		called = append(called, msg)
		mu.Unlock()
	}

	dispatcher := handlers.NewSingleQueue(ctx, 10, handleFn)
	ch := dispatcher.GetChannelOf(0)

	ch <- 1
	ch <- 2
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(called) != 2 || !slices.Contains(called, 1) || !slices.Contains(called, 2) {
		t.Errorf("Handler was not called correctly: %v", called)
	}
}

// Test that PartitionedQueue dispatches messages to the correct worker
// based on their PartitionKey.
func TestPartitionedQueue_DispatchesToCorrectWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu        sync.Mutex
		workerHit = make(map[string][]int)
		wg        sync.WaitGroup
	)
	wg.Add(4)

	handleFn := func(_ context.Context, msg dummyMessage) {
		defer wg.Done()
		mu.Lock()
		workerHit[msg.group] = append(workerHit[msg.group], msg.id)
		mu.Unlock()
	}

	dispatcher := handlers.NewPartitionedQueue(ctx, 10, 10, handleFn)

	msgs := []dummyMessage{
		{1, "groupA"},
		{2, "groupA"},
		{3, "groupB"},
		{4, "groupB"},
	}

	for _, msg := range msgs {
		ch := dispatcher.GetChannelOf(msg)
		ch <- msg
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(workerHit["groupA"]) != 2 || len(workerHit["groupB"]) != 2 {
		t.Errorf("Expected each group to handle 2 messages: got %v", workerHit)
	}
}

// Test that SingleQueue stops receiving messages after context cancelation
// and panics on send to closed channel.
func TestWorkerDispatcher_GracefulShutdownOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	called := make(chan int, 1)
	dispatcher := handlers.NewSingleQueue(ctx, 1, func(_ context.Context, msg int) {
		called <- msg
	})

	msg := 0
	dispatcher.GetChannelOf(msg) <- msg

	select {
	case <-called:
		// expected
	case <-time.After(1 * time.Second):
		t.Fatal("Handler was not called before context cancel")
	}

	cancel()
	time.Sleep(100 * time.Millisecond) // give goroutine time to exit

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic when sending to closed channel, but no panic occurred")
		} else {
			t.Logf("Recovered from expected panic: %v", r)
		}
	}()

	dispatcher.GetChannelOf(msg) <- msg // should panic
}

// handlerState is used to block and observe handler execution.
type handlerState struct {
	blockUntil chan struct{}
	entered    chan struct{}
}

func (h *handlerState) Handle(ctx context.Context, msg int) {
	h.entered <- struct{}{}
	<-h.blockUntil
}

// Test that SingleQueue blocks when buffer is full.
func TestSingleQueue_BlocksWhenBufferIsFull(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	state := &handlerState{
		blockUntil: make(chan struct{}),
		entered:    make(chan struct{}, 1),
	}

	dispatcher := handlers.NewSingleQueue(ctx, 1, state.Handle)
	targetCh := dispatcher.GetChannelOf(0)

	go func() {
		targetCh <- 1 // handler consumes this and blocks
	}()

	select {
	case <-state.entered:
		// handler entered
	case <-time.After(time.Second):
		t.Fatal("Handler did not start")
	}

	targetCh <- 2 // buffer fills

	blocked := make(chan struct{})
	go func() {
		targetCh <- 3 // should block
		blocked <- struct{}{}
	}()

	select {
	case <-blocked:
		t.Fatal("Expected third message to block, but it didn't")
	case <-time.After(200 * time.Millisecond):
		// expected
	}

	close(state.blockUntil) // unblock handler

	select {
	case <-blocked:
		// good
	case <-time.After(time.Second):
		t.Fatal("Third message never unblocked")
	}
}

// Test that messages with the same partition key
// are processed in order.
func TestPartitionedQueue_OrderIsPreservedForSamePartitionKey(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu        sync.Mutex
		processed []int
		wg        sync.WaitGroup
	)
	wg.Add(5)

	handleFn := func(_ context.Context, msg dummyMessage) {
		defer wg.Done()
		mu.Lock()
		processed = append(processed, msg.id)
		mu.Unlock()
	}

	dispatcher := handlers.NewPartitionedQueue(ctx, 2, 10, handleFn)

	key := "sameKey"
	for i := 0; i < 5; i++ {
		msg := dummyMessage{id: i, group: key}
		dispatcher.GetChannelOf(msg) <- msg
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	for i := 0; i < len(processed); i++ {
		if processed[i] != i {
			t.Fatalf("Expected order %v but got %v", []int{0, 1, 2, 3, 4}, processed)
		}
	}
}
