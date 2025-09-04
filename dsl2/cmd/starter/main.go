package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	yaml "github.com/goccy/go-yaml"

	dsl "github.com/temporalio/samples-go/dsl2"
	"go.temporal.io/sdk/client"
)

func main() {
	// ----- CLI flags -----
	var (
		yamlPath  string
		hostport  string
		namespace string
		taskQueue string
		wfid      string
		timeout   time.Duration
	)
	flag.StringVar(&yamlPath, "f", "", "Path to workflow YAML (required)")
	flag.StringVar(&yamlPath, "file", "", "Path to workflow YAML (required)") // alias
	flag.StringVar(&hostport, "host", envOr("TEMPORAL_HOSTPORT", "localhost:7233"), "Temporal Host:Port")
	flag.StringVar(&namespace, "ns", envOr("TEMPORAL_NAMESPACE", "default"), "Temporal Namespace")
	flag.StringVar(&taskQueue, "q", "", "Override task queue (optional, otherwise use YAML.taskQueue or 'demo')")
	flag.StringVar(&wfid, "id", "", "Workflow ID (optional, default auto-generate)")
	flag.DurationVar(&timeout, "timeout", 2*time.Minute, "Starter context timeout")
	flag.Parse()

	if yamlPath == "" {
		yamlPath = "workflow.yaml"
	}

	// ----- Load YAML -> Workflow -----
	wf, err := loadWorkflowFromYAML(yamlPath)
	if err != nil {
		log.Fatalf("load yaml: %v", err)
	}

	// 允许通过 CLI 覆盖 YAML 内的 taskQueue
	if taskQueue != "" {
		wf.TaskQueue = taskQueue
	}
	if wf.TaskQueue == "" {
		wf.TaskQueue = "demo"
	}

	// ----- Connect Temporal -----
	c, err := client.Dial(client.Options{
		HostPort:  hostport,
		Namespace: namespace,
	})
	if err != nil {
		log.Fatalf("client.Dial: %v", err)
	}
	defer c.Close()

	// ----- Start Workflow -----
	if wfid == "" {
		wfid = fmt.Sprintf("dsl-%d", time.Now().UnixNano())
	}
	opts := client.StartWorkflowOptions{
		ID:        wfid,
		TaskQueue: wf.TaskQueue,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	fmt.Printf("Starting Workflow: %+v\n", wf)
	run, err := c.ExecuteWorkflow(ctx, opts, dsl.SimpleDSLWorkflow, wf)
	if err != nil {
		log.Fatalf("start workflow: %v", err)
	}
	log.Printf("Started Workflow: WorkflowID=%s RunID=%s (taskQueue=%s)", run.GetID(), run.GetRunID(), wf.TaskQueue)

	// ----- Wait result & pretty print bindings -----
	var out map[string]any
	if err := run.Get(ctx, &out); err != nil {
		log.Fatalf("get result: %v", err)
	}
	bs, _ := json.MarshalIndent(out, "", "  ")
	log.Printf("Result bindings:\n%s", string(bs))
}

func loadWorkflowFromYAML(path string) (dsl.Workflow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return dsl.Workflow{}, fmt.Errorf("read file: %w", err)
	}
	var wf dsl.Workflow
	// 使用 sigs.k8s.io/yaml 以支持结构体上的 json 标签
	if err := yaml.Unmarshal(b, &wf); err != nil {
		return dsl.Workflow{}, fmt.Errorf("unmarshal yaml: %w", err)
	}
	fmt.Printf("Loaded Workflow from %s: %+v\n", path, wf)
	return wf, nil
}

// envOr returns env var value if present, otherwise fallback.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
