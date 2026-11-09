// Package rpcexport lets a running strategy instance declare a named,
// typed set of its own methods as remotely invocable, and lets a caller
// discover and invoke them. It has no dependency on bbgo or grpc so it can
// be imported by both the strategy registration side (pkg/bbgo) and the
// transport side (pkg/grpc) without creating an import cycle.
package rpcexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// ErrMethodNotFound is returned by Enqueue when no method with the given
// name has been exported on this instance's Registry.
var ErrMethodNotFound = errors.New("rpcexport: method not found")

// ErrInboxFull is returned by Enqueue when the instance's pending-call
// inbox is at capacity, i.e. the strategy isn't draining Requests() fast
// enough.
var ErrInboxFull = errors.New("rpcexport: inbox full")

const defaultInboxSize = 32

// ParamSchema describes one positional parameter of an exported method, for
// discovery purposes. Name is either an explicit hint passed to Export, or
// a generic "argN" fallback, since Go reflection cannot recover real
// parameter names from a compiled function.
type ParamSchema struct {
	Name string
	Type string
}

// MethodInfo describes one exported method, for discovery purposes.
type MethodInfo struct {
	Name        string
	Description string
	Mutating    bool
	Params      []ParamSchema
}

// Call is one pending invocation, delivered to the strategy via
// Registry.Requests(). The strategy must eventually call Registry.Dispatch
// on it (exactly once) to produce a result.
type Call struct {
	Ctx    context.Context
	Method string
	Params json.RawMessage // JSON array, positional, matching the exported fn's parameters

	Result chan CallResult
}

// CallResult is the outcome of a dispatched Call.
type CallResult struct {
	Data json.RawMessage
	Err  error
}

type exportedMethod struct {
	info   MethodInfo
	fn     reflect.Value
	inArgs []reflect.Type
}

// Registry holds the set of methods one strategy instance has exported,
// plus the inbox of pending calls for that instance.
type Registry struct {
	instanceID string
	methods    map[string]*exportedMethod
	inbox      chan *Call
}

// NewRegistry creates a Registry for one strategy instance, identified by
// instanceID.
func NewRegistry(instanceID string) *Registry {
	return &Registry{
		instanceID: instanceID,
		methods:    make(map[string]*exportedMethod),
		inbox:      make(chan *Call, defaultInboxSize),
	}
}

// InstanceID returns the instance ID this registry was created for.
func (r *Registry) InstanceID() string {
	return r.instanceID
}

// Export registers fn under name/desc so it becomes remotely invocable.
// mutating marks whether invoking it can change live trading state; this
// is surfaced via discovery and logged on every call, but does not change
// how the call is executed.
//
// fn may take any number of parameters of JSON-unmarshalable types, and
// must return at most one non-error value plus an optional trailing error,
// e.g. func(symbol string) (Status, error), func(qty float64) error, or
// func() Status.
//
// paramNames are optional, positional hints used for the discovery schema;
// Go reflection cannot recover a compiled function's real parameter names,
// so params without a hint are named "arg0", "arg1", etc.
//
// Export panics if fn is not a function; it is meant to be called during a
// strategy's one-time setup (ExportRPC), not with dynamic/untrusted input.
func (r *Registry) Export(name, desc string, mutating bool, fn interface{}, paramNames ...string) {
	ft := reflect.TypeOf(fn)
	if ft == nil || ft.Kind() != reflect.Func {
		panic(fmt.Sprintf("rpcexport: Export(%q): fn must be a function, got %T", name, fn))
	}

	inArgs := make([]reflect.Type, ft.NumIn())
	params := make([]ParamSchema, ft.NumIn())
	for i := 0; i < ft.NumIn(); i++ {
		inArgs[i] = ft.In(i)

		pname := fmt.Sprintf("arg%d", i)
		if i < len(paramNames) {
			pname = paramNames[i]
		}

		params[i] = ParamSchema{Name: pname, Type: inArgs[i].String()}
	}

	r.methods[name] = &exportedMethod{
		info: MethodInfo{
			Name:        name,
			Description: desc,
			Mutating:    mutating,
			Params:      params,
		},
		fn:     reflect.ValueOf(fn),
		inArgs: inArgs,
	}
}

// Methods returns the discovery info for every method exported so far.
func (r *Registry) Methods() []MethodInfo {
	out := make([]MethodInfo, 0, len(r.methods))
	for _, m := range r.methods {
		out = append(out, m.info)
	}
	return out
}

// Requests returns the channel of pending calls. The owning strategy is
// expected to drain this from inside its own Run()/CrossRun() loop, at a
// time of its choosing, and call Dispatch on whatever it receives.
func (r *Registry) Requests() <-chan *Call {
	return r.inbox
}

// Enqueue is called by the transport (e.g. the gRPC handler) to submit an
// invocation. It validates that the method exists and that the instance's
// inbox has room, then returns the channel the caller should wait on for
// the result. It never blocks: if the inbox is full, it returns
// ErrInboxFull immediately rather than waiting for the strategy to catch
// up.
func (r *Registry) Enqueue(ctx context.Context, method string, params json.RawMessage) (<-chan CallResult, error) {
	if _, ok := r.methods[method]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrMethodNotFound, method)
	}

	call := &Call{
		Ctx:    ctx,
		Method: method,
		Params: params,
		Result: make(chan CallResult, 1),
	}

	select {
	case r.inbox <- call:
		return call.Result, nil
	default:
		return nil, ErrInboxFull
	}
}

// Dispatch executes a Call that the strategy pulled off Requests(), and
// sends exactly one CallResult on call.Result. A panic inside the exported
// function is recovered and turned into a CallResult error, so a bug in one
// export can't take down the strategy's own event loop.
func (r *Registry) Dispatch(call *Call) {
	defer func() {
		if rec := recover(); rec != nil {
			call.Result <- CallResult{Err: fmt.Errorf("rpcexport: panic in method %q: %v", call.Method, rec)}
		}
	}()

	m, ok := r.methods[call.Method]
	if !ok {
		call.Result <- CallResult{Err: fmt.Errorf("%w: %q", ErrMethodNotFound, call.Method)}
		return
	}

	args, err := unmarshalArgs(call.Params, m.inArgs)
	if err != nil {
		call.Result <- CallResult{Err: err}
		return
	}

	out := m.fn.Call(args)
	data, err := extractResult(out)
	call.Result <- CallResult{Data: data, Err: err}
}

func unmarshalArgs(params json.RawMessage, types []reflect.Type) ([]reflect.Value, error) {
	if len(types) == 0 {
		return nil, nil
	}

	var raw []json.RawMessage
	if len(params) > 0 {
		if err := json.Unmarshal(params, &raw); err != nil {
			return nil, fmt.Errorf("rpcexport: params must be a JSON array: %w", err)
		}
	}

	if len(raw) != len(types) {
		return nil, fmt.Errorf("rpcexport: expected %d param(s), got %d", len(types), len(raw))
	}

	args := make([]reflect.Value, len(types))
	for i, t := range types {
		pv := reflect.New(t)
		if err := json.Unmarshal(raw[i], pv.Interface()); err != nil {
			return nil, fmt.Errorf("rpcexport: param %d: %w", i, err)
		}
		args[i] = pv.Elem()
	}

	return args, nil
}

// extractResult maps a function's return values onto a (JSON data, error)
// pair: the first non-error return value becomes the JSON-marshaled data,
// and any error-typed return value (if non-nil) becomes the error.
func extractResult(out []reflect.Value) (json.RawMessage, error) {
	var err error
	var data interface{}
	hasData := false

	for _, v := range out {
		if v.Type().Implements(errorType) {
			if !v.IsNil() {
				err, _ = v.Interface().(error)
			}
			continue
		}

		if !hasData {
			data = v.Interface()
			hasData = true
		}
	}

	if !hasData {
		return nil, err
	}

	raw, mErr := json.Marshal(data)
	if mErr != nil {
		return nil, fmt.Errorf("rpcexport: marshal result: %w", mErr)
	}

	return raw, err
}
