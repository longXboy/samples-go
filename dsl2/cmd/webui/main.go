package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
	"errors"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
	"gopkg.in/yaml.v3"
)

// DSL 结构体定义（从主模块复制）
type Workflow struct {
	Version     string         `yaml:"version,omitempty"`
	TaskQueue   string         `yaml:"taskQueue,omitempty"`
	Variables   map[string]any `yaml:"variables,omitempty"`
	Root        *Statement     `yaml:"root"`
	Retry       *RetryPolicy   `yaml:"retry,omitempty"`
	TimeoutSec  int            `yaml:"timeoutSec,omitempty"`
	Concurrency int            `yaml:"concurrency,omitempty"`
}

type Statement struct {
	ID       string              `yaml:"id,omitempty"`
	Activity *ActivityInvocation `yaml:"activity,omitempty"`
	Sequence *Sequence           `yaml:"sequence,omitempty"`
	Parallel *Parallel           `yaml:"parallel,omitempty"`
	Map      *Map                `yaml:"map,omitempty"`
	While    *While              `yaml:"while,omitempty"`
	If       *If                 `yaml:"if,omitempty"`
}

type Sequence struct {
	Elements []*Statement `yaml:"elements"`
}

type Parallel struct {
	Branches []*Statement `yaml:"branches"`
}

type Map struct {
	ItemsRef    string     `yaml:"itemsRef"`
	ItemVar     string     `yaml:"itemVar,omitempty"`
	Concurrency int        `yaml:"concurrency,omitempty"`
	Body        *Statement `yaml:"body"`
	CollectVar  string     `yaml:"collectVar,omitempty"`
	FailFast    bool       `yaml:"failFast,omitempty"`
}

type If struct {
	Cond Cond       `yaml:"cond"`
	Then *Statement `yaml:"then"`
	Else *Statement `yaml:"else,omitempty"`
}

type While struct {
	Cond         Cond       `yaml:"cond"`
	Body         *Statement `yaml:"body"`
	MaxIters     int        `yaml:"maxIters,omitempty"`
	SleepSeconds int        `yaml:"sleepSeconds,omitempty"`
}

type ActivityInvocation struct {
	Name   string   `yaml:"name"`
	Args   []Value  `yaml:"args,omitempty"`
	Result string   `yaml:"result,omitempty"`
	Opts   *ActOpts `yaml:"opts,omitempty"`
}

type ActOpts struct {
	StartToCloseSeconds    int          `yaml:"startToCloseSeconds,omitempty"`
	ScheduleToCloseSeconds int          `yaml:"scheduleToCloseSeconds,omitempty"`
	HeartbeatSeconds       int          `yaml:"heartbeatSeconds,omitempty"`
	Retry                  *RetryPolicy `yaml:"retry,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts        int     `yaml:"maxAttempts,omitempty"`
	InitialIntervalSec int     `yaml:"initialIntervalSec,omitempty"`
	MaxIntervalSec     int     `yaml:"maxIntervalSec,omitempty"`
	BackoffCoefficient float64 `yaml:"backoffCoefficient,omitempty"`
}

type Cond struct {
	Truthy *Value   `yaml:"truthy,omitempty"`
	Eq     *Compare `yaml:"eq,omitempty"`
	Ne     *Compare `yaml:"ne,omitempty"`
	Not    *Cond    `yaml:"not,omitempty"`
	Any    []Cond   `yaml:"any,omitempty"`
	All    []Cond   `yaml:"all,omitempty"`
}

type Compare struct {
	Left  Value `yaml:"left"`
	Right Value `yaml:"right"`
}

type Value struct {
	Ref   string   `yaml:"ref,omitempty"`
	Str   *string  `yaml:"str,omitempty"`
	Int   *int64   `yaml:"int,omitempty"`
	Float *float64 `yaml:"float,omitempty"`
	Bool  *bool    `yaml:"bool,omitempty"`
}

// 基本的验证函数
func (wf Workflow) validate() error {
	if wf.Root == nil {
		return errors.New("root statement is nil")
	}
	return wf.Root.validate()
}

func (s *Statement) validate() error {
	if s == nil {
		return errors.New("nil statement")
	}
	cnt := 0
	if s.Activity != nil {
		cnt++
	}
	if s.Sequence != nil {
		cnt++
	}
	if s.Parallel != nil {
		cnt++
	}
	if s.Map != nil {
		cnt++
	}
	if s.While != nil {
		cnt++
	}
	if s.If != nil {
		cnt++
	}
	if cnt != 1 {
		return fmt.Errorf("statement must have exactly one of activity/sequence/parallel/map/while/if")
	}
	
	// 基本验证
	if s.Activity != nil && s.Activity.Name == "" {
		return errors.New("activity name required")
	}
	if s.Sequence != nil {
		for _, e := range s.Sequence.Elements {
			if err := e.validate(); err != nil {
				return err
			}
		}
	}
	if s.Parallel != nil {
		for _, b := range s.Parallel.Branches {
			if err := b.validate(); err != nil {
				return err
			}
		}
	}
	if s.Map != nil {
		if s.Map.Body == nil {
			return errors.New("map body required")
		}
		if err := s.Map.Body.validate(); err != nil {
			return err
		}
		if s.Map.ItemsRef == "" {
			return errors.New("map itemsRef required")
		}
	}
	if s.While != nil {
		if s.While.Body == nil {
			return errors.New("while body required")
		}
		if err := s.While.Body.validate(); err != nil {
			return err
		}
	}
	if s.If != nil {
		if s.If.Then == nil {
			return errors.New("if then branch required")
		}
		if err := s.If.Then.validate(); err != nil {
			return err
		}
		if s.If.Else != nil {
			if err := s.If.Else.validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

// 简化的工作流函数（用于演示）
func SimpleDSLWorkflow(ctx workflow.Context, wf Workflow) (map[string]any, error) {
	// 这里是一个简化版本，仅用于演示和验证
	// 实际执行需要完整的 DSL 引擎
	return map[string]any{
		"status": "simulated",
		"message": "This is a web UI demo. Connect to real Temporal worker for full execution.",
	}, nil
}

type Server struct {
	temporalClient client.Client
}

type WorkflowRequest struct {
	YAML string `json:"yaml"`
}

type WorkflowResponse struct {
	Success    bool        `json:"success"`
	Error      string      `json:"error,omitempty"`
	Result     interface{} `json:"result,omitempty"`
	WorkflowID string      `json:"workflowId,omitempty"`
	RunID      string      `json:"runId,omitempty"`
}

type WorkflowStatus struct {
	WorkflowID string      `json:"workflowId"`
	RunID      string      `json:"runId"`
	Status     string      `json:"status"`
	Result     interface{} `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
}

func main() {
	// 尝试创建 Temporal 客户端，但如果失败也能继续运行（仅验证模式）
	var c client.Client
	var err error
	
	c, err = client.Dial(client.Options{})
	if err != nil {
		log.Printf("Warning: Unable to create Temporal client: %v. Running in validation-only mode.", err)
	}
	
	server := &Server{
		temporalClient: c,
	}

	// 静态文件服务
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	
	// 主页面
	http.HandleFunc("/", server.handleIndex)
	
	// API 路由
	http.HandleFunc("/api/workflow/execute", server.handleExecuteWorkflow)
	http.HandleFunc("/api/workflow/status", server.handleWorkflowStatus)
	http.HandleFunc("/api/workflow/list", server.handleListWorkflows)
	http.HandleFunc("/api/examples", server.handleExamples)

	fmt.Println("🚀 Starting DSL Workflow Web UI on http://localhost:8080")
	fmt.Println("📝 Features: YAML Editor, Workflow Validation, Execution, Examples")
	if c == nil {
		fmt.Println("⚠️  Running in validation-only mode (no Temporal connection)")
	} else {
		fmt.Println("✅ Connected to Temporal server")
	}
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DSL Workflow Visual Designer</title>
    <link rel="stylesheet" href="/static/visual-style.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
</head>
<body>
    <div class="app-container">
        <!-- 顶部工具栏 -->
        <header class="toolbar">
            <div class="toolbar-left">
                <h1><i class="fas fa-project-diagram"></i> DSL Workflow Designer</h1>
            </div>
            <div class="toolbar-center">
                <button id="validateBtn" class="btn btn-secondary">
                    <i class="fas fa-check-circle"></i> Validate
                </button>
                <button id="executeBtn" class="btn btn-primary">
                    <i class="fas fa-play"></i> Execute
                </button>
                <button id="saveBtn" class="btn btn-secondary">
                    <i class="fas fa-save"></i> Save
                </button>
            </div>
            <div class="toolbar-right">
                <select id="exampleSelect" class="form-select">
                    <option value="">Load Example...</option>
                </select>
            </div>
        </header>

        <div class="main-workspace">
            <!-- 左侧节点面板 -->
            <div class="node-palette">
                <div class="palette-section">
                    <h3><i class="fas fa-cube"></i> Basic Nodes</h3>
                    <div class="node-category">
                        <div class="palette-node" data-type="activity" draggable="true">
                            <i class="fas fa-cog"></i>
                            <span>Activity</span>
                        </div>
                        <div class="palette-node" data-type="parallel" draggable="true">
                            <i class="fas fa-code-branch"></i>
                            <span>Parallel</span>
                        </div>
                        <div class="palette-node" data-type="sequence" draggable="true">
                            <i class="fas fa-arrow-right"></i>
                            <span>Sequence</span>
                        </div>
                    </div>
                </div>
                
                <div class="palette-section">
                    <h3><i class="fas fa-magic"></i> Control Flow</h3>
                    <div class="node-category">
                        <div class="palette-node" data-type="if" draggable="true">
                            <i class="fas fa-question"></i>
                            <span>If/Else</span>
                        </div>
                        <div class="palette-node" data-type="while" draggable="true">
                            <i class="fas fa-sync"></i>
                            <span>While Loop</span>
                        </div>
                        <div class="palette-node" data-type="map" draggable="true">
                            <i class="fas fa-list"></i>
                            <span>Map</span>
                        </div>
                    </div>
                </div>

                <div class="palette-section">
                    <h3><i class="fas fa-tools"></i> Utilities</h3>
                    <div class="node-category">
                        <div class="palette-node" data-type="start" draggable="true">
                            <i class="fas fa-play-circle"></i>
                            <span>Start</span>
                        </div>
                        <div class="palette-node" data-type="end" draggable="true">
                            <i class="fas fa-stop-circle"></i>
                            <span>End</span>
                        </div>
                    </div>
                </div>
            </div>

            <!-- 中央工作区 -->
            <div class="workflow-canvas" id="workflowCanvas">
                <div class="canvas-grid"></div>
                <div class="canvas-content" id="canvasContent">
                    <!-- 拖拽的节点将出现在这里 -->
                </div>
                
                <!-- 画布右键菜单 -->
                <div id="contextMenu" class="context-menu">
                    <div class="menu-item" data-action="delete">
                        <i class="fas fa-trash"></i> Delete
                    </div>
                    <div class="menu-item" data-action="disconnect">
                        <i class="fas fa-unlink"></i> Disconnect
                    </div>
                    <div class="menu-item" data-action="copy">
                        <i class="fas fa-copy"></i> Copy
                    </div>
                    <div class="menu-item" data-action="edit">
                        <i class="fas fa-edit"></i> Edit
                    </div>
                </div>
            </div>

            <!-- 右侧属性面板 -->
            <div class="properties-panel" id="propertiesPanel">
                <div class="panel-header">
                    <h3><i class="fas fa-sliders-h"></i> Properties</h3>
                </div>
                <div class="panel-content" id="propertiesContent">
                    <div class="no-selection">
                        <i class="fas fa-mouse-pointer"></i>
                        <p>Select a node to edit its properties</p>
                    </div>
                </div>
            </div>
        </div>

        <!-- 底部状态栏和结果面板 -->
        <div class="bottom-panel">
            <div class="status-bar" id="statusBar">
                <span class="status-text">Ready</span>
                <div class="status-actions">
                    <button id="toggleResults" class="btn-small">
                        <i class="fas fa-terminal"></i> Results
                    </button>
                    <button id="toggleYaml" class="btn-small">
                        <i class="fas fa-code"></i> YAML
                    </button>
                </div>
            </div>
            
            <div class="results-container" id="resultsContainer" style="display: none;">
                <div class="results-tabs">
                    <button class="tab-btn active" data-tab="execution">Execution Results</button>
                    <button class="tab-btn" data-tab="yaml">Generated YAML</button>
                    <button class="tab-btn" data-tab="validation">Validation</button>
                </div>
                <div class="results-content">
                    <div class="tab-pane active" id="executionResults"></div>
                    <div class="tab-pane" id="yamlOutput">
                        <pre><code id="yamlCode"></code></pre>
                    </div>
                    <div class="tab-pane" id="validationResults"></div>
                </div>
            </div>
        </div>

        <!-- 节点编辑模态框 -->
        <div id="nodeEditModal" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h3 id="modalTitle">Edit Node</h3>
                    <button class="modal-close" id="modalClose">
                        <i class="fas fa-times"></i>
                    </button>
                </div>
                <div class="modal-body" id="modalBody">
                    <!-- 动态内容 -->
                </div>
                <div class="modal-footer">
                    <button id="modalCancel" class="btn btn-secondary">Cancel</button>
                    <button id="modalSave" class="btn btn-primary">Save</button>
                </div>
            </div>
        </div>
    </div>

    <!-- SVG 定义 -->
    <svg width="0" height="0" style="position: absolute;">
        <defs>
            <marker id="arrowhead" markerWidth="10" markerHeight="7" 
                    refX="0" refY="3.5" orient="auto">
                <polygon points="0 0, 10 3.5, 0 7" fill="#666" />
            </marker>
        </defs>
    </svg>

    <script src="/static/visual-app.js"></script>
</body>
</html>`

	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, nil)
}

func (s *Server) handleExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req WorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 解析 YAML
	var workflow Workflow
	if err := yaml.Unmarshal([]byte(req.YAML), &workflow); err != nil {
		respondJSON(w, WorkflowResponse{
			Success: false,
			Error:   fmt.Sprintf("YAML parsing error: %v", err),
		})
		return
	}

	// 验证工作流
	if err := workflow.validate(); err != nil {
		respondJSON(w, WorkflowResponse{
			Success: false,
			Error:   fmt.Sprintf("Workflow validation error: %v", err),
		})
		return
	}

	// 如果没有 Temporal 客户端，返回验证成功信息
	if s.temporalClient == nil {
		respondJSON(w, WorkflowResponse{
			Success:    true,
			WorkflowID: fmt.Sprintf("demo-%d", time.Now().UnixNano()),
			RunID:      "demo-run",
			Result: map[string]interface{}{
				"status":  "validated",
				"message": "Workflow YAML is valid. Connect to Temporal worker for execution.",
				"workflow": map[string]interface{}{
					"version":   workflow.Version,
					"taskQueue": workflow.TaskQueue,
					"variables": workflow.Variables,
				},
			},
		})
		return
	}

	// 尝试执行工作流
	workflowID := fmt.Sprintf("dsl-%d", time.Now().UnixNano())
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflow.TaskQueue,
	}

	we, err := s.temporalClient.ExecuteWorkflow(context.Background(), workflowOptions, SimpleDSLWorkflow, workflow)
	if err != nil {
		respondJSON(w, WorkflowResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to start workflow: %v", err),
		})
		return
	}

	// 等待结果
	var result map[string]interface{}
	err = we.Get(context.Background(), &result)
	
	response := WorkflowResponse{
		Success:    err == nil,
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
	}

	if err != nil {
		response.Error = err.Error()
	} else {
		response.Result = result
	}

	respondJSON(w, response)
}

func (s *Server) handleWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("id")
	if workflowID == "" {
		http.Error(w, "Missing workflow ID", http.StatusBadRequest)
		return
	}

	if s.temporalClient == nil {
		respondJSON(w, WorkflowStatus{
			WorkflowID: workflowID,
			Status:     "Demo Mode",
			Result:     map[string]interface{}{"message": "No Temporal connection available"},
		})
		return
	}

	// 这里可以实现真正的状态查询
	respondJSON(w, WorkflowStatus{
		WorkflowID: workflowID,
		Status:     "Unknown",
		Result:     map[string]interface{}{"message": "Status query not implemented in demo"},
	})
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	// 返回空列表（演示）
	respondJSON(w, []interface{}{})
}

func (s *Server) handleExamples(w http.ResponseWriter, r *http.Request) {
	examples := map[string]string{
		"Basic Parallel": `version: "1.0"
taskQueue: "demo"
timeoutSec: 30
variables:
  x: 1
  y: 2
root:
  sequence:
    elements:
      - parallel:
          branches:
            - activity:
                name: "DoA"
                args: [{ ref: "x" }]
                result: "a"
            - activity:
                name: "DoB"
                args: [{ ref: "y" }]
                result: "b"
      - activity:
          name: "DoC"
          args: [{ ref: "a" }, { ref: "b" }]
          result: "c"`,

		"Map with Collection": `version: "1.0"
taskQueue: "demo"
timeoutSec: 30
variables:
  urls: ["https://a", "https://b", "https://c"]
root:
  map:
    itemsRef: "urls"
    itemVar: "url"
    concurrency: 3
    collectVar: "pages"
    body:
      activity:
        name: "Fetch"
        args: [{ ref: "url" }]
        result: "page"`,

		"Conditional Branch": `version: "1.0"
taskQueue: "demo"
timeoutSec: 30
variables:
  x: 5
  testFlag: true
root:
  sequence:
    elements:
      - if:
          cond:
            eq:
              left: { ref: "x" }
              right: { int: 5 }
          then:
            activity:
              name: "DoA"
              args: [{ ref: "x" }]
              result: "result"
          else:
            activity:
              name: "DoB"
              args: [{ int: 0 }]
              result: "result"`,

		"While Loop": `version: "1.0"
taskQueue: "demo"
timeoutSec: 30
variables:
  approved: false
root:
  while:
    cond:
      not:
        truthy: { ref: "approved" }
    sleepSeconds: 1
    maxIters: 3
    body:
      activity:
        name: "MockApprove"
        result: "approved"`,

		"Complex Nested": `version: "1.0"
taskQueue: "demo"
timeoutSec: 30
variables:
  mode: "production"
  items: [1, 2, 3]
root:
  sequence:
    elements:
      - if:
          cond:
            eq:
              left: { ref: "mode" }
              right: { str: "production" }
          then:
            sequence:
              elements:
                - parallel:
                    branches:
                      - activity:
                          name: "ValidateInput"
                          result: "validated"
                      - activity:
                          name: "CheckPermissions"
                          result: "authorized"
                - map:
                    itemsRef: "items"
                    itemVar: "item"
                    collectVar: "results"
                    body:
                      activity:
                        name: "ProcessItem"
                        args: [{ ref: "item" }]
                        result: "processed"
          else:
            activity:
              name: "DevModeProcess"
              result: "dev_result"`,
	}

	respondJSON(w, examples)
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}