package main

import (
	"log"
	"os"

	dsl "github.com/temporalio/samples-go/dsl2"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	host := envOr("TEMPORAL_HOSTPORT", "localhost:7233")
	ns := envOr("TEMPORAL_NAMESPACE", "default")
	taskQueue := envOr("TASK_QUEUE", "demo")

	c, err := client.Dial(client.Options{
		HostPort:  host,
		Namespace: ns,
	})
	if err != nil {
		log.Fatalf("client.Dial: %v", err)
	}
	defer c.Close()

	w := worker.New(c, taskQueue, worker.Options{})

	// 注册 DSL 的 Workflow
	w.RegisterWorkflow(dsl.SimpleDSLWorkflow)

	// 注册示例 Activities
	a := &dsl.Activities{}
	w.RegisterActivity(a)

	log.Printf("Worker started (namespace=%s, host=%s, taskQueue=%s)", ns, host, taskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("worker run failed: %v", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
