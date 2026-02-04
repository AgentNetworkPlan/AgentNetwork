package executors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/execution"
)

func TestSearchExecutor(t *testing.T) {
	// Create temp directory with test files
	tempDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tempDir, "readme.md"), []byte("# Title\nThis is a test"), 0644)
	os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tempDir, "subdir", "nested.txt"), []byte("nested content"), 0644)

	executor := NewSearchExecutor([]string{tempDir})
	executor.Initialize()
	defer executor.Shutdown()

	// Test file name search
	t.Run("FileSearch", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeSearch, map[string]any{
			"query":       "test",
			"search_type": "file",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		count, ok := result.Output["count"].(int)
		if !ok || count == 0 {
			t.Error("Expected at least one result")
		}
	})

	// Test content search
	t.Run("ContentSearch", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeSearch, map[string]any{
			"query":       "world",
			"search_type": "content",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		count, ok := result.Output["count"].(int)
		if !ok || count == 0 {
			t.Error("Expected at least one result")
		}
	})

	// Test missing query
	t.Run("MissingQuery", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeSearch, map[string]any{})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.Success {
			t.Error("Expected failure for missing query")
		}
	})
}

func TestSearchExecutorCanExecute(t *testing.T) {
	executor := NewSearchExecutor([]string{})

	// Valid job
	job := execution.NewExecutionJob("task1", execution.JobTypeSearch, map[string]any{
		"query": "test",
	})
	if !executor.CanExecute(job) {
		t.Error("Should be able to execute search job with query")
	}

	// Missing query
	job2 := execution.NewExecutionJob("task2", execution.JobTypeSearch, map[string]any{})
	if executor.CanExecute(job2) {
		t.Error("Should not execute without query")
	}

	// Wrong type
	job3 := execution.NewExecutionJob("task3", execution.JobTypeCompute, map[string]any{
		"query": "test",
	})
	if executor.CanExecute(job3) {
		t.Error("Should not execute compute job")
	}
}

func TestComputeExecutor(t *testing.T) {
	executor := NewComputeExecutor()
	executor.Initialize()
	defer executor.Shutdown()

	// Test hash
	t.Run("Hash", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeCompute, map[string]any{
			"operation": "hash",
			"data":      "hello",
			"algorithm": "sha256",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		hash, ok := result.Output["result"].(string)
		if !ok || hash == "" {
			t.Error("Expected hash result")
		}
	})

	// Test encode
	t.Run("Encode", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeCompute, map[string]any{
			"operation": "encode",
			"data":      "hello",
			"format":    "base64",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		encoded, ok := result.Output["result"].(string)
		if !ok || encoded != "aGVsbG8=" {
			t.Errorf("Expected 'aGVsbG8=', got '%s'", encoded)
		}
	})

	// Test decode
	t.Run("Decode", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeCompute, map[string]any{
			"operation": "decode",
			"data":      "aGVsbG8=",
			"format":    "base64",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		decoded, ok := result.Output["result"].(string)
		if !ok || decoded != "hello" {
			t.Errorf("Expected 'hello', got '%s'", decoded)
		}
	})

	// Test transform
	t.Run("Transform", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeCompute, map[string]any{
			"operation":      "transform",
			"data":           "Hello World",
			"transform_type": "uppercase",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		transformed, ok := result.Output["result"].(string)
		if !ok || transformed != "HELLO WORLD" {
			t.Errorf("Expected 'HELLO WORLD', got '%s'", transformed)
		}
	})

	// Test aggregate
	t.Run("Aggregate", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeCompute, map[string]any{
			"operation":      "aggregate",
			"values":         []any{1.0, 2.0, 3.0, 4.0, 5.0},
			"aggregate_type": "sum",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		sum, ok := result.Output["result"].(float64)
		if !ok || sum != 15.0 {
			t.Errorf("Expected 15.0, got %v", sum)
		}
	})

	// Test validate
	t.Run("Validate", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeCompute, map[string]any{
			"operation":       "validate",
			"data":            `{"key": "value"}`,
			"validation_type": "json",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		validResult, ok := result.Output["result"].(map[string]any)
		if !ok {
			t.Error("Expected map result")
		} else if valid, _ := validResult["valid"].(bool); !valid {
			t.Error("Expected valid JSON")
		}
	})
}

func TestLLMExecutor(t *testing.T) {
	executor := NewLLMExecutor()
	executor.Initialize()
	defer executor.Shutdown()

	// Register mock provider
	mockProvider := NewMockLLMProvider("mock")
	executor.RegisterProvider(mockProvider)

	// Test chat
	t.Run("Chat", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeLLM, map[string]any{
			"llm_type": "chat",
			"provider": "mock",
			"message":  "Hello",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}

		llmResult, ok := result.Output["result"].(map[string]any)
		if !ok {
			t.Error("Expected map result")
		} else if content, _ := llmResult["content"].(string); content == "" {
			t.Error("Expected content in response")
		}
	})

	// Test completion
	t.Run("Completion", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeLLM, map[string]any{
			"llm_type": "completion",
			"provider": "mock",
			"prompt":   "Once upon a time",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}
	})

	// Test analysis
	t.Run("Analysis", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeLLM, map[string]any{
			"llm_type":      "analysis",
			"provider":      "mock",
			"content":       "I love this product!",
			"analysis_type": "sentiment",
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}
	})

	// Test summary
	t.Run("Summary", func(t *testing.T) {
		job := execution.NewExecutionJob("task1", execution.JobTypeLLM, map[string]any{
			"llm_type":   "summary",
			"provider":   "mock",
			"content":    "This is a long text that needs to be summarized...",
			"max_length": 50.0,
		})

		result, err := executor.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Errorf("Expected success, got error: %s", result.Error)
		}
	})

	// Test no provider
	t.Run("NoProvider", func(t *testing.T) {
		executor2 := NewLLMExecutor()
		job := execution.NewExecutionJob("task1", execution.JobTypeLLM, map[string]any{
			"llm_type": "chat",
			"message":  "Hello",
		})

		result, err := executor2.Execute(context.Background(), job)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.Success {
			t.Error("Expected failure with no provider")
		}
	})
}

func TestBaseExecutor(t *testing.T) {
	base := NewBaseExecutor("test", "1.0.0", []execution.JobType{execution.JobTypeCompute})

	if base.Name() != "test" {
		t.Errorf("Expected 'test', got %s", base.Name())
	}
	if base.Version() != "1.0.0" {
		t.Errorf("Expected '1.0.0', got %s", base.Version())
	}

	types := base.SupportedTypes()
	if len(types) != 1 || types[0] != execution.JobTypeCompute {
		t.Error("Expected JobTypeCompute")
	}

	// Initialize
	if err := base.Initialize(); err != nil {
		t.Errorf("Initialize failed: %v", err)
	}
	if !base.IsInitialized() {
		t.Error("Should be initialized")
	}

	// Shutdown
	if err := base.Shutdown(); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	if base.IsInitialized() {
		t.Error("Should not be initialized after shutdown")
	}
}

func TestExecutionResult(t *testing.T) {
	// Success result
	success := NewSuccessResult(map[string]any{"key": "value"}, nil)
	if !success.Success {
		t.Error("Success result should have Success=true")
	}
	if success.Output["key"] != "value" {
		t.Error("Output should contain key")
	}

	// Error result
	errResult := NewErrorResult("test error")
	if errResult.Success {
		t.Error("Error result should have Success=false")
	}
	if errResult.Error != "test error" {
		t.Errorf("Expected 'test error', got %s", errResult.Error)
	}
}
