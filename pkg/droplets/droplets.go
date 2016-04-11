package droplets

import (
	"fmt"

	"github.com/aybabtme/godotto/internal/ottoutil"

	"github.com/digitalocean/godo"
	"github.com/robertkrimen/otto"
)

var q = otto.Value{}

func Apply(vm *otto.Otto, client *godo.Client) (otto.Value, error) {
	root, err := vm.Object(`({})`)
	if err != nil {
		return q, err
	}

	svc := dropletSvc{
		svc: client.Droplets,
	}

	for _, applier := range []struct {
		Name   string
		Method func(otto.FunctionCall) otto.Value
	}{
		{"list", svc.list},
		{"listByTag", svc.listByTag},
		{"get", svc.get},
		{"create", svc.create},
		{"createMultiple", svc.createMultiple},
		{"delete", svc.delete},
		{"deleteByTag", svc.deleteByTag},
		{"kernels", svc.kernels},
		{"snapshots", svc.snapshots},
		{"backups", svc.backups},
		{"actions", svc.actions},
		{"neighbors", svc.neighbors},
	} {
		if err := root.Set(applier.Name, applier.Method); err != nil {
			return q, fmt.Errorf("preparing method %q, %v", applier.Name, err)
		}
	}

	return root.Value(), nil
}

type dropletSvc struct {
	svc godo.DropletsService
}

func (svc *dropletSvc) create(all otto.FunctionCall) otto.Value {
	vm := all.Otto
	arg := all.Argument(0).Object()
	if arg == nil {
		ottoutil.Throw(vm, "argument must be a object")
	}

	imgArg := ottoutil.GetObject(vm, arg, "image").Object()
	if imgArg == nil {
		ottoutil.Throw(vm, "object must contain an 'image' field")
	}

	sshArgs := ottoutil.GetObject(vm, arg, "ssh_keys").Object()
	if sshArgs == nil {
		ottoutil.Throw(vm, "object must contain an 'ssh_keys' field")
	}

	opts := &godo.DropletCreateRequest{
		Name:   ottoutil.String(vm, ottoutil.GetObject(vm, arg, "name")),
		Region: ottoutil.String(vm, ottoutil.GetObject(vm, arg, "region")),
		Size:   ottoutil.String(vm, ottoutil.GetObject(vm, arg, "size")),
		Image: godo.DropletCreateImage{
			ID:   int(ottoutil.Int(vm, ottoutil.GetObject(vm, imgArg, "id"))),
			Slug: ottoutil.String(vm, ottoutil.GetObject(vm, imgArg, "slug")),
		},
		Backups:           ottoutil.Bool(vm, ottoutil.GetObject(vm, imgArg, "backups")),
		IPv6:              ottoutil.Bool(vm, ottoutil.GetObject(vm, imgArg, "ipv6")),
		PrivateNetworking: ottoutil.Bool(vm, ottoutil.GetObject(vm, imgArg, "private_networking")),
		UserData:          ottoutil.String(vm, ottoutil.GetObject(vm, arg, "size")),
	}

	for _, k := range sshArgs.Keys() {
		sshArg := ottoutil.GetObject(vm, sshArgs, k).Object()
		if sshArg == nil {
			ottoutil.Throw(vm, "'ssh_keys' field must be an object")
		}
		opts.SSHKeys = append(opts.SSHKeys, godo.DropletCreateSSHKey{
			ID:          int(ottoutil.Int(vm, ottoutil.GetObject(vm, sshArg, "id"))),
			Fingerprint: ottoutil.String(vm, ottoutil.GetObject(vm, sshArg, "fingerprint")),
		})
	}

	d, _, err := svc.svc.Create(opts)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}

	v, err := svc.dropletToVM(vm, *d)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}
	return v
}

func (svc *dropletSvc) get(all otto.FunctionCall) otto.Value {
	vm := all.Otto
	arg := all.Argument(0)

	var did int
	switch {
	case arg.IsNumber():
		did = ottoutil.Int(vm, arg)
	case arg.IsObject():
		did = ottoutil.Int(vm, ottoutil.GetObject(vm, arg.Object(), "id"))
	default:
		ottoutil.Throw(vm, "argument must be a Droplet or a DropletID")
	}

	d, _, err := svc.svc.Get(did)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}
	v, err := svc.dropletToVM(vm, *d)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}
	return v
}

func (svc *dropletSvc) delete(all otto.FunctionCall) otto.Value {
	vm := all.Otto
	arg := all.Argument(0)

	var did int
	switch {
	case arg.IsNumber():
		did = ottoutil.Int(vm, arg)
	case arg.IsObject():
		did = ottoutil.Int(vm, ottoutil.GetObject(vm, arg.Object(), "id"))
	default:
		ottoutil.Throw(vm, "argument must be a Droplet or a DropletID")
	}

	_, err := svc.svc.Delete(did)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}
	return q
}

func (svc *dropletSvc) list(all otto.FunctionCall) otto.Value {
	vm := all.Otto
	opt := &godo.ListOptions{Page: 1, PerPage: 200}

	var droplets = make([]otto.Value, 0)

	for {
		items, resp, err := svc.svc.List(opt)
		if err != nil {
			ottoutil.Throw(vm, err.Error())
		}

		for _, d := range items {
			v, err := svc.dropletToVM(vm, d)
			if err != nil {
				ottoutil.Throw(vm, err.Error())
			}
			droplets = append(droplets, v)
		}

		if resp.Links != nil && !resp.Links.IsLastPage() {
			opt.Page++
		} else {
			break
		}
	}

	v, err := vm.ToValue(droplets)
	if err != nil {
		ottoutil.Throw(vm, err.Error())
	}
	return v
}

// helpers

func (svc *dropletSvc) dropletToVM(vm *otto.Otto, g godo.Droplet) (otto.Value, error) {
	d, _ := vm.Object(`({})`)
	for _, field := range []struct {
		name string
		v    interface{}
	}{
		{"id", g.ID},
		{"name", g.Name},
		{"memory", g.Memory},
		{"vcpus", g.Vcpus},
		{"disk", g.Disk},
		{"region", g.Region},
		{"image", g.Image},
		{"size", g.Size},
		{"size_slug", g.SizeSlug},
		{"backup_ids", g.BackupIDs},
		{"snapshot_ids", g.SnapshotIDs},
		{"locked", g.Locked},
		{"status", g.Status},
		{"networks", g.Networks},
		{"created_at", g.Created},
		{"kernel", g.Kernel},
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

// not implemented

func (svc *dropletSvc) listByTag(all otto.FunctionCall) otto.Value {
	ottoutil.Throw(all.Otto, "not implemented!")
	return q
}
func (svc *dropletSvc) createMultiple(all otto.FunctionCall) otto.Value {
	ottoutil.Throw(all.Otto, "not implemented!")
	return q
}
func (svc *dropletSvc) deleteByTag(all otto.FunctionCall) otto.Value {
	ottoutil.Throw(all.Otto, "not implemented!")
	return q
}
func (svc *dropletSvc) kernels(all otto.FunctionCall) otto.Value {
	ottoutil.Throw(all.Otto, "not implemented!")
	return q
}
func (svc *dropletSvc) snapshots(all otto.FunctionCall) otto.Value {
	ottoutil.Throw(all.Otto, "not implemented!")
	return q
}
func (svc *dropletSvc) backups(all otto.FunctionCall) otto.Value {
	ottoutil.Throw(all.Otto, "not implemented!")
	return q
}
func (svc *dropletSvc) actions(all otto.FunctionCall) otto.Value {
	ottoutil.Throw(all.Otto, "not implemented!")
	return q
}
func (svc *dropletSvc) neighbors(all otto.FunctionCall) otto.Value {
	ottoutil.Throw(all.Otto, "not implemented!")
	return q
}
