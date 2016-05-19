package actions

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/aybabtme/godotto/internal/ottoutil"
	"github.com/aybabtme/godotto/pkg/extra/do/cloud"
	"github.com/aybabtme/godotto/pkg/extra/do/cloud/actions"
	"github.com/robertkrimen/otto"
)

var q = otto.Value{}

func Apply(ctx context.Context, vm *otto.Otto, client cloud.Client) (otto.Value, error) {
	root, err := vm.Object(`({})`)
	if err != nil {
		return q, err
	}

	svc := actionSvc{
		ctx: ctx,
		svc: client.Actions(),
	}

	for _, applier := range []struct {
		Name   string
		Method func(otto.FunctionCall) otto.Value
	}{
		{"get", svc.get},
		{"list", svc.list},
	} {
		if err := root.Set(applier.Name, applier.Method); err != nil {
			return q, fmt.Errorf("preparing method %q, %v", applier.Name, err)
		}
	}

	return root.Value(), nil
}

type actionSvc struct {
	ctx context.Context
	svc actions.Client
}

func (svc *actionSvc) get(all otto.FunctionCall) otto.Value {
	vm := all.Otto
	arg := all.Argument(0)

	var aid int
	switch {
	case arg.IsNumber():
		aid = ottoutil.Int(vm, arg)
	case arg.IsObject():
		aid = ottoutil.Int(vm, ottoutil.GetObject(vm, arg.Object(), "id"))
	default:
		ottoutil.Throw(vm, "argument must be an Action or an ActionID")
	}

	a, err := svc.svc.Get(svc.ctx, aid)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}
	v, err := svc.actionToVM(vm, a)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}
	return v
}

func (svc *actionSvc) list(all otto.FunctionCall) otto.Value {

	vm := all.Otto
	var actions = make([]otto.Value, 0)
	actionc, errc := svc.svc.List(svc.ctx)
	for action := range actionc {
		v, err := svc.actionToVM(vm, action)
		if err != nil {
			ottoutil.Throw(vm, err.Error())
		}
		actions = append(actions, v)
	}
	if err := <-errc; err != nil {
		ottoutil.Throw(vm, err.Error())
	}

	v, err := vm.ToValue(actions)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}
	return v
}

func (svc *actionSvc) actionToVM(vm *otto.Otto, a actions.Action) (otto.Value, error) {
	d, _ := vm.Object(`({})`)
	g := a.Struct()
	for _, field := range []struct {
		name string
		v    interface{}
	}{
		{"id", int64(g.ID)},
		{"status", g.Status},
		{"type", g.Type},
		{"started_at", g.StartedAt.Format(time.RFC3339Nano)},
		{"completed_at", g.CompletedAt.Format(time.RFC3339Nano)},
		{"resource_id", int64(g.ResourceID)},
		{"resource_type", g.ResourceType},
		{"region_slug", g.RegionSlug},
	} {
		v, err := vm.ToValue(field.v)
		if err != nil {
			return q, fmt.Errorf("can't prepare field %q: %v", field.name, err)
		}
		if err := d.Set(field.name, v); err != nil {
			return q, fmt.Errorf("can't set field %q: %v", field.name, err)
		}
	}
	return d.Value(), nil
}
