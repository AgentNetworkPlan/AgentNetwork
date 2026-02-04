package task

import (
	"os"
	"testing"
	"time"
)

func TestNewTaskManager(t *testing.T) {
	config := &TaskManagerConfig{
		DataDir:           t.TempDir(),
		DefaultBidding:    10 * time.Minute,
		MaxTasksPerHour:   2,
		ReputationBonus:   0.05,
		MinRepToPublish:   30.0,
		DepositMultiplier: 1.2,
	}

	tm := NewTaskManager(config)
	if tm == nil {
		t.Fatal("NewTaskManager returned nil")
	}
}

func TestPublishTask(t *testing.T) {
	config := &TaskManagerConfig{
		DataDir:           t.TempDir(),
		DefaultBidding:    10 * time.Minute,
		MaxTasksPerHour:   5,
		ReputationBonus:   0.05,
		MinRepToPublish:   30.0,
		DepositMultiplier: 1.2,
	}
	tm := NewTaskManager(config)

	task := &Task{
		Type:        TaskTypeSearch,
		Title:       "Search for AI papers",
		Description: "Find recent papers about LLM",
		RequesterID: "node1",
		Reward:      5.0,
		Deadline:    time.Now().Add(24 * time.Hour).Unix(),
	}

	// Should fail with insufficient reputation
	err := tm.PublishTask(task, 20.0)
	if err == nil {
		t.Error("Should fail with insufficient reputation")
	}

	// Should succeed with sufficient reputation
	err = tm.PublishTask(task, 50.0)
	if err != nil {
		t.Errorf("PublishTask failed: %v", err)
	}

	if task.ID == "" {
		t.Error("Task ID should be generated")
	}

	if task.Status != StatusPublished {
		t.Errorf("Task status should be published, got %s", task.Status)
	}

	// Verify deposit was calculated
	if task.RequesterDeposit != task.Reward*1.2 {
		t.Errorf("Deposit should be %.1f, got %.1f", task.Reward*1.2, task.RequesterDeposit)
	}
}

func TestTaskBidding(t *testing.T) {
	config := &TaskManagerConfig{
		DataDir:           t.TempDir(),
		DefaultBidding:    10 * time.Minute,
		MaxTasksPerHour:   5,
		ReputationBonus:   0.05,
		MinRepToPublish:   30.0,
		DepositMultiplier: 1.2,
	}
	tm := NewTaskManager(config)

	task := &Task{
		Type:          TaskTypeSearch,
		Title:         "Search task",
		RequesterID:   "node1",
		Reward:        5.0,
		BiddingPeriod: 3600, // 1 hour
		MinReputation: 20.0,
	}

	err := tm.PublishTask(task, 50.0)
	if err != nil {
		t.Fatalf("PublishTask failed: %v", err)
	}

	// Submit bid
	bid := &TaskBid{
		TaskID:        task.ID,
		BidderID:      "node2",
		BidAmount:     4.5,
		EstimatedTime: 1800,
		Reputation:    40.0,
		Message:       "I can do this quickly",
	}

	err = tm.SubmitBid(bid)
	if err != nil {
		t.Errorf("SubmitBid failed: %v", err)
	}

	// Verify bid was added
	updatedTask, _ := tm.GetTask(task.ID)
	if len(updatedTask.Bids) != 1 {
		t.Errorf("Expected 1 bid, got %d", len(updatedTask.Bids))
	}

	// Submit bid with insufficient reputation
	lowRepBid := &TaskBid{
		TaskID:     task.ID,
		BidderID:   "node3",
		BidAmount:  4.0,
		Reputation: 10.0, // Below minimum
	}

	err = tm.SubmitBid(lowRepBid)
	if err == nil {
		t.Error("Should fail with insufficient reputation")
	}
}

func TestTaskClaim(t *testing.T) {
	config := &TaskManagerConfig{
		DataDir:           t.TempDir(),
		DefaultBidding:    0, // No bidding period for claim mode
		MaxTasksPerHour:   5,
		ReputationBonus:   0.05,
		MinRepToPublish:   30.0,
		DepositMultiplier: 1.2,
	}
	tm := NewTaskManager(config)

	task := &Task{
		Type:        TaskTypeSearch,
		Title:       "Quick task",
		RequesterID: "node1",
		Reward:      5.0,
		PublishMode: ModeBroadcast,
	}
	// Set bidding period to 0 for claim mode
	task.BiddingPeriod = 0

	err := tm.PublishTask(task, 50.0)
	if err != nil {
		t.Fatalf("PublishTask failed: %v", err)
	}
	// Override bidding period after publish
	task.BiddingPeriod = 0
	task.BiddingEndsAt = 0

	claim := &TaskClaim{
		TaskID:    task.ID,
		ClaimerID: "node2",
		ClaimTime: time.Now().Unix(),
	}

	err = tm.ClaimTask(claim, 30.0)
	if err != nil {
		t.Errorf("ClaimTask failed: %v", err)
	}

	updatedTask, _ := tm.GetTask(task.ID)
	if updatedTask.ExecutorID != "node2" {
		t.Errorf("Executor should be node2, got %s", updatedTask.ExecutorID)
	}

	if updatedTask.Status != StatusAccepted {
		t.Errorf("Status should be accepted, got %s", updatedTask.Status)
	}
}

func TestTaskLifecycle(t *testing.T) {
	config := &TaskManagerConfig{
		DataDir:           t.TempDir(),
		DefaultBidding:    0,
		MaxTasksPerHour:   5,
		ReputationBonus:   0.05,
		MinRepToPublish:   30.0,
		DepositMultiplier: 1.2,
	}
	tm := NewTaskManager(config)

	// 1. Publish task
	task := &Task{
		Type:        TaskTypeSearch,
		Title:       "Test task",
		RequesterID: "requester1",
		Reward:      10.0,
	}

	err := tm.PublishTask(task, 50.0)
	if err != nil {
		t.Fatalf("PublishTask failed: %v", err)
	}

	// 2. Assign task
	assignment := &TaskAssignment{
		TaskID:     task.ID,
		AssignedTo: "executor1",
	}

	err = tm.AssignTask(assignment)
	if err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}

	// 3. Start execution
	err = tm.StartExecution(task.ID, "executor1")
	if err != nil {
		t.Fatalf("StartExecution failed: %v", err)
	}

	updatedTask, _ := tm.GetTask(task.ID)
	if updatedTask.Status != StatusInProgress {
		t.Errorf("Status should be in_progress, got %s", updatedTask.Status)
	}

	// 4. Submit delivery
	err = tm.SubmitDelivery(task.ID, "executor1", "hash123", "sig456")
	if err != nil {
		t.Fatalf("SubmitDelivery failed: %v", err)
	}

	// 5. Confirm delivery
	err = tm.ConfirmDelivery(task.ID, "requester1", "sig789")
	if err != nil {
		t.Fatalf("ConfirmDelivery failed: %v", err)
	}

	updatedTask, _ = tm.GetTask(task.ID)
	if updatedTask.Status != StatusVerified {
		t.Errorf("Status should be verified, got %s", updatedTask.Status)
	}

	// 6. Settle task
	result, err := tm.SettleTask(task.ID)
	if err != nil {
		t.Fatalf("SettleTask failed: %v", err)
	}

	if result.RewardAmount != 10.0 {
		t.Errorf("Reward should be 10.0, got %.1f", result.RewardAmount)
	}

	// 7. Complete task
	err = tm.CompleteTask(task.ID)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	updatedTask, _ = tm.GetTask(task.ID)
	if updatedTask.Status != StatusCompleted {
		t.Errorf("Status should be completed, got %s", updatedTask.Status)
	}
}

func TestTaskDispute(t *testing.T) {
	config := &TaskManagerConfig{
		DataDir:           t.TempDir(),
		MaxTasksPerHour:   5,
		MinRepToPublish:   30.0,
		DepositMultiplier: 1.2,
	}
	tm := NewTaskManager(config)

	task := &Task{
		Type:        TaskTypeSearch,
		Title:       "Disputed task",
		RequesterID: "requester1",
		Reward:      10.0,
	}

	tm.PublishTask(task, 50.0)

	assignment := &TaskAssignment{
		TaskID:     task.ID,
		AssignedTo: "executor1",
	}
	tm.AssignTask(assignment)
	tm.StartExecution(task.ID, "executor1")

	// Dispute the task
	err := tm.DisputeTask(task.ID, "requester1", "Quality issue")
	if err != nil {
		t.Errorf("DisputeTask failed: %v", err)
	}

	updatedTask, _ := tm.GetTask(task.ID)
	if updatedTask.Status != StatusDisputed {
		t.Errorf("Status should be disputed, got %s", updatedTask.Status)
	}
}

func TestTaskCancel(t *testing.T) {
	config := &TaskManagerConfig{
		DataDir:         t.TempDir(),
		MaxTasksPerHour: 5,
		MinRepToPublish: 30.0,
	}
	tm := NewTaskManager(config)

	task := &Task{
		Type:        TaskTypeSearch,
		Title:       "Cancelled task",
		RequesterID: "requester1",
		Reward:      10.0,
	}

	tm.PublishTask(task, 50.0)

	// Only requester can cancel
	err := tm.CancelTask(task.ID, "other_node")
	if err == nil {
		t.Error("Only requester should be able to cancel")
	}

	err = tm.CancelTask(task.ID, "requester1")
	if err != nil {
		t.Errorf("CancelTask failed: %v", err)
	}

	updatedTask, _ := tm.GetTask(task.ID)
	if updatedTask.Status != StatusCancelled {
		t.Errorf("Status should be cancelled, got %s", updatedTask.Status)
	}
}

func TestTaskQueries(t *testing.T) {
	config := &TaskManagerConfig{
		DataDir:         t.TempDir(),
		MaxTasksPerHour: 10,
		MinRepToPublish: 30.0,
	}
	tm := NewTaskManager(config)

	// Create multiple tasks
	for i := 0; i < 3; i++ {
		task := &Task{
			Type:        TaskTypeSearch,
			Title:       "Task",
			RequesterID: "requester1",
			Reward:      float64(i + 1),
		}
		tm.PublishTask(task, 50.0)
	}

	// Query by requester
	tasks := tm.GetTasksByRequester("requester1")
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	// Query by status
	openTasks := tm.GetOpenTasks()
	if len(openTasks) != 3 {
		t.Errorf("Expected 3 open tasks, got %d", len(openTasks))
	}

	// Query statistics
	stats := tm.GetStatistics()
	if stats.TotalTasks != 3 {
		t.Errorf("Expected 3 total tasks, got %d", stats.TotalTasks)
	}
}

func TestCapabilityRegistry(t *testing.T) {
	tm := NewTaskManager(&TaskManagerConfig{DataDir: t.TempDir()})

	// Register capabilities
	cap1 := &AgentCapability{
		AgentID:      "agent1",
		Capabilities: []string{"coding", "search"},
	}
	cap2 := &AgentCapability{
		AgentID:      "agent2",
		Capabilities: []string{"translation", "search"},
	}

	tm.RegisterCapability(cap1)
	tm.RegisterCapability(cap2)

	// Find agents with search capability
	agents := tm.FindAgentsByCapability([]string{"search"})
	if len(agents) != 2 {
		t.Errorf("Expected 2 agents with search, got %d", len(agents))
	}

	// Find agents with both coding and search
	agents = tm.FindAgentsByCapability([]string{"coding", "search"})
	if len(agents) != 1 || agents[0] != "agent1" {
		t.Errorf("Expected only agent1, got %v", agents)
	}
}

func TestTaskFilter(t *testing.T) {
	filter := NewTaskFilter()

	// Test normal task
	normalTask := &Task{
		Type:        TaskTypeSearch,
		Title:       "Search papers",
		Description: "Find research papers about AI",
	}

	allowed, _ := filter.IsAllowed(normalTask)
	if !allowed {
		t.Error("Normal task should be allowed")
	}

	// Test blocked task
	blockedTask := &Task{
		Type:        TaskTypeSearch,
		Title:       "hack website",
		Description: "Help me attack this server",
	}

	allowed, reason := filter.IsAllowed(blockedTask)
	if allowed {
		t.Error("Blocked task should not be allowed")
	}
	if reason == "" {
		t.Error("Should provide reason for blocking")
	}
}

func TestTaskRateLimiter(t *testing.T) {
	limiter := NewTaskRateLimiter()

	// Should be able to publish within quota
	can, _ := limiter.CanPublish("node1", 50.0)
	if !can {
		t.Error("Should be able to publish")
	}

	// Consume quota
	limiter.ConsumeQuota("node1")
	limiter.ConsumeQuota("node1")
	limiter.ConsumeQuota("node1")
	limiter.ConsumeQuota("node1")

	// Check remaining quota
	remaining := limiter.GetRemainingQuota("node1", 50.0)
	// Base 2 + 50*0.05 = 4.5 -> 4, consumed 4, remaining 0
	if remaining != 0 {
		t.Errorf("Expected 0 remaining, got %d", remaining)
	}

	// Should exceed quota
	can, reason := limiter.CanPublish("node1", 50.0)
	if can {
		t.Error("Should not be able to publish when quota exceeded")
	}
	if reason != "quota exceeded" {
		t.Errorf("Unexpected reason: %s", reason)
	}

	// Test cooldown
	limiter.ApplyCooldown("node1")
	can, reason = limiter.CanPublish("node1", 50.0)
	if can {
		t.Error("Should not be able to publish during cooldown")
	}
}

func TestTaskValidation(t *testing.T) {
	// Valid task
	validTask := &Task{
		ID:          "task1",
		Type:        TaskTypeSearch,
		Title:       "Test",
		RequesterID: "node1",
		Reward:      5.0,
	}

	if !validTask.IsValid() {
		t.Error("Valid task should pass validation")
	}

	// Invalid tasks
	invalidTasks := []*Task{
		{}, // Empty
		{ID: "task1", Type: TaskTypeSearch, Title: "Test"}, // Missing requester
		{ID: "task1", RequesterID: "node1", Title: "Test"}, // Missing type
		{ID: "task1", Type: TaskTypeSearch, RequesterID: "node1", Reward: -1}, // Negative reward
	}

	for i, task := range invalidTasks {
		if task.IsValid() {
			t.Errorf("Invalid task %d should not pass validation", i)
		}
	}
}

func TestTaskStateTransitions(t *testing.T) {
	task := &Task{
		Status: StatusPublished,
	}

	// Valid transitions
	if !task.CanTransition(StatusAccepted) {
		t.Error("Should be able to transition from published to accepted")
	}

	// Invalid transitions
	if task.CanTransition(StatusCompleted) {
		t.Error("Should not be able to transition directly to completed")
	}

	task.Status = StatusDelivered
	if !task.CanTransition(StatusVerified) {
		t.Error("Should be able to transition from delivered to verified")
	}

	if !task.CanTransition(StatusDisputed) {
		t.Error("Should be able to transition from delivered to disputed")
	}
}

func TestTaskPersistence(t *testing.T) {
	tempDir := t.TempDir()

	config := &TaskManagerConfig{
		DataDir:         tempDir,
		MaxTasksPerHour: 5,
		MinRepToPublish: 30.0,
	}

	// Create and publish task
	tm1 := NewTaskManager(config)
	task := &Task{
		Type:        TaskTypeSearch,
		Title:       "Persisted task",
		RequesterID: "node1",
		Reward:      10.0,
	}
	tm1.PublishTask(task, 50.0)
	taskID := task.ID

	// Create new manager and verify persistence
	tm2 := NewTaskManager(config)
	loadedTask, err := tm2.GetTask(taskID)
	if err != nil {
		t.Fatalf("Failed to load task: %v", err)
	}

	if loadedTask.Title != "Persisted task" {
		t.Errorf("Expected title 'Persisted task', got '%s'", loadedTask.Title)
	}

	// Cleanup
	os.RemoveAll(tempDir)
}

func TestTaskSizeValidation(t *testing.T) {
	// Valid task
	validTask := &Task{
		Title:       "Valid title",
		Description: "Valid description",
	}

	ok, _ := ValidateTaskSize(validTask)
	if !ok {
		t.Error("Valid task should pass size validation")
	}

	// Title too long
	longTitle := make([]byte, MaxTaskTitleSize+1)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	invalidTask := &Task{
		Title: string(longTitle),
	}

	ok, reason := ValidateTaskSize(invalidTask)
	if ok {
		t.Error("Task with long title should fail validation")
	}
	if reason != "title too long" {
		t.Errorf("Unexpected reason: %s", reason)
	}
}
