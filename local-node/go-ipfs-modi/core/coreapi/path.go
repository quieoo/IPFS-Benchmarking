package coreapi

import (
	"context"
	"fmt"
	flatfs "github.com/ipfs/go-ds-flatfs"
	"github.com/ipfs/go-ipfs/namesys/resolve"
	gopath "path"
	"time"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	ipfspath "github.com/ipfs/go-path"
	"github.com/ipfs/go-path/resolver"
	uio "github.com/ipfs/go-unixfs/io"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	path "github.com/ipfs/interface-go-ipfs-core/path"
)

// ResolveNode resolves the path `p` using Unixfs resolver, gets and returns the
// resolved Node.
func (api *CoreAPI) ResolveNode(ctx context.Context, p path.Path) (ipld.Node, error) {
	rp, err := api.ResolvePath(ctx, p)

	if err != nil {
		return nil, err
	}
	flatfs.IOReport.CidStart(rp.Cid(),time.Now())

	node, err := api.dag.Get(ctx, rp.Cid())

	flatfs.IOReport.CidFinishResolve(rp.Cid(),time.Now())
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ResolvePath resolves the path `p` using Unixfs resolver, returns the
// resolved path.
func (api *CoreAPI) ResolvePath(ctx context.Context, p path.Path) (path.Resolved, error) {

	//fmt.Println(p.String())
	if _, ok := p.(path.Resolved); ok {
		//fmt.Println("path resolved")
		return p.(path.Resolved), nil
	}
	if err := p.IsValid(); err != nil {
		return nil, err
	}

	ipath := ipfspath.Path(p.String())
	//fmt.Println("ipath1 "+ipath)

	ipath, err := resolve.ResolveIPNS(ctx, api.namesys, ipath)
	//fmt.Println("ipath2 "+ipath)

	if err == resolve.ErrNoNamesys {
		return nil, coreiface.ErrOffline
	} else if err != nil {
		return nil, err
	}

	var resolveOnce resolver.ResolveOnce

	switch ipath.Segments()[0] {
	case "ipfs":
		resolveOnce = uio.ResolveUnixfsOnce
	case "ipld":
		resolveOnce = resolver.ResolveSingle
	default:
		return nil, fmt.Errorf("unsupported path namespace: %s", p.Namespace())
	}

	r := &resolver.Resolver{
		DAG:         api.dag,
		ResolveOnce: resolveOnce,
	}

	node, rest, err := r.ResolveToLastNode(ctx, ipath)
	if err != nil {
		return nil, err
	}

	root, err := cid.Parse(ipath.Segments()[1])
	if err != nil {
		return nil, err
	}

	return path.NewResolvedPath(ipath, node, root, gopath.Join(rest...)), nil
}
