// Package executors 提供任务执行器实现
package executors

import (
	"github.com/AgentNetworkPlan/AgentNetwork/internal/execution"
)

// 重导出 execution 包中的类型，便于使用
type (
	Executor        = execution.Executor
	ExecutionResult = execution.ExecutionResult
	ExecutionJob    = execution.ExecutionJob
	ExecutorInfo    = execution.ExecutorInfo
	BaseExecutor    = execution.BaseExecutor
	JobType         = execution.JobType
	Artifact        = execution.Artifact
	ResourceEstimate = execution.ResourceEstimate
	ResourceUsage   = execution.ResourceUsage
	ResourceLimit   = execution.ResourceLimit
)

// 重导出 execution 包中的常量
const (
	JobTypeSearch  = execution.JobTypeSearch
	JobTypeCompute = execution.JobTypeCompute
	JobTypeLLM     = execution.JobTypeLLM
	JobTypeCustom  = execution.JobTypeCustom
)

// 重导出 execution 包中的函数
var (
	NewBaseExecutor  = execution.NewBaseExecutor
	NewSuccessResult = execution.NewSuccessResult
	NewErrorResult   = execution.NewErrorResult
)
