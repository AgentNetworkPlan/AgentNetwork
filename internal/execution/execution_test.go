package execution

import (
	"testing"
	"time"
)

func TestNewExecutionJob(t *testing.T) {
	job := NewExecutionJob("task1", JobTypeSearch, map[string]any{"query": "test"})

	if job.TaskID != "task1" {
		t.Errorf("Expected task1, got %s", job.TaskID)
	}
	if job.Type != JobTypeSearch {
		t.Errorf("Expected search type, got %s", job.Type)
	}
	if job.Status != JobPending {
		t.Errorf("Expected pending status, got %s", job.Status)
	}
	if job.Priority != PriorityNormal {
		t.Errorf("Expected normal priority, got %d", job.Priority)
	}
	if job.Timeout != 300 {
		t.Errorf("Expected 300s timeout, got %d", job.Timeout)
	}
}

func TestJobStatusTransitions(t *testing.T) {
	job := NewExecutionJob("task1", JobTypeCompute, nil)

	// Pending -> Running
	job.SetRunning()
	if job.Status != JobRunning {
		t.Errorf("Expected running, got %s", job.Status)
	}
	if job.StartedAt == 0 {
		t.Error("StartedAt should be set")
	}

	// Running -> Completed
	job.SetCompleted(map[string]any{"result": "done"}, nil)
	if job.Status != JobCompleted {
		t.Errorf("Expected completed, got %s", job.Status)
	}
	if job.Progress != 100 {
		t.Errorf("Expected 100 progress, got %f", job.Progress)
	}
	if job.CompletedAt == 0 {
		t.Error("CompletedAt should be set")
	}
}

func TestJobFailed(t *testing.T) {
	job := NewExecutionJob("task1", JobTypeCompute, nil)
	job.SetRunning()
	job.SetFailed("test error")

	if job.Status != JobFailed {
		t.Errorf("Expected failed, got %s", job.Status)
	}
	if job.Error != "test error" {
		t.Errorf("Expected 'test error', got %s", job.Error)
	}
}

func TestJobCancelled(t *testing.T) {
	job := NewExecutionJob("task1", JobTypeCompute, nil)
	job.SetCancelled()

	if job.Status != JobCancelled {
		t.Errorf("Expected cancelled, got %s", job.Status)
	}
}

func TestJobTimeout(t *testing.T) {
	job := NewExecutionJob("task1", JobTypeCompute, nil)
	job.SetRunning()
	job.SetTimeout()

	if job.Status != JobTimeout {
		t.Errorf("Expected timeout, got %s", job.Status)
	}
	if job.Error != "execution timeout" {
		t.Errorf("Expected 'execution timeout', got %s", job.Error)
	}
}

func TestJobIsTerminal(t *testing.T) {
	tests := []struct {
		status   JobStatus
		terminal bool
	}{
		{JobPending, false},
		{JobQueued, false},
		{JobRunning, false},
		{JobCompleted, true},
		{JobFailed, true},
		{JobCancelled, true},
		{JobTimeout, true},
	}

	for _, tc := range tests {
		job := NewExecutionJob("task1", JobTypeCompute, nil)
		job.Status = tc.status
		if job.IsTerminal() != tc.terminal {
			t.Errorf("Status %s: expected IsTerminal=%v, got %v", tc.status, tc.terminal, job.IsTerminal())
		}
	}
}

func TestJobCanRetry(t *testing.T) {
	job := NewExecutionJob("task1", JobTypeCompute, nil)
	job.MaxRetries = 3

	// Failed with retries available
	job.Status = JobFailed
	job.RetryCount = 0
	if !job.CanRetry() {
		t.Error("Should be able to retry")
	}

	// Exhausted retries
	job.RetryCount = 3
	if job.CanRetry() {
		t.Error("Should not be able to retry")
	}

	// Not failed
	job.Status = JobCompleted
	job.RetryCount = 0
	if job.CanRetry() {
		t.Error("Completed jobs should not retry")
	}
}

func TestJobDuration(t *testing.T) {
	job := NewExecutionJob("task1", JobTypeCompute, nil)

	// Not started yet
	if job.Duration() != 0 {
		t.Error("Duration should be 0 before start")
	}

	// Started but not completed
	job.StartedAt = time.Now().Unix() - 5
	duration := job.Duration()
	if duration < 5*time.Second || duration > 6*time.Second {
		t.Errorf("Expected ~5s duration, got %v", duration)
	}

	// Completed
	job.CompletedAt = job.StartedAt + 10
	if job.Duration() != 10*time.Second {
		t.Errorf("Expected 10s duration, got %v", job.Duration())
	}
}

func TestJobUpdateProgress(t *testing.T) {
	job := NewExecutionJob("task1", JobTypeCompute, nil)

	job.UpdateProgress(50, "halfway done")
	if job.Progress != 50 {
		t.Errorf("Expected 50, got %f", job.Progress)
	}
	if job.Message != "halfway done" {
		t.Errorf("Expected 'halfway done', got %s", job.Message)
	}

	// Test bounds
	job.UpdateProgress(-10, "")
	if job.Progress != 0 {
		t.Errorf("Progress should be capped at 0, got %f", job.Progress)
	}

	job.UpdateProgress(150, "")
	if job.Progress != 100 {
		t.Errorf("Progress should be capped at 100, got %f", job.Progress)
	}
}

func TestPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue()

	// Create jobs with different priorities
	job1 := NewExecutionJob("task1", JobTypeCompute, nil)
	job1.ID = "job1"
	job1.Priority = PriorityLow
	job1.CreatedAt = 100

	job2 := NewExecutionJob("task2", JobTypeCompute, nil)
	job2.ID = "job2"
	job2.Priority = PriorityHigh
	job2.CreatedAt = 200

	job3 := NewExecutionJob("task3", JobTypeCompute, nil)
	job3.ID = "job3"
	job3.Priority = PriorityNormal
	job3.CreatedAt = 300

	// Enqueue
	pq.Enqueue(job1)
	pq.Enqueue(job2)
	pq.Enqueue(job3)

	if pq.Size() != 3 {
		t.Errorf("Expected size 3, got %d", pq.Size())
	}

	// Dequeue should return highest priority first
	dequeued := pq.Dequeue()
	if dequeued.ID != "job2" {
		t.Errorf("Expected job2 (high priority), got %s", dequeued.ID)
	}

	dequeued = pq.Dequeue()
	if dequeued.ID != "job3" {
		t.Errorf("Expected job3 (normal priority), got %s", dequeued.ID)
	}

	dequeued = pq.Dequeue()
	if dequeued.ID != "job1" {
		t.Errorf("Expected job1 (low priority), got %s", dequeued.ID)
	}

	// Queue should be empty
	if pq.Size() != 0 {
		t.Errorf("Expected size 0, got %d", pq.Size())
	}

	// Dequeue empty queue
	if pq.Dequeue() != nil {
		t.Error("Dequeue from empty queue should return nil")
	}
}

func TestPriorityQueueSamePriority(t *testing.T) {
	pq := NewPriorityQueue()

	// Jobs with same priority should be ordered by creation time
	job1 := NewExecutionJob("task1", JobTypeCompute, nil)
	job1.ID = "job1"
	job1.Priority = PriorityNormal
	job1.CreatedAt = 100

	job2 := NewExecutionJob("task2", JobTypeCompute, nil)
	job2.ID = "job2"
	job2.Priority = PriorityNormal
	job2.CreatedAt = 200

	pq.Enqueue(job2) // Later
	pq.Enqueue(job1) // Earlier

	// Earlier job should come out first
	dequeued := pq.Dequeue()
	if dequeued.ID != "job1" {
		t.Errorf("Expected job1 (earlier), got %s", dequeued.ID)
	}
}

func TestPriorityQueueRemove(t *testing.T) {
	pq := NewPriorityQueue()

	job1 := NewExecutionJob("task1", JobTypeCompute, nil)
	job1.ID = "job1"

	job2 := NewExecutionJob("task2", JobTypeCompute, nil)
	job2.ID = "job2"

	pq.Enqueue(job1)
	pq.Enqueue(job2)

	// Remove job1
	if !pq.Remove("job1") {
		t.Error("Remove should return true")
	}

	if pq.Size() != 1 {
		t.Errorf("Expected size 1, got %d", pq.Size())
	}

	if pq.Contains("job1") {
		t.Error("Queue should not contain job1")
	}

	// Remove non-existent
	if pq.Remove("job999") {
		t.Error("Remove non-existent should return false")
	}
}

func TestPriorityQueueUpdatePriority(t *testing.T) {
	pq := NewPriorityQueue()

	job1 := NewExecutionJob("task1", JobTypeCompute, nil)
	job1.ID = "job1"
	job1.Priority = PriorityLow

	job2 := NewExecutionJob("task2", JobTypeCompute, nil)
	job2.ID = "job2"
	job2.Priority = PriorityHigh

	pq.Enqueue(job1)
	pq.Enqueue(job2)

	// job2 should be first
	if pq.Peek().ID != "job2" {
		t.Error("job2 should be first")
	}

	// Update job1 to critical priority
	pq.UpdatePriority("job1", PriorityCritical)

	// Now job1 should be first
	if pq.Peek().ID != "job1" {
		t.Error("job1 should be first after priority update")
	}
}

func TestPriorityQueueClear(t *testing.T) {
	pq := NewPriorityQueue()

	for i := 0; i < 5; i++ {
		job := NewExecutionJob("task", JobTypeCompute, nil)
		job.ID = string(rune('a' + i))
		pq.Enqueue(job)
	}

	pq.Clear()

	if pq.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", pq.Size())
	}
}

func TestPriorityQueueList(t *testing.T) {
	pq := NewPriorityQueue()

	job1 := NewExecutionJob("task1", JobTypeCompute, nil)
	job1.ID = "job1"

	job2 := NewExecutionJob("task2", JobTypeCompute, nil)
	job2.ID = "job2"

	pq.Enqueue(job1)
	pq.Enqueue(job2)

	jobs := pq.List()
	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}
}
