package service

import (
	"sync"
	"testing"
	"time"
)

// TestExecutorAcquireExclusive 验证同一任务执行槽位的互斥性。
func TestExecutorAcquireExclusive(t *testing.T) {
	e := &Executor{running: make(map[uint]*runHandle)}
	if !e.Acquire(1) {
		t.Fatal("first acquire should succeed")
	}
	if e.Acquire(1) {
		t.Fatal("second acquire must fail while held")
	}
	e.clearRunning(1)
	if !e.Acquire(1) {
		t.Fatal("acquire should succeed after release")
	}
	e.clearRunning(1)
}

// TestExecutorAcquireConcurrent 并发抢占,恰好一个成功(回归并发双执行竞态)。
func TestExecutorAcquireConcurrent(t *testing.T) {
	e := &Executor{running: make(map[uint]*runHandle)}
	const n = 50
	var wg sync.WaitGroup
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- e.Acquire(42)
		}()
	}
	wg.Wait()
	close(results)

	success := 0
	for ok := range results {
		if ok {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("exactly one concurrent acquire should succeed, got %d", success)
	}
}

// TestLogBrokerAppendUnsubscribeNoPanic 回归:Append 与 Unsubscribe 并发曾导致
// "send on closed channel" panic(订阅者断开时 close(ch) 与发送竞争)。
func TestLogBrokerAppendUnsubscribeNoPanic(t *testing.T) {
	bc := &broadcaster{subs: make(map[chan string]struct{}), maxLines: 100}
	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				bc.Append("line")
			}
		}
	}()

	for i := 0; i < 500; i++ {
		_, ch := bc.Subscribe()
		bc.Unsubscribe(ch)
		time.Sleep(time.Millisecond)
	}

	close(stop)
	wg.Wait()
}

// TestLogBrokerAppendCloseAllNoPanic 回归:Append 与 closeAll 并发不 panic。
func TestLogBrokerAppendCloseAllNoPanic(t *testing.T) {
	bc := &broadcaster{subs: make(map[chan string]struct{}), maxLines: 100}
	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				bc.Append("x")
			}
		}
	}()

	for i := 0; i < 50; i++ {
		_, ch := bc.Subscribe()
		_ = ch
		time.Sleep(time.Millisecond)
	}
	bc.closeAll()
	close(stop)
	wg.Wait()
}

// TestLogBrokerHistoryTrim 验证环形缓冲只保留最近 maxLines 行。
func TestLogBrokerHistoryTrim(t *testing.T) {
	bc := &broadcaster{subs: make(map[chan string]struct{}), maxLines: 5}
	for i := 0; i < 10; i++ {
		bc.Append("line")
	}
	history, ch := bc.Subscribe()
	bc.Unsubscribe(ch)
	if len(history) != 5 {
		t.Fatalf("history len = %d, want 5", len(history))
	}
}
