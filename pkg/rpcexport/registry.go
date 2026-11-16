// Package rpcexport lets bbgo expose a named, typed set of its own
// methods as remotely invocable over gRPC, and lets a caller discover and
// call them. It is intentionally gRPC-shaped (JSON-encoded params/results,
// synchronous Invoke) rather than transport-agnostic, since gRPC is the
// only transport this is built for.
package rpcexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// ErrMethodNotFound is returned by Invoke when no method with the given
// name has been exported on this Registry.
var ErrMethodNotFound = errors.New("rpcexport: method not found")

// ErrDuplicateMethod is returned by Export when a method with the given
// name has already been exported on this Registry. Two independently
// written components can legitimately pick the same name; that's a
// normal error to surface, not a panic.
var ErrDuplicateMethod = errors.New("rpcexport: method already exported")

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

type exportedMethod struct {
	info   MethodInfo
	fn     reflect.Value
	inArgs []reflect.Type
}

// Registry holds the set of methods bbgo has exported for remote
// invocation.
//
// Export must only be called during setup, before the gRPC server starts
// accepting calls. Invoke only reads the methods map afterward, so
// concurrent Invoke calls are safe without a mutex as long as no Export
// call races with them.
type Registry struct {
	methods map[string]*exportedMethod
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{methods: make(map[string]*exportedMethod)}
}

// Export registers fn under name/desc so it becomes remotely invocable.
// mutating marks whether invoking it can change live state; this is
// surfaced via discovery and logged on every call, but does not change
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
// Export panics if fn is not a function; it is meant to be called during
// one-time setup, not with dynamic/untrusted input. It returns
// ErrDuplicateMethod if a method with this name has already been
// exported on this Registry.
func (r *Registry) Export(name, desc string, mutating bool, fn interface{}, paramNames ...string) error {
	if _, exists := r.methods[name]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicateMethod, name)
	}

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

	return nil
}

// Methods returns the discovery info for every method exported so far.
func (r *Registry) Methods() []MethodInfo {
	out := make([]MethodInfo, 0, len(r.methods))
	for _, m := range r.methods {
		out = append(out, m.info)
	}
	return out
}

type invokeResult struct {
	data json.RawMessage
	err  error
}

// Invoke looks up method, unmarshals params (a JSON array, positional)
// into its argument types, and calls it immediately. The call runs in its
// own goroutine so it can be raced against ctx's deadline; a panic inside
// the exported function is recovered and returned as an error instead of
// propagating. If ctx is done before the call returns, Invoke returns
// ctx.Err() and the call's eventual result (if any) is discarded.
func (r *Registry) Invoke(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	m, ok := r.methods[method]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrMethodNotFound, method)
	}

	args, err := unmarshalArgs(params, m.inArgs)
	if err != nil {
		return nil, err
	}

	resultCh := make(chan invokeResult, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				resultCh <- invokeResult{err: fmt.Errorf("rpcexport: panic in method %q: %v", method, rec)}
			}
		}()

		out := m.fn.Call(args)
		data, err := extractResult(out)
		resultCh <- invokeResult{data: data, err: err}
	}()

	select {
	case res := <-resultCh:
		return res.data, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("rpcexport: method %q did not return in time: %w", method, ctx.Err())
	}
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
