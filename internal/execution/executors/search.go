// Package executors 提供任务执行器实现
package executors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/execution"
)

// SearchExecutor 搜索执行器
type SearchExecutor struct {
	*execution.BaseExecutor
	searchPaths []string // 搜索路径
}

// NewSearchExecutor 创建搜索执行器
func NewSearchExecutor(searchPaths []string) *SearchExecutor {
	return &SearchExecutor{
		BaseExecutor: execution.NewBaseExecutor("search", "1.0.0", []execution.JobType{execution.JobTypeSearch}),
		searchPaths:  searchPaths,
	}
}

// CanExecute 检查是否能执行
func (e *SearchExecutor) CanExecute(job *execution.ExecutionJob) bool {
	if !e.BaseExecutor.CanExecute(job) {
		return false
	}
	// 检查必要参数
	_, hasQuery := job.Input["query"]
	return hasQuery
}

// EstimateResources 估算资源
func (e *SearchExecutor) EstimateResources(job *execution.ExecutionJob) (*execution.ResourceEstimate, error) {
	return &execution.ResourceEstimate{
		CPUTimeMs:   500,
		MemoryBytes: 32 * 1024 * 1024, // 32MB
		DurationSec: 10,
	}, nil
}

// Execute 执行搜索
func (e *SearchExecutor) Execute(ctx context.Context, job *execution.ExecutionJob) (*execution.ExecutionResult, error) {
	query, ok := job.Input["query"].(string)
	if !ok || query == "" {
		return execution.NewErrorResult("missing or invalid query parameter"), nil
	}

	searchType, _ := job.Input["search_type"].(string)
	if searchType == "" {
		searchType = "file" // 默认文件搜索
	}

	var results []SearchResult
	var err error

	switch searchType {
	case "file":
		results, err = e.searchFiles(ctx, job, query)
	case "content":
		results, err = e.searchContent(ctx, job, query)
	default:
		return execution.NewErrorResult(fmt.Sprintf("unsupported search type: %s", searchType)), nil
	}

	if err != nil {
		return execution.NewErrorResult(err.Error()), nil
	}

	return execution.NewSuccessResult(map[string]any{
		"results": results,
		"count":   len(results),
		"query":   query,
	}, nil), nil
}

// SearchResult 搜索结果
type SearchResult struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Type    string `json:"type"` // file/dir
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
	Match   string `json:"match,omitempty"` // 匹配的内容
}

// searchFiles 搜索文件名
func (e *SearchExecutor) searchFiles(ctx context.Context, job *execution.ExecutionJob, query string) ([]SearchResult, error) {
	results := make([]SearchResult, 0)
	query = strings.ToLower(query)

	maxResults := 100
	if mr, ok := job.Input["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	for _, searchPath := range e.searchPaths {
		err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			// 检查上下文是否取消
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err != nil {
				return nil // 跳过无法访问的文件
			}

			if len(results) >= maxResults {
				return filepath.SkipDir
			}

			name := strings.ToLower(info.Name())
			if strings.Contains(name, query) {
				itemType := "file"
				if info.IsDir() {
					itemType = "dir"
				}
				results = append(results, SearchResult{
					Path:    path,
					Name:    info.Name(),
					Type:    itemType,
					Size:    info.Size(),
					ModTime: info.ModTime().Unix(),
				})
			}
			return nil
		})

		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			continue // 继续搜索其他路径
		}
	}

	return results, nil
}

// searchContent 搜索文件内容
func (e *SearchExecutor) searchContent(ctx context.Context, job *execution.ExecutionJob, query string) ([]SearchResult, error) {
	results := make([]SearchResult, 0)
	query = strings.ToLower(query)

	maxResults := 50
	if mr, ok := job.Input["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	// 支持的文本文件扩展名
	textExts := map[string]bool{
		".txt": true, ".md": true, ".go": true, ".py": true,
		".js": true, ".ts": true, ".json": true, ".yaml": true,
		".yml": true, ".xml": true, ".html": true, ".css": true,
		".sh": true, ".bat": true, ".log": true,
	}

	for _, searchPath := range e.searchPaths {
		err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err != nil || info.IsDir() {
				return nil
			}

			if len(results) >= maxResults {
				return filepath.SkipDir
			}

			// 只搜索文本文件
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if !textExts[ext] {
				return nil
			}

			// 限制文件大小（最大1MB）
			if info.Size() > 1024*1024 {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			contentStr := strings.ToLower(string(content))
			if strings.Contains(contentStr, query) {
				// 提取匹配的行
				lines := strings.Split(string(content), "\n")
				var matchLine string
				for _, line := range lines {
					if strings.Contains(strings.ToLower(line), query) {
						matchLine = strings.TrimSpace(line)
						if len(matchLine) > 100 {
							matchLine = matchLine[:100] + "..."
						}
						break
					}
				}

				results = append(results, SearchResult{
					Path:    path,
					Name:    info.Name(),
					Type:    "file",
					Size:    info.Size(),
					ModTime: info.ModTime().Unix(),
					Match:   matchLine,
				})
			}
			return nil
		})

		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			continue
		}
	}

	return results, nil
}
