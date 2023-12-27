package main

import (
	"context"
	"fmt"

	"github.com/akali/swe"
)

type NilSerializable[T any] struct {
	value T
}

func (ns NilSerializable[any]) Serialize() any {
	return ns.value
}

func (ns NilSerializable[any]) Deserialize(v any) (swe.Serializable[any], error) {
	return ns, nil
}

type FunctionalTask struct {
	condition func(context.Context) swe.Condition
	execute   func(context.Context, swe.Context) (swe.Context, error)
}

func FT(condition func(context.Context) swe.Condition,
	execute func(context.Context, swe.Context) (swe.Context, error)) FunctionalTask {
	return FunctionalTask{condition, execute}
}

func (t FunctionalTask) Condition(ctx context.Context) swe.Condition {
	return t.condition(ctx)
}

func (t FunctionalTask) Execute(ctx context.Context, sweCtx swe.Context) (swe.Context, error) {
	return t.execute(ctx, sweCtx)
}

func (t FunctionalTask) Serializer() swe.Serializable[any] {
	return NilSerializable[any]{}
}

type AlwaysTrue struct{}

func AT(context.Context) swe.Condition {
	return AlwaysTrue{}
}

func (AlwaysTrue) Retry(context.Context, swe.Context) bool {
	return true
}

func (AlwaysTrue) Execute(context.Context, swe.Context) bool {
	return true
}

func (AlwaysTrue) Serializer() swe.Serializable[any] {
	return NilSerializable[any]{}
}

type Outcome struct {
	complete bool
	failure  bool
	more     []swe.TaskGraph
}

func NewCompleteOutcome() Outcome {
	return Outcome{
		complete: true,
		failure:  false,
		more:     nil,
	}
}

func NewFailureOutcome() Outcome {
	return Outcome{
		complete: false,
		failure:  true,
		more:     nil,
	}
}

func NewOutcomeWithMore(graphs []swe.TaskGraph) Outcome {
	return Outcome{
		complete: false,
		failure:  false,
		more:     graphs,
	}
}

func (o Outcome) More() []swe.TaskGraph {
	return o.more
}
func (o Outcome) Complete() bool {
	return o.complete
}
func (o Outcome) Failure() bool {
	return o.failure
}
func (o Outcome) Serializer() swe.Serializable[any] {
	return NilSerializable[any]{}
}

type TaskExecutionPolicy struct {
	onSuccess, onFailure swe.Outcome
}

func NewTaskExecutionPolicy(onSuccess, onFailure swe.Outcome) swe.TaskExecutionPolicy {
	return &TaskExecutionPolicy{onSuccess: onSuccess, onFailure: onFailure}
}

func (tep *TaskExecutionPolicy) OnSuccess() swe.Outcome {
	return tep.onSuccess
}
func (tep *TaskExecutionPolicy) OnFailure() swe.Outcome {
	return tep.onFailure
}
func (tep *TaskExecutionPolicy) Serializer() swe.Serializable[any] {
	return NilSerializable[any]{}
}

type SimpleTaskGraph struct {
	task swe.Task
	next swe.TaskExecutionPolicy
}

func (s *SimpleTaskGraph) Task() swe.Task {
	return s.task
}

func (s *SimpleTaskGraph) Next() swe.TaskExecutionPolicy {
	return s.next
}

func (s *SimpleTaskGraph) Serializer() swe.Serializable[any] {
	return NilSerializable[any]{}
}

type SimpleWorkflow struct {
	taskGraph swe.TaskGraph
	outcome   swe.Outcome
}

func NewSimpleWorkflow(taskGraph swe.TaskGraph) *SimpleWorkflow {
	return &SimpleWorkflow{taskGraph: taskGraph}
}

func (s *SimpleWorkflow) Graph(context.Context) swe.TaskGraph {
	return s.taskGraph
}

func (s *SimpleWorkflow) IsComplete(context.Context) bool {
	return s.outcome != nil && s.outcome.Complete()
}

func (s *SimpleWorkflow) IsFailed(context.Context) bool {
	return s.outcome != nil && s.outcome.Failure()
}

func (s *SimpleWorkflow) nextOnFailure(ctx context.Context, err error) (swe.Workflow, error) {
	tg := s.Graph(ctx)
	nextOutcome := tg.Next().OnFailure()
	nextTaskGraphs := nextOutcome.More()
	if len(nextTaskGraphs) == 0 {
		return &SimpleWorkflow{
			taskGraph: nil,
			outcome:   nextOutcome,
		}, nil
	}
	return &SimpleWorkflow{
		taskGraph: nextTaskGraphs[0],
		outcome:   nextOutcome,
	}, nil
}

func (s *SimpleWorkflow) nextOnSuccess(ctx context.Context, nxtCtx swe.Context) (swe.Workflow, error) {
	tg := s.Graph(ctx)
	nextOutcome := tg.Next().OnSuccess()
	nextTaskGraphs := nextOutcome.More()
	if len(nextTaskGraphs) == 0 {
		return &SimpleWorkflow{
			taskGraph: nil,
			outcome:   nextOutcome,
		}, nil
	}
	return &SimpleWorkflow{
		taskGraph: nextTaskGraphs[0],
		outcome:   nextOutcome,
	}, nil
}

func (s *SimpleWorkflow) Progress(ctx context.Context, sweCtx swe.Context) (swe.Workflow, error) {
	task := s.taskGraph.Task()
	if !task.Condition(ctx).Execute(ctx, sweCtx) {
		return nil, fmt.Errorf("condition to run task didn't met")
	}
	nextCtx, err := task.Execute(ctx, sweCtx)
	if err != nil {
		fmt.Println(err)
		return s.nextOnFailure(ctx, err)
	}
	return s.nextOnSuccess(ctx, nextCtx)
}

func (s *SimpleWorkflow) Serializer() swe.Serializable[any] {
	return NilSerializable[any]{}
}

type WorkflowBuilder struct {
	simpleGraph *SimpleTaskGraph
	ctx         context.Context
}

func NewWorkflowBuilder(ctx context.Context) *WorkflowBuilder {
	return &WorkflowBuilder{
		simpleGraph: &SimpleTaskGraph{
			task: nil,
			next: NewTaskExecutionPolicy(NewCompleteOutcome(), NewFailureOutcome()),
		},
		ctx: ctx}
}

func (wb *WorkflowBuilder) StartWith(task swe.Task) swe.WorkflowBuilder {
	wb.simpleGraph.task = task
	return wb
}

func (wb *WorkflowBuilder) Then(tep swe.TaskExecutionPolicy) swe.WorkflowBuilder {
	wb.simpleGraph.next = tep
	return wb
}

func (wb *WorkflowBuilder) Build() swe.Workflow {
	return NewSimpleWorkflow(wb.simpleGraph)
}

func NewSingletonWorkfow(task swe.Task, tpe swe.TaskExecutionPolicy) swe.Workflow {
	return NewSimpleWorkflow(&SimpleTaskGraph{
		task: task,
		next: tpe,
	})
}

func main() {

	wb := NewWorkflowBuilder(context.Background())

	printHello := FT(AT, func(context.Context, swe.Context) (swe.Context, error) {
		fmt.Println("Hello")
		return nil, nil
	})
	printWorld := FT(AT, func(context.Context, swe.Context) (swe.Context, error) {
		fmt.Println("World")
		return nil, nil
	})

	w := wb.StartWith(printHello).Then(
		NewTaskExecutionPolicy(
			NewOutcomeWithMore([]swe.TaskGraph{
				NewSingletonWorkfow(printWorld, NewTaskExecutionPolicy(NewCompleteOutcome(), NewFailureOutcome())).Graph(context.Background()),
			}),
			NewFailureOutcome())).Build()

	for !w.IsComplete(context.Background()) && !w.IsFailed(context.Background()) {
		var err error
		w, err = w.Progress(context.Background(), nil)
		if err != nil {
			fmt.Printf("Failed to execute: %v", err)
			break
		}
	}
	if w.IsComplete(context.Background()) {
		fmt.Println("Workflow complete!")
	} else if w.IsFailed(context.Background()) {
		fmt.Println("Workflow failed!")
	}
}
