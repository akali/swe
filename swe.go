package swe

import "context"

type Serializable[T any] interface {
	Serialize() T
	Deserialize(T) (Serializable[T], error)
}

type Task interface {
	Condition(context.Context) Condition
	Execute(context.Context, Context) (Context, error)

	Serializer() Serializable[any]
}

type Context interface {
	Get(context.Context, string) any
	Set(context.Context, string, any) error

	Serializer() Serializable[any]
}

type Condition interface {
	Retry(context.Context, Context) bool
	Execute(context.Context, Context) bool

	Serializer() Serializable[any]
}

type Workflow interface {
	Graph(context.Context) TaskGraph

	IsComplete(context.Context) bool
	IsFailed(context.Context) bool

	Progress(context.Context, Context) (Workflow, error)

	Serializer() Serializable[any]
}

type Outcome interface {
	More() []TaskGraph
	Complete() bool
	Failure() bool

	Serializer() Serializable[any]
}

type TaskExecutionPolicy interface {
	OnSuccess() Outcome
	OnFailure() Outcome

	Serializer() Serializable[any]
}

type TaskGraph interface {
	Task() Task
	Next() TaskExecutionPolicy

	Serializer() Serializable[any]
}

type WorkflowBuilder interface {
	StartWith(Task) WorkflowBuilder
	Then(TaskExecutionPolicy) WorkflowBuilder
	Build() Workflow
}
