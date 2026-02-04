// Package executors 提供任务执行器实现
package executors

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/execution"
)

// ComputeExecutor 计算执行器
type ComputeExecutor struct {
	*execution.BaseExecutor
}

// NewComputeExecutor 创建计算执行器
func NewComputeExecutor() *ComputeExecutor {
	return &ComputeExecutor{
		BaseExecutor: execution.NewBaseExecutor("compute", "1.0.0", []execution.JobType{execution.JobTypeCompute}),
	}
}

// CanExecute 检查是否能执行
func (e *ComputeExecutor) CanExecute(job *execution.ExecutionJob) bool {
	if !e.BaseExecutor.CanExecute(job) {
		return false
	}
	_, hasOp := job.Input["operation"]
	return hasOp
}

// EstimateResources 估算资源
func (e *ComputeExecutor) EstimateResources(job *execution.ExecutionJob) (*execution.ResourceEstimate, error) {
	return &execution.ResourceEstimate{
		CPUTimeMs:   1000,
		MemoryBytes: 64 * 1024 * 1024, // 64MB
		DurationSec: 30,
	}, nil
}

// Execute 执行计算
func (e *ComputeExecutor) Execute(ctx context.Context, job *execution.ExecutionJob) (*execution.ExecutionResult, error) {
	operation, ok := job.Input["operation"].(string)
	if !ok || operation == "" {
		return execution.NewErrorResult("missing or invalid operation parameter"), nil
	}

	var result any
	var err error

	switch operation {
	case "hash":
		result, err = e.computeHash(job)
	case "encode":
		result, err = e.computeEncode(job)
	case "decode":
		result, err = e.computeDecode(job)
	case "transform":
		result, err = e.computeTransform(job)
	case "aggregate":
		result, err = e.computeAggregate(job)
	case "validate":
		result, err = e.computeValidate(job)
	default:
		return execution.NewErrorResult(fmt.Sprintf("unsupported operation: %s", operation)), nil
	}

	if err != nil {
		return execution.NewErrorResult(err.Error()), nil
	}

	return execution.NewSuccessResult(map[string]any{
		"operation": operation,
		"result":    result,
	}, nil), nil
}

// computeHash 计算哈希
func (e *ComputeExecutor) computeHash(job *execution.ExecutionJob) (any, error) {
	data, ok := job.Input["data"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid data parameter")
	}

	algorithm, _ := job.Input["algorithm"].(string)
	if algorithm == "" {
		algorithm = "sha256"
	}

	switch algorithm {
	case "sha256":
		hash := sha256.Sum256([]byte(data))
		return hex.EncodeToString(hash[:]), nil
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

// computeEncode 编码
func (e *ComputeExecutor) computeEncode(job *execution.ExecutionJob) (any, error) {
	data, ok := job.Input["data"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid data parameter")
	}

	format, _ := job.Input["format"].(string)
	if format == "" {
		format = "base64"
	}

	switch format {
	case "base64":
		return base64.StdEncoding.EncodeToString([]byte(data)), nil
	case "hex":
		return hex.EncodeToString([]byte(data)), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// computeDecode 解码
func (e *ComputeExecutor) computeDecode(job *execution.ExecutionJob) (any, error) {
	data, ok := job.Input["data"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid data parameter")
	}

	format, _ := job.Input["format"].(string)
	if format == "" {
		format = "base64"
	}

	switch format {
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, err
		}
		return string(decoded), nil
	case "hex":
		decoded, err := hex.DecodeString(data)
		if err != nil {
			return nil, err
		}
		return string(decoded), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// computeTransform 数据转换
func (e *ComputeExecutor) computeTransform(job *execution.ExecutionJob) (any, error) {
	data, ok := job.Input["data"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid data parameter")
	}

	transformType, _ := job.Input["transform_type"].(string)
	if transformType == "" {
		return nil, fmt.Errorf("missing transform_type parameter")
	}

	switch transformType {
	case "uppercase":
		return strings.ToUpper(data), nil
	case "lowercase":
		return strings.ToLower(data), nil
	case "trim":
		return strings.TrimSpace(data), nil
	case "reverse":
		runes := []rune(data)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes), nil
	case "json_parse":
		var result any
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return nil, err
		}
		return result, nil
	case "json_stringify":
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		return string(jsonBytes), nil
	default:
		return nil, fmt.Errorf("unsupported transform type: %s", transformType)
	}
}

// computeAggregate 数据聚合
func (e *ComputeExecutor) computeAggregate(job *execution.ExecutionJob) (any, error) {
	values, ok := job.Input["values"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing or invalid values parameter")
	}

	aggregateType, _ := job.Input["aggregate_type"].(string)
	if aggregateType == "" {
		aggregateType = "sum"
	}

	// 转换为数字
	numbers := make([]float64, 0, len(values))
	for _, v := range values {
		switch n := v.(type) {
		case float64:
			numbers = append(numbers, n)
		case int:
			numbers = append(numbers, float64(n))
		case int64:
			numbers = append(numbers, float64(n))
		}
	}

	if len(numbers) == 0 {
		return nil, fmt.Errorf("no numeric values provided")
	}

	switch aggregateType {
	case "sum":
		sum := 0.0
		for _, n := range numbers {
			sum += n
		}
		return sum, nil
	case "avg":
		sum := 0.0
		for _, n := range numbers {
			sum += n
		}
		return sum / float64(len(numbers)), nil
	case "min":
		min := numbers[0]
		for _, n := range numbers {
			if n < min {
				min = n
			}
		}
		return min, nil
	case "max":
		max := numbers[0]
		for _, n := range numbers {
			if n > max {
				max = n
			}
		}
		return max, nil
	case "count":
		return len(numbers), nil
	default:
		return nil, fmt.Errorf("unsupported aggregate type: %s", aggregateType)
	}
}

// computeValidate 数据验证
func (e *ComputeExecutor) computeValidate(job *execution.ExecutionJob) (any, error) {
	data, ok := job.Input["data"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid data parameter")
	}

	validationType, _ := job.Input["validation_type"].(string)
	if validationType == "" {
		return nil, fmt.Errorf("missing validation_type parameter")
	}

	result := map[string]any{
		"valid":  false,
		"reason": "",
	}

	switch validationType {
	case "json":
		var js any
		if err := json.Unmarshal([]byte(data), &js); err != nil {
			result["reason"] = "invalid JSON: " + err.Error()
		} else {
			result["valid"] = true
		}
	case "not_empty":
		if strings.TrimSpace(data) != "" {
			result["valid"] = true
		} else {
			result["reason"] = "data is empty"
		}
	case "length":
		minLen, _ := job.Input["min_length"].(float64)
		maxLen, _ := job.Input["max_length"].(float64)
		if maxLen == 0 {
			maxLen = math.MaxFloat64
		}
		dataLen := float64(len(data))
		if dataLen >= minLen && dataLen <= maxLen {
			result["valid"] = true
		} else {
			result["reason"] = fmt.Sprintf("length %d not in range [%.0f, %.0f]", len(data), minLen, maxLen)
		}
	default:
		return nil, fmt.Errorf("unsupported validation type: %s", validationType)
	}

	return result, nil
}
