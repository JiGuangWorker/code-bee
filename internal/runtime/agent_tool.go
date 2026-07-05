// 本文件实现 AgentTool，包装 agent.Runner 调用 AI 智能体。
//
// 核心流程:
// 1. 根据 tool.PromptTemplate 用 PromptBuilder 渲染 prompt
// 2. 调 runner.Run(ctx, kind, prompt) 执行智能体
// 3. 通过 ArtifactResolver 从结果文件加载结构化数据
// 4. 返回 StepResult（含 Status/Data/Output）
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// AgentTool 包装 agent.Runner，执行 AI 智能体任务。
type AgentTool struct {
	def     *schema.Tool
	runner  Runner
	builder *PromptBuilder
}

// NewAgentTool 创建一个 AgentTool 实例。
func NewAgentTool(def *schema.Tool, runner Runner, builder *PromptBuilder) (*AgentTool, error) {
	if def.Type != "agent" {
		return nil, fmt.Errorf("AgentTool: tool type must be agent, got %s", def.Type)
	}
	if def.PromptTemplate == "" {
		return nil, fmt.Errorf("AgentTool: prompt_template required for tool %s", def.Name)
	}
	if runner == nil {
		return nil, fmt.Errorf("AgentTool: runner is nil for tool %s", def.Name)
	}
	if builder == nil {
		return nil, fmt.Errorf("AgentTool: builder is nil for tool %s", def.Name)
	}

	return &AgentTool{def: def, runner: runner, builder: builder}, nil
}

// Definition 返回工具的 schema 定义。
func (t *AgentTool) Definition() *schema.Tool {
	return t.def
}

// Execute 执行智能体任务。
func (t *AgentTool) Execute(ctx context.Context, inv Invocation) (*StepResult, error) {
	// 1. 构建 prompt 数据
	data := t.buildPromptData(inv)

	// 2. 渲染 prompt
	prompt, err := t.builder.Build(t.def.PromptTemplate, data)
	if err != nil {
		return &StepResult{StageName: inv.StageName, Output: ""}, fmt.Errorf("AgentTool.Build prompt: %w", err)
	}

	// 3. 调用 runner
	kind := agent.TaskKind(mapTemplateToKind(t.def.PromptTemplate))
	runResult, err := t.runner.Run(ctx, kind, prompt)
	output := ""
	success := false
	if runResult != nil {
		output = runResult.Output
		success = runResult.Success
	}
	if err != nil {
		return &StepResult{
			StageName: inv.StageName,
			Success:   success,
			Output:    output,
		}, fmt.Errorf("AgentTool.Execute[%s]: %w", t.def.Name, err)
	}

	// 4. 加载结果文件
	result := &StepResult{
		StageName: inv.StageName,
		Success:   true,
		Output:    output,
	}

	if inv.EC != nil && inv.EC.Artifacts != nil && inv.ResultFilePath != "" {
		loaded, loadErr := inv.EC.Artifacts.LoadResult(inv.StageName, inv.ResultFilePath)
		if loadErr != nil {
			return result, fmt.Errorf("AgentTool.LoadResult[%s]: %w", t.def.Name, loadErr)
		}
		result.Data = loaded
		result.Status = extractStatus(loaded)
		result.Blocked = result.Status == "BLOCKED"
	}

	return result, nil
}

// buildPromptData 从 Invocation 构建 PromptBuilder 所需的 PromptData。
//
// 根据 tool.PromptTemplate 类型，从 ExecutionContext 查询所需的上游字段。
// stage name 约定:
// - issue-handling: 接单阶段
// - coding: 编码阶段
// - review: 审查阶段
// - issue-post: Issue 提交阶段
// - loop-judge: 价值评估阶段
func (t *AgentTool) buildPromptData(inv Invocation) PromptData {
	data := PromptData{
		WorkerID:       inv.Platform.WorkerID,
		IssueURL:       inv.Platform.IssueURL,
		PlatformName:   inv.Platform.PlatformName,
		PlatformGuide:  inv.Platform.PlatformGuide,
		ResultFilePath: inv.ResultFilePath,
	}

	if inv.Config != nil {
		data.Repo = inv.Config.Repo
		data.IssueNumber = inv.Config.IssueNumber
		data.DefaultAgent = inv.Config.DefaultAgent
		data.ReviewerAgent = inv.Config.ReviewerAgent
		data.IssuePostAgent = inv.Config.IssuePostAgent
	}

	if inv.Loop != nil {
		data.Round = inv.Loop.Round
		data.MaxRounds = inv.Loop.MaxIterations
		data.ConsecutiveUnknowns = inv.Loop.ConsecutiveUnknown
		data.ReviewerFeedback = inv.Loop.LastFeedback
	}

	// 根据 prompt 模板类型，从 EC 查询上游字段
	if inv.EC != nil {
		switch t.def.PromptTemplate {
		case promptCoding:
			// 从 issue-handling 拿 agent/summary/acceptance
			if v, ok := inv.EC.Lookup("issue-handling", "agent"); ok {
				if s, ok := v.(string); ok {
					data.Agent = s
				}
			}
			if v, ok := inv.EC.Lookup("issue-handling", "summary"); ok {
				if s, ok := v.(string); ok {
					data.IssueSummary = s
				}
			}
			if v, ok := inv.EC.Lookup("issue-handling", "acceptance"); ok {
				if s, ok := v.(string); ok {
					data.Acceptance = s
				}
			}

		case promptReview:
			// 从 issue-handling 拿 summary/acceptance
			if v, ok := inv.EC.Lookup("issue-handling", "summary"); ok {
				if s, ok := v.(string); ok {
					data.IssueSummary = s
				}
			}
			if v, ok := inv.EC.Lookup("issue-handling", "acceptance"); ok {
				if s, ok := v.(string); ok {
					data.Acceptance = s
				}
			}
			// 从 coding 拿 summary/evidence/acceptance_check
			if v, ok := inv.EC.Lookup("coding", "summary"); ok {
				if s, ok := v.(string); ok {
					data.CodingSummary = s
				}
			}
			if v, ok := inv.EC.Lookup("coding", "evidence"); ok {
				if s, ok := v.(string); ok {
					data.CodingEvidence = s
				}
			}
			if v, ok := inv.EC.Lookup("coding", "acceptance_check"); ok {
				if s, ok := v.(string); ok {
					data.CodingAcceptanceCheck = s
				}
			}

		case promptIssuePost:
			// source file path 从 input_from stage 的结果文件获取
			if inv.EC.Artifacts != nil {
				sourceStage, _ := getStringArg(inv.Args, "source_stage")
				if sourceStage == "" {
					// 默认从 input_from 字段推断
					sourceStage = ""
				}
				if sourceStage != "" {
					data.SourceFilePath = inv.EC.Artifacts.ResolveResultFile(sourceStage, inv.Args, 0)
				}
			}
			if v, ok := getStringArg(inv.Args, "purpose"); ok {
				data.Purpose = v
			}

		case promptLoopJudge:
			// history file path 从 artifacts 获取
			if inv.EC.Artifacts != nil {
				data.HistoryFilePath = inv.EC.Artifacts.LoopHistoryPath()
			}
		}
	}

	return data
}

// extractStatus 从结果 map 中提取 status 字段。
func extractStatus(data map[string]any) string {
	if data == nil {
		return ""
	}
	if v, ok := data["status"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getStringArg 从 args map 中获取字符串参数。
func getStringArg(args map[string]any, key string) (string, bool) {
	if args == nil {
		return "", false
	}
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
