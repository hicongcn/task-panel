package service

import (
	"sync"
)

// LogBroker 是运行中任务日志的内存 pub/sub,供 SSE 实时订阅。
// 每个执行实例(以 taskLogID 标识)对应一个 broadcaster,持有最近日志环形缓冲与订阅者。
type LogBroker struct {
	mu   sync.RWMutex
	subs map[uint]*broadcaster
}

type broadcaster struct {
	mu       sync.RWMutex
	subs     map[chan string]struct{}
	history  []string
	maxLines int
	done     bool
}

var defaultLogBroker = &LogBroker{subs: make(map[uint]*broadcaster)}

// GetLogBroker 返回全局单例。
func GetLogBroker() *LogBroker { return defaultLogBroker }

// GetOrCreate 返回 taskLogID 对应的 broadcaster,不存在则创建。
func (b *LogBroker) GetOrCreate(taskLogID uint) *broadcaster {
	b.mu.Lock()
	defer b.mu.Unlock()
	bc, ok := b.subs[taskLogID]
	if !ok {
		bc = &broadcaster{subs: make(map[chan string]struct{}), maxLines: 2000}
		b.subs[taskLogID] = bc
	}
	return bc
}

// Get 返回已存在的 broadcaster,不存在返回 nil。
func (b *LogBroker) Get(taskLogID uint) *broadcaster {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.subs[taskLogID]
}

// Remove 移除并关闭 broadcaster 的所有订阅。
func (b *LogBroker) Remove(taskLogID uint) {
	b.mu.Lock()
	bc, ok := b.subs[taskLogID]
	delete(b.subs, taskLogID)
	b.mu.Unlock()
	if ok {
		bc.closeAll()
	}
}

// Append 写入一行日志:入历史缓冲 + 广播给订阅者。
// 注意:收集订阅者快照与发送必须在同一把锁内完成 —— Unsubscribe/closeAll 会
// 在锁内 close(ch),若在锁外发送,可能与 close 竞争导致 "send on closed channel" panic。
// 发送是带 default 的非阻塞写,订阅者消费不过来则丢弃,不会阻塞执行器。
func (b *broadcaster) Append(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.history = append(b.history, line)
	if len(b.history) > b.maxLines {
		b.history = b.history[len(b.history)-b.maxLines:]
	}
	for ch := range b.subs {
		select {
		case ch <- line:
		default:
			// 订阅者消费不过来则丢弃,避免阻塞执行器。
		}
	}
}

// Subscribe 订阅实时日志,返回历史快照与实时 channel。
func (b *broadcaster) Subscribe() ([]string, chan string) {
	ch := make(chan string, 256)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	history := append([]string(nil), b.history...)
	b.mu.Unlock()
	return history, ch
}

// Unsubscribe 取消订阅。
func (b *broadcaster) Unsubscribe(ch chan string) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

// Done 标记结束并通知订阅者(发一个哨兵行)。
func (b *broadcaster) Done() {
	b.mu.Lock()
	b.done = true
	b.mu.Unlock()
	b.Append("\x00DONE")
}

// IsDone 返回是否已结束。
func (b *broadcaster) IsDone() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.done
}

func (b *broadcaster) closeAll() {
	b.mu.Lock()
	for ch := range b.subs {
		close(ch)
	}
	b.subs = make(map[chan string]struct{})
	b.mu.Unlock()
}
