package dsl

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

/*
   =============== 顶层模型 ===============
*/

// Workflow 是整张编排图
type Workflow struct {
	Version    string         `yaml:"version,omitempty"`
	TaskQueue  string         `yaml:"taskQueue,omitempty"`
	Variables  map[string]any `yaml:"variables,omitempty"`  // 初始变量
	Root       *Statement     `yaml:"root"`                 // 入口
	Retry      *RetryPolicy   `yaml:"retry,omitempty"`      // 可选：全局默认重试
	TimeoutSec int            `yaml:"timeoutSec,omitempty"` // 可选：全局默认超时
	// Concurrency: 作为 Map 的默认并发窗口（可被 Map 节点覆盖）
	Concurrency int `yaml:"concurrency,omitempty"`
}

// Statement：一个节点，要么是 Activity，要么是组合（Sequence/Parallel/Map/While/If）
type Statement struct {
	ID       string              `yaml:"id,omitempty"` // 可选：便于日志/排障
	Activity *ActivityInvocation `yaml:"activity,omitempty"`
	Sequence *Sequence           `yaml:"sequence,omitempty"`
	Parallel *Parallel           `yaml:"parallel,omitempty"`
	Map      *Map                `yaml:"map,omitempty"`
	While    *While              `yaml:"while,omitempty"`
	If       *If                 `yaml:"if,omitempty"`
}

// 顺序
type Sequence struct {
	Elements []*Statement `yaml:"elements"`
}

// 并行
type Parallel struct {
	Branches []*Statement `yaml:"branches"`
}

// 集合并行（对 items 做并发执行 Body）
type Map struct {
	ItemsRef    string     `yaml:"itemsRef"`              // 变量名：[]any / []T
	ItemVar     string     `yaml:"itemVar,omitempty"`     // Body 中当前元素变量名，默认 "_item"
	Concurrency int        `yaml:"concurrency,omitempty"` // 并发窗口；0 则用 Workflow.Concurrency；<=0 视作 1
	Body        *Statement `yaml:"body"`
	CollectVar  string     `yaml:"collectVar,omitempty"` // 可选：收集 Body 产生的某些变量（见注释）
	FailFast    bool       `yaml:"failFast,omitempty"`
}

// 条件分支
type If struct {
	Cond Cond       `yaml:"cond"`           // 条件表达式
	Then *Statement `yaml:"then"`           // 条件为真时执行的语句
	Else *Statement `yaml:"else,omitempty"` // 可选：条件为假时执行的语句
}

// 条件循环
type While struct {
	Cond         Cond       `yaml:"cond"` // 条件只依赖变量
	Body         *Statement `yaml:"body"`
	MaxIters     int        `yaml:"maxIters,omitempty"`     // 安全上限（0 表示不限制）
	SleepSeconds int        `yaml:"sleepSeconds,omitempty"` // 每轮之间 Sleep，避免忙等
	// ContinueEvery int        `yaml:"continueEvery,omitempty"` // 可选：每 N 轮 ContinueAsNew（实际环境再打开）
}

// 调用 Activity
type ActivityInvocation struct {
	Name   string   `yaml:"name"`             // Activity 名
	Args   []Value  `yaml:"args,omitempty"`   // 入参（支持 ref/字面量）
	Result string   `yaml:"result,omitempty"` // Optional：把返回值写入变量
	Opts   *ActOpts `yaml:"opts,omitempty"`   // 节点级选项（超时/重试）
}

// 节点级 ActivityOptions / 重试策略
type ActOpts struct {
	StartToCloseSeconds    int          `yaml:"startToCloseSeconds,omitempty"`
	ScheduleToCloseSeconds int          `yaml:"scheduleToCloseSeconds,omitempty"`
	HeartbeatSeconds       int          `yaml:"heartbeatSeconds,omitempty"`
	Retry                  *RetryPolicy `yaml:"retry,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts        int     `yaml:"maxAttempts,omitempty"`        // 0: 使用 SDK 默认；1: 不重试
	InitialIntervalSec int     `yaml:"initialIntervalSec,omitempty"` // 初始重试间隔
	MaxIntervalSec     int     `yaml:"maxIntervalSec,omitempty"`
	BackoffCoefficient float64 `yaml:"backoffCoefficient,omitempty"` // 默认 2.0
}

// 条件（结构化，避免不确定解析）
type Cond struct {
	// truthy: 变量为 true / 非空字符串 / 非零数字 / 非空集合
	Truthy *Value `yaml:"truthy,omitempty"`
	// eq/ne: 左右值比较
	Eq *Compare `yaml:"eq,omitempty"`
	Ne *Compare `yaml:"ne,omitempty"`
	// NOT / ANY / ALL（简单组合）
	Not *Cond  `yaml:"not,omitempty"`
	Any []Cond `yaml:"any,omitempty"`
	All []Cond `yaml:"all,omitempty"`
}

type Compare struct {
	Left  Value `yaml:"left"`
	Right Value `yaml:"right"`
}

// Value：带类型的值或变量引用（二选一）
type Value struct {
	Ref   string   `yaml:"ref,omitempty"` // 引用变量，如 "foo"
	Str   *string  `yaml:"str,omitempty"`
	Int   *int64   `yaml:"int,omitempty"`
	Float *float64 `yaml:"float,omitempty"`
	Bool  *bool    `yaml:"bool,omitempty"`
	// 可按需扩展：Map、Array、JSON Raw 等
}

/*
   =============== 入口与执行 ===============
*/

// SimpleDSLWorkflow 是可直接注册到 Temporal 的 Workflow 函数
func SimpleDSLWorkflow(ctx workflow.Context, wf Workflow) (map[string]any, error) {
	logger := workflow.GetLogger(ctx)

	// 初始化变量快照（工作流内部使用）
	bindings := make(map[string]any, len(wf.Variables))
	for k, v := range wf.Variables {
		bindings[k] = v
	}

	// 全局 ActivityOptions（可被节点覆盖）
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: durationOrDefault(wf.TimeoutSec, 30*time.Second),
	}
	if wf.Retry != nil {
		ao.RetryPolicy = toRetryPolicy(wf.Retry)
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// 校验 DSL
	if err := wf.validate(); err != nil {
		logger.Error("DSL validation failed", "error", err)
		return nil, err
	}

	// 执行
	if err := wf.Root.execute(ctx, wf, bindings); err != nil {
		logger.Error("DSL workflow failed", "error", err)
		return nil, err
	}

	logger.Info("DSL workflow completed")
	return bindings, nil
}

/*
   =============== 执行实现（各节点） ===============
*/

func (s *Statement) execute(ctx workflow.Context, wf Workflow, bindings map[string]any) error {
	switch {
	case s.Activity != nil:
		return s.Activity.execute(ctx, wf, bindings)
	case s.Sequence != nil:
		return s.Sequence.execute(ctx, wf, bindings)
	case s.Parallel != nil:
		return s.Parallel.execute(ctx, wf, bindings)
	case s.Map != nil:
		return s.Map.execute(ctx, wf, bindings)
	case s.While != nil:
		return s.While.execute(ctx, wf, bindings)
	case s.If != nil:
		return s.If.execute(ctx, wf, bindings)
	default:
		return errors.New("invalid statement: empty")
	}
}

// ----- Activity -----

func (a ActivityInvocation) execute(ctx workflow.Context, wf Workflow, bindings map[string]any) error {
	fmt.Printf("Executing activity: %+v\n", a)
	// 局部 ActivityOptions
	if a.Opts != nil {
		ctx = workflow.WithActivityOptions(ctx, mergeActOpts(ctx, a.Opts))
	}

	// 解析参数
	args := make([]interface{}, 0, len(a.Args))
	for i := range a.Args {
		v, err := evalValue(a.Args[i], bindings)
		if err != nil {
			return fmt.Errorf("activity %s arg[%d] eval: %w", a.Name, i, err)
		}
		args = append(args, v)
	}

	// 执行
	var result any
	f := workflow.ExecuteActivity(ctx, a.Name, args...)
	if err := f.Get(ctx, &result); err != nil {
		return fmt.Errorf("activity %s failed: %w", a.Name, err)
	}

	// 写回变量
	if a.Result != "" {
		bindings[a.Result] = result
	}
	return nil
}

// ----- Sequence -----

func (s Sequence) execute(ctx workflow.Context, wf Workflow, bindings map[string]any) error {
	for _, st := range s.Elements {
		fmt.Printf("Executing sequence element: %+v\n", st)

		if st == nil {
			return errors.New("sequence has nil element")
		}
		if err := st.execute(ctx, wf, bindings); err != nil {
			return err
		}
		fmt.Printf("Finished executing sequence element: %+v\n", st)
	}
	return nil
}

// ----- Parallel -----
// 采用 copy-on-write；成功分支合并回主 bindings；合并冲突直接报错
func (p Parallel) execute(ctx workflow.Context, wf Workflow, bindings map[string]any) error {
	if len(p.Branches) == 0 {
		return nil
	}
	selector := workflow.NewSelector(ctx)
	logger := workflow.GetLogger(ctx)

	type mergeResult struct {
		local map[string]any
		err   error
	}

	fmt.Printf("Parallel: starting %d branches\n", len(p.Branches))

	// 存储所有结果
	results := make([]mergeResult, 0, len(p.Branches))
	completed := 0

	for i, st := range p.Branches {
		localBindings := cloneMap(bindings) // 浅拷贝：建议变量保持标量/小对象
		f := executeAsync(st, ctx, wf, localBindings)
		branchIndex := i // 捕获循环变量
		selector.AddFuture(f, func(f workflow.Future) {
			err := f.Get(ctx, nil)
			if err != nil {
				fmt.Printf("Parallel: branch %d failed with error: %v\n", branchIndex, err)
				results = append(results, mergeResult{nil, err})
			} else {
				fmt.Printf("Parallel: branch %d completed successfully\n", branchIndex)
				results = append(results, mergeResult{localBindings, nil})
			}
			completed++
		})
	}

	fmt.Printf("Parallel: waiting for %d branches to complete\n", len(p.Branches))
	
	// 等待所有分支完成
	for completed < len(p.Branches) {
		fmt.Printf("Parallel: waiting for completion (%d/%d done)\n", completed, len(p.Branches))
		selector.Select(ctx)
	}

	fmt.Printf("Parallel: all %d branches completed\n", len(p.Branches))

	// 检查是否有错误
	var firstErr error
	for _, r := range results {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
	}

	if firstErr != nil {
		logger.Error("parallel failed", "error", firstErr)
		return firstErr
	}

	fmt.Printf("Parallel: merging results from %d branches\n", len(results))
	// 使用保存的结果进行合并（检测冲突）
	for _, r := range results {
		if r.local == nil {
			continue
		}
		for k, v := range r.local {
			if _, exists := bindings[k]; exists && !reflect.DeepEqual(bindings[k], v) {
				return fmt.Errorf("variable %q written by multiple branches with different values", k)
			}
			bindings[k] = v
		}
	}

	fmt.Printf("Parallel: completed successfully\n")
	return nil
}

// ----- Map -----
// 并发窗口控制；Body 内可把结果写入 bindings，结束后可按需汇总（这里示例：将所有分支写入的 bindings[CollectVar_i] 收集到 CollectVar 数组）
func (m Map) execute(ctx workflow.Context, wf Workflow, bindings map[string]any) error {
	itemsAny, ok := bindings[m.ItemsRef]
	if !ok {
		return fmt.Errorf("map items var %q not found", m.ItemsRef)
	}
	items, ok := toSlice(itemsAny)
	if !ok {
		return fmt.Errorf("map items var %q is not a slice", m.ItemsRef)
	}

	fmt.Printf("Map: processing %d items\n", len(items))

	itemVar := m.ItemVar
	if itemVar == "" {
		itemVar = "_item"
	}

	// 并发窗口
	window := m.Concurrency
	if window <= 0 {
		if wf.Concurrency > 0 {
			window = wf.Concurrency
		} else {
			window = 1
		}
	}

	fmt.Printf("Map: using concurrency window of %d\n", window)

	type branchRes struct {
		local map[string]any
		err   error
		idx   int
	}

	childCtx, cancel := workflow.WithCancel(ctx)
	defer cancel() // 确保清理

	inflight := 0
	next := 0
	selector := workflow.NewSelector(ctx)
	
	// 存储所有结果
	allResults := make([]branchRes, 0, len(items))
	completed := 0

	emit := func(idx int, it any) {
		localBindings := cloneMap(bindings)
		localBindings[itemVar] = it
		f := executeAsync(m.Body, childCtx, wf, localBindings)
		inflight++
		fmt.Printf("Map: started processing item %d (inflight: %d)\n", idx, inflight)
		selector.AddFuture(f, func(f workflow.Future) {
			err := f.Get(childCtx, nil)
			if err != nil {
				fmt.Printf("Map: item %d failed with error: %v\n", idx, err)
			} else {
				fmt.Printf("Map: item %d completed successfully\n", idx)
			}
			allResults = append(allResults, branchRes{localBindings, err, idx})
			completed++
		})
	}

	// 先放初始窗口
	for next < len(items) && inflight < window {
		emit(next, items[next])
		next++
	}

	fmt.Printf("Map: started initial window, waiting for results\n")

	// 调度循环：简化版本，类似于 Parallel
	totalExpected := len(items)
	for completed < totalExpected {
		fmt.Printf("Map: waiting (completed: %d/%d, inflight: %d)\n", completed, totalExpected, inflight)
		selector.Select(ctx)
		
		// 检查新完成的任务
		if completed < len(allResults) {
			// 有新的结果
			lastResult := allResults[len(allResults)-1]
			inflight--
			
			if lastResult.err != nil {
				if m.FailFast {
					cancel()
					fmt.Printf("Map: failing fast due to error: %v\n", lastResult.err)
					return lastResult.err
				}
			}
			
			// 继续补位
			if next < len(items) && inflight < window {
				emit(next, items[next])
				next++
			}
		}
	}

	fmt.Printf("Map: all items processed, processing results\n")

	// 分离成功和失败的结果
	successResults := make([]branchRes, 0, len(items))
	collected := make([]any, len(items)) // 保持顺序
	var firstErr error

	// 识别被收集的变量名
	collectVars := make(map[string]bool)

	for _, r := range allResults {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
		} else {
			successResults = append(successResults, r)
			
			if m.CollectVar != "" {
				// 收集逻辑：按索引顺序收集
				var collectedValue any
				found := false
				
				// 1. 优先查找 CollectVar 本身
				if v, ok := r.local[m.CollectVar]; ok {
					collectedValue = v
					found = true
					collectVars[m.CollectVar] = true
				} else if v, ok := r.local[fmt.Sprintf("%s_%d", m.CollectVar, r.idx)]; ok {
					// 2. 查找 CollectVar_<index>
					collectedValue = v
					found = true
					collectVars[fmt.Sprintf("%s_%d", m.CollectVar, r.idx)] = true
				} else {
					// 3. 查找在当前迭代中新增的变量 (相对于输入 bindings)
					for k, v := range r.local {
						if k != itemVar && k != m.CollectVar && !strings.HasPrefix(k, m.CollectVar+"_") {
							if _, existsInOriginal := bindings[k]; !existsInOriginal {
								collectedValue = v
								found = true
								collectVars[k] = true
								fmt.Printf("Map: collecting variable %q = %v for item %d\n", k, v, r.idx)
								break
							}
						}
					}
				}
				
				if found {
					// 确保按索引顺序放置
					if r.idx < len(collected) {
						collected[r.idx] = collectedValue
					}
				}
			}
		}
	}

	if firstErr != nil && m.FailFast {
		return firstErr
	}
	
	// 合并成功分支的变量更改（检测冲突）
	for _, r := range successResults {
		for k, v := range r.local {
			// 跳过临时变量 itemVar、CollectVar 相关变量，以及被收集的变量
			if k == itemVar || 
			   (m.CollectVar != "" && (k == m.CollectVar || strings.HasPrefix(k, m.CollectVar+"_"))) ||
			   collectVars[k] {
				continue
			}
			if _, exists := bindings[k]; exists && !reflect.DeepEqual(bindings[k], v) {
				return fmt.Errorf("variable %q written by multiple map iterations with different values", k)
			}
			bindings[k] = v
		}
	}
	
	if m.CollectVar != "" {
		// 过滤掉 nil 值，保持收集到的值
		finalCollected := make([]any, 0, len(items))
		for _, v := range collected {
			if v != nil {
				finalCollected = append(finalCollected, v)
			}
		}
		bindings[m.CollectVar] = finalCollected
		fmt.Printf("Map: collected %d values to %s: %v\n", len(finalCollected), m.CollectVar, finalCollected)
	}

	fmt.Printf("Map: completed successfully with %d successful results\n", len(successResults))
	return firstErr
}

// ----- If -----

func (i If) execute(ctx workflow.Context, wf Workflow, bindings map[string]any) error {
	fmt.Printf("If: evaluating condition\n")
	
	// 评估条件
	ok, err := evalCond(i.Cond, bindings)
	if err != nil {
		return fmt.Errorf("if condition eval failed: %w", err)
	}
	
	if ok {
		fmt.Printf("If: condition is true, executing then branch\n")
		if i.Then != nil {
			return i.Then.execute(ctx, wf, bindings)
		}
	} else {
		fmt.Printf("If: condition is false, executing else branch\n")
		if i.Else != nil {
			return i.Else.execute(ctx, wf, bindings)
		}
	}
	
	fmt.Printf("If: completed\n")
	return nil
}

// ----- While -----

func (w While) execute(ctx workflow.Context, wf Workflow, bindings map[string]any) error {
	iter := 0
	for {
		ok, err := evalCond(w.Cond, bindings)
		if err != nil {
			return fmt.Errorf("while cond eval failed: %w", err)
		}
		if !ok {
			return nil
		}
		if w.MaxIters > 0 && iter >= w.MaxIters {
			return fmt.Errorf("while exceeded MaxIters=%d", w.MaxIters)
		}
		if err := w.Body.execute(ctx, wf, bindings); err != nil {
			return err
		}
		if w.SleepSeconds > 0 {
			_ = workflow.Sleep(ctx, time.Duration(w.SleepSeconds)*time.Second)
		}
		iter++

		// 如需分段历史，可在此根据 iter 或运行时指标触发 ContinueAsNew
		// return workflow.NewContinueAsNewError(ctx, SimpleDSLWorkflow, wfNext)
	}
}

/*
   =============== 校验 ===============
*/

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
		return fmt.Errorf("statement(id=%s) must have exactly one of activity/sequence/parallel/map/while/if", s.ID)
	}
	if s.Activity != nil {
		if s.Activity.Name == "" {
			return errors.New("activity name required")
		}
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

/*
   =============== 工具 & 评估器 ===============
*/

func executeAsync(st *Statement, ctx workflow.Context, wf Workflow, bindings map[string]any) workflow.Future {
	f, set := workflow.NewFuture(ctx)
	workflow.Go(ctx, func(ctx workflow.Context) {
		err := st.execute(ctx, wf, bindings)
		set.Set(nil, err)
	})
	return f
}

// 合并全局与节点级 AO
func mergeActOpts(ctx workflow.Context, o *ActOpts) workflow.ActivityOptions {
	parent := workflow.GetActivityOptions(ctx)
	ao := parent
	if o.StartToCloseSeconds > 0 {
		ao.StartToCloseTimeout = time.Duration(o.StartToCloseSeconds) * time.Second
	}
	if o.ScheduleToCloseSeconds > 0 {
		ao.ScheduleToCloseTimeout = time.Duration(o.ScheduleToCloseSeconds) * time.Second
	}
	if o.HeartbeatSeconds > 0 {
		ao.HeartbeatTimeout = time.Duration(o.HeartbeatSeconds) * time.Second
	}
	if o.Retry != nil {
		ao.RetryPolicy = toRetryPolicy(o.Retry)
	}
	return ao
}

func toRetryPolicy(r *RetryPolicy) *temporal.RetryPolicy {
	if r == nil {
		return nil
	}
	p := &temporal.RetryPolicy{}
	if r.MaxAttempts > 0 {
		p.MaximumAttempts = int32(r.MaxAttempts)
	}
	if r.InitialIntervalSec > 0 {
		p.InitialInterval = time.Duration(r.InitialIntervalSec) * time.Second
	}
	if r.MaxIntervalSec > 0 {
		p.MaximumInterval = time.Duration(r.MaxIntervalSec) * time.Second
	}
	if r.BackoffCoefficient > 0 {
		p.BackoffCoefficient = r.BackoffCoefficient
	} else {
		p.BackoffCoefficient = 2.0
	}
	return p
}

func durationOrDefault(sec int, d time.Duration) time.Duration {
	if sec > 0 {
		return time.Duration(sec) * time.Second
	}
	return d
}

func cloneMap(m map[string]any) map[string]any {
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v // 浅拷贝：建议仅存放标量或小对象
	}
	return cp
}

func toSlice(v any) ([]any, bool) {
	// 支持 []any、[]T
	switch arr := v.(type) {
	case []any:
		return arr, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		n := rv.Len()
		out := make([]any, n)
		for i := 0; i < n; i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, true
	}
	return nil, false
}

// 计算 Value（ref 或 字面量）
func evalValue(v Value, bindings map[string]any) (any, error) {
	if v.Ref != "" {
		val, ok := bindings[v.Ref]
		if !ok {
			return nil, fmt.Errorf("ref %q not found", v.Ref)
		}
		return val, nil
	}
	if v.Str != nil {
		return *v.Str, nil
	}
	if v.Int != nil {
		return *v.Int, nil
	}
	if v.Float != nil {
		return *v.Float, nil
	}
	if v.Bool != nil {
		return *v.Bool, nil
	}
	return nil, errors.New("empty value")
}

func evalCond(c Cond, bindings map[string]any) (bool, error) {
	// 组合逻辑优先
	if c.Not != nil {
		ok, err := evalCond(*c.Not, bindings)
		return !ok, err
	}
	if len(c.All) > 0 {
		for _, sub := range c.All {
			ok, err := evalCond(sub, bindings)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	}
	if len(c.Any) > 0 {
		anyMatch := false
		for _, sub := range c.Any {
			ok, err := evalCond(sub, bindings)
			if err != nil {
				return false, err
			}
			anyMatch = anyMatch || ok
		}
		return anyMatch, nil
	}

	// 原子谓词
	if c.Truthy != nil {
		v, err := evalValue(*c.Truthy, bindings)
		if err != nil {
			return false, err
		}
		return isTruthy(v), nil
	}
	if c.Eq != nil {
		l, err := evalValue(c.Eq.Left, bindings)
		if err != nil {
			return false, err
		}
		r, err := evalValue(c.Eq.Right, bindings)
		if err != nil {
			return false, err
		}
		return deepEqualNumberAware(l, r), nil
	}
	if c.Ne != nil {
		l, err := evalValue(c.Ne.Left, bindings)
		if err != nil {
			return false, err
		}
		r, err := evalValue(c.Ne.Right, bindings)
		if err != nil {
			return false, err
		}
		return !deepEqualNumberAware(l, r), nil
	}

	return false, errors.New("empty condition")
}

func isTruthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x != ""
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0 && !math.IsNaN(x)
	case []any:
		return len(x) > 0
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map:
			return rv.Len() > 0
		case reflect.Pointer, reflect.Interface:
			return !rv.IsNil()
		}
		return v != nil
	}
}

func deepEqualNumberAware(a, b any) bool {
	// 让 1 == 1.0 等价
	af, aIsNum := toFloat(a)
	bf, bIsNum := toFloat(b)
	if aIsNum && bIsNum {
		return (math.IsNaN(af) && math.IsNaN(bf)) || af == bf
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}
