package auth

import (
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	task := NewTask(
		identity.NodeID(),
		"compute",
		[]byte("test payload"),
		3,
		time.Now().Add(time.Hour),
	)
	
	if task.TaskID == "" {
		t.Error("任务 ID 不应为空")
	}

	if task.TaskType != "compute" {
		t.Errorf("任务类型不匹配")
	}

	if task.Status != TaskPending {
		t.Errorf("初始状态应为 pending")
	}

	if task.Difficulty != 3 {
		t.Errorf("难度不匹配")
	}
}

func TestTask_SignTask(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	task := NewTask(
		identity.NodeID(),
		"inference",
		nil,
		5,
		time.Now().Add(time.Hour),
	)

	err := task.SignTask(identity)
	if err != nil {
		t.Fatalf("任务签名失败: %v", err)
	}

	if len(task.RequesterSig) == 0 {
		t.Error("签名不应为空")
	}
}

func TestTask_VerifyRequesterSignature(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	task := NewTask(
		identity.NodeID(),
		"storage",
		nil,
		2,
		time.Now().Add(time.Hour),
	)

	task.SignTask(identity)

	valid, err := task.VerifyRequesterSignature(identity.PublicKeyHex())
	if err != nil {
		t.Fatalf("验证签名失败: %v", err)
	}
	if !valid {
		t.Error("任务签名验证失败")
	}
}

func TestTask_AssignTo(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	task := NewTask(
		identity.NodeID(),
		"compute",
		nil,
		3,
		time.Now().Add(time.Hour),
	)

	err := task.AssignTo("worker-001")
	if err != nil {
		t.Fatalf("分配任务失败: %v", err)
	}

	if task.WorkerID != "worker-001" {
		t.Error("WorkerID 不匹配")
	}

	if task.Status != TaskAssigned {
		t.Error("状态应为 assigned")
	}
}

func TestNewProofOfTask(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	result := []byte("task result")
	proof := NewProofOfTask("task-001", identity.NodeID(), result, 1000)
	
	if proof.TaskID != "task-001" {
		t.Error("任务 ID 不匹配")
	}

	if proof.WorkerID != identity.NodeID() {
		t.Error("工作节点 ID 不匹配")
	}

	if proof.ResultHash == "" {
		t.Error("结果哈希不应为空")
	}

	if proof.ExecutionTime != 1000 {
		t.Error("执行时间不匹配")
	}
}

func TestProofOfTask_Sign(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	result := []byte("task result")
	proof := NewProofOfTask("task-001", identity.NodeID(), result, 1000)

	err := proof.Sign(identity)
	if err != nil {
		t.Fatalf("证明签名失败: %v", err)
	}

	if len(proof.WorkerSignature) == 0 {
		t.Error("签名不应为空")
	}
}

func TestProofOfTask_VerifySignature(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	result := []byte("task result")
	proof := NewProofOfTask("task-001", identity.NodeID(), result, 1000)
	proof.Sign(identity)

	valid, err := proof.VerifySignature(identity.PublicKeyHex())
	if err != nil {
		t.Fatalf("验证签名失败: %v", err)
	}
	if !valid {
		t.Error("证明签名验证失败")
	}
}

func TestProofOfTask_AddIntermediateHash(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	proof := NewProofOfTask("task-001", identity.NodeID(), []byte("result"), 1000)

	proof.AddIntermediateHash([]byte("step1"))
	proof.AddIntermediateHash([]byte("step2"))

	if len(proof.IntermediateHash) != 2 {
		t.Errorf("中间哈希数量应为 2，实际为 %d", len(proof.IntermediateHash))
	}
}

func TestTaskVerification(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	verification := TaskVerification{
		TaskID:     "task-001",
		VerifierID: identity.NodeID(),
		IsValid:    true,
		Reason:     "验证通过",
		Timestamp:  time.Now(),
	}

	if verification.VerifierID != identity.NodeID() {
		t.Error("验证者 ID 不匹配")
	}

	if !verification.IsValid {
		t.Error("验证结果应为通过")
	}
}

func TestTaskManager(t *testing.T) {
	manager := NewTaskManager()
	identity, _ := NewNodeIdentity()

	task := NewTask(
		identity.NodeID(),
		"compute",
		nil,
		3,
		time.Now().Add(time.Hour),
	)
	task.SignTask(identity)

	// 提交任务
	err := manager.SubmitTask(task)
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 获取任务
	retrieved, err := manager.GetTask(task.TaskID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}

	if retrieved.TaskID != task.TaskID {
		t.Error("获取的任务 ID 不匹配")
	}
}

func TestTaskManager_AssignTask(t *testing.T) {
	manager := NewTaskManager()
	identity, _ := NewNodeIdentity()

	task := NewTask(
		identity.NodeID(),
		"compute",
		nil,
		3,
		time.Now().Add(time.Hour),
	)
	task.SignTask(identity)
	manager.SubmitTask(task)

	// 分配任务
	err := manager.AssignTask(task.TaskID, "worker-001")
	if err != nil {
		t.Fatalf("分配任务失败: %v", err)
	}

	retrieved, _ := manager.GetTask(task.TaskID)
	if retrieved.WorkerID != "worker-001" {
		t.Error("WorkerID 不匹配")
	}
	if retrieved.Status != TaskAssigned {
		t.Error("状态应为 assigned")
	}
}

func TestTaskManager_SubmitProof(t *testing.T) {
	manager := NewTaskManager()
	requester, _ := NewNodeIdentity()
	worker, _ := NewNodeIdentity()

	task := NewTask(
		requester.NodeID(),
		"compute",
		nil,
		3,
		time.Now().Add(time.Hour),
	)
	task.SignTask(requester)
	manager.SubmitTask(task)
	manager.AssignTask(task.TaskID, worker.NodeID())

	// 创建并签名证明
	proof := NewProofOfTask(task.TaskID, worker.NodeID(), []byte("result"), 500)
	proof.Sign(worker)

	err := manager.SubmitProof(proof)
	if err != nil {
		t.Fatalf("提交证明失败: %v", err)
	}

	// 获取证明
	retrieved, err := manager.GetProof(task.TaskID)
	if err != nil {
		t.Fatalf("获取证明失败: %v", err)
	}

	if retrieved.TaskID != proof.TaskID {
		t.Error("获取的证明任务 ID 不匹配")
	}
}

func TestTaskManager_GetPendingTasks(t *testing.T) {
	manager := NewTaskManager()
	identity, _ := NewNodeIdentity()

	// 创建多个任务
	for i := 0; i < 5; i++ {
		task := NewTask(
			identity.NodeID(),
			"compute",
			nil,
			3,
			time.Now().Add(time.Hour),
		)
		task.SignTask(identity)
		manager.SubmitTask(task)
	}

	// 获取待处理任务
	pendingTasks := manager.GetPendingTasks()
	if len(pendingTasks) != 5 {
		t.Errorf("待处理任务数量应为 5，实际为 %d", len(pendingTasks))
	}
}

func TestTaskManager_GetTasksByWorker(t *testing.T) {
	manager := NewTaskManager()
	identity, _ := NewNodeIdentity()

	// 创建任务并分配给不同的 worker
	for i := 0; i < 3; i++ {
		task := NewTask(identity.NodeID(), "compute", nil, 3, time.Now().Add(time.Hour))
		task.SignTask(identity)
		manager.SubmitTask(task)
		manager.AssignTask(task.TaskID, "worker-001")
	}

	task := NewTask(identity.NodeID(), "compute", nil, 3, time.Now().Add(time.Hour))
	task.SignTask(identity)
	manager.SubmitTask(task)
	manager.AssignTask(task.TaskID, "worker-002")

	// 获取 worker-001 的任务
	workerTasks := manager.GetTasksByWorker("worker-001")
	if len(workerTasks) != 3 {
		t.Errorf("worker-001 应有 3 个任务，实际有 %d", len(workerTasks))
	}
}

func TestTask_IsExpired(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	// 创建一个已过期的任务
	task := NewTask(
		identity.NodeID(),
		"compute",
		nil,
		3,
		time.Now().Add(-time.Hour), // 过去的时间
	)

	if !task.IsExpired() {
		t.Error("任务应该已过期")
	}

	// 创建一个未过期的任务
	task2 := NewTask(
		identity.NodeID(),
		"compute",
		nil,
		3,
		time.Now().Add(time.Hour), // 将来的时间
	)

	if task2.IsExpired() {
		t.Error("任务不应该过期")
	}
}
