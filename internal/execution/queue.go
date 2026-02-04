// Package execution 提供任务执行引擎功能
package execution

import (
	"container/heap"
	"sync"
)

// PriorityQueue 优先级队列
type PriorityQueue struct {
	mu    sync.RWMutex
	items []*queueItem
	index map[string]int // jobID -> index in items
}

type queueItem struct {
	job      *ExecutionJob
	priority int   // 优先级 * 1000000 - 创建时间（确保高优先级优先，同优先级按时间排序）
	index    int
}

// NewPriorityQueue 创建优先级队列
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		items: make([]*queueItem, 0),
		index: make(map[string]int),
	}
	heap.Init(pq)
	return pq
}

// Len 实现 heap.Interface
func (pq *PriorityQueue) Len() int {
	return len(pq.items)
}

// Less 实现 heap.Interface - 优先级高的排前面
func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.items[i].priority > pq.items[j].priority
}

// Swap 实现 heap.Interface
func (pq *PriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
	pq.index[pq.items[i].job.ID] = i
	pq.index[pq.items[j].job.ID] = j
}

// Push 实现 heap.Interface
func (pq *PriorityQueue) Push(x any) {
	n := len(pq.items)
	item := x.(*queueItem)
	item.index = n
	pq.items = append(pq.items, item)
	pq.index[item.job.ID] = n
}

// Pop 实现 heap.Interface
func (pq *PriorityQueue) Pop() any {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	pq.items = old[0 : n-1]
	delete(pq.index, item.job.ID)
	return item
}

// Enqueue 入队
func (pq *PriorityQueue) Enqueue(job *ExecutionJob) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// 计算优先级分数：优先级 * 1000000 - 创建时间
	// 这样高优先级的任务优先，同优先级的按创建时间排序
	priority := int(job.Priority)*1000000 - int(job.CreatedAt%1000000)

	item := &queueItem{
		job:      job,
		priority: priority,
	}
	heap.Push(pq, item)
	job.Status = JobQueued
}

// Dequeue 出队
func (pq *PriorityQueue) Dequeue() *ExecutionJob {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items) == 0 {
		return nil
	}

	item := heap.Pop(pq).(*queueItem)
	return item.job
}

// Peek 查看队首元素但不出队
func (pq *PriorityQueue) Peek() *ExecutionJob {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if len(pq.items) == 0 {
		return nil
	}
	return pq.items[0].job
}

// Remove 从队列中移除指定任务
func (pq *PriorityQueue) Remove(jobID string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	idx, exists := pq.index[jobID]
	if !exists {
		return false
	}

	heap.Remove(pq, idx)
	return true
}

// Size 队列大小
func (pq *PriorityQueue) Size() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.items)
}

// Contains 检查任务是否在队列中
func (pq *PriorityQueue) Contains(jobID string) bool {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	_, exists := pq.index[jobID]
	return exists
}

// UpdatePriority 更新任务优先级
func (pq *PriorityQueue) UpdatePriority(jobID string, newPriority JobPriority) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	idx, exists := pq.index[jobID]
	if !exists {
		return false
	}

	item := pq.items[idx]
	item.job.Priority = newPriority
	item.priority = int(newPriority)*1000000 - int(item.job.CreatedAt%1000000)
	heap.Fix(pq, idx)
	return true
}

// Clear 清空队列
func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.items = make([]*queueItem, 0)
	pq.index = make(map[string]int)
}

// List 列出所有队列中的任务
func (pq *PriorityQueue) List() []*ExecutionJob {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	result := make([]*ExecutionJob, len(pq.items))
	for i, item := range pq.items {
		result[i] = item.job
	}
	return result
}
