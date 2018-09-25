package manifest

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
	"github.com/multiformats/go-multihash"
	"github.com/ugorji/go/codec"
)

type layer struct {
	numChildren int
	size        uint64
}

type node struct {
	cid   *cid.Cid
	size  uint64
	links []*node
}

func (n node) String() string        { return n.cid.String() }
func (n node) Cid() *cid.Cid         { return n.cid }
func (n node) Size() (uint64, error) { return n.size, nil }
func (n node) Links() (links []*format.Link) {
	for _, l := range n.links {
		links = append(links, &format.Link{
			Size: l.size,
			Cid:  l.Cid(),
		})
	}
	return
}

// Not needed for manifest test:
func (n node) Loggable() map[string]interface{}                          { return nil }
func (n node) Copy() format.Node                                         { return nil }
func (n node) RawData() []byte                                           { return nil }
func (n node) Resolve(path []string) (interface{}, []string, error)      { return nil, nil, nil }
func (n node) ResolveLink(path []string) (*format.Link, []string, error) { return nil, nil, nil }
func (n node) Stat() (*format.NodeStat, error)                           { return nil, nil }
func (n node) Tree(path string, depth int) []string                      { return nil }

func NewGraph(layers []layer) (list []format.Node) {
	root := newNode(2 * kb)
	list = append(list, root)
	insert(root, layers, &list)
	return
}

func insert(n *node, layers []layer, list *[]format.Node) {
	if len(layers) > 0 {
		for i := 0; i < layers[0].numChildren; i++ {
			ch := newNode(layers[0].size)
			n.links = append(n.links, ch)
			*list = append(*list, ch)
			insert(ch, layers[1:], list)
		}
	}
}

// monotonic content counter for unique, consistent cids
var content = 0

func newNode(size uint64) *node {
	// Create a cid manually by specifying the 'prefix' parameters
	pref := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   multihash.SHA2_256,
		MhLength: -1, // default length
	}

	// And then feed it some data
	c, err := pref.Sum([]byte(strconv.Itoa(content)))
	if err != nil {
		panic(err)
	}

	content++
	return &node{
		cid:  c,
		size: size,
	}
}

type TestNodeGetter struct {
	Nodes []format.Node
}

func (ng TestNodeGetter) Get(_ context.Context, id *cid.Cid) (format.Node, error) {
	for _, node := range ng.Nodes {
		if id.Equals(node.Cid()) {
			return node, nil
		}
	}
	return nil, fmt.Errorf("cid not found: %s", id.String())
}

func TestNewManifest(t *testing.T) {
	g := NewGraph([]layer{
		{2, 4 * kb},
		{20, 5 * kb},
		{100, 256 * kb},
	})

	ng := TestNodeGetter{g}
	mf, err := NewManifest(context.Background(), ng, g[0])
	if err != nil {
		t.Error(err.Error())
	}

	verifyManifest(t, mf)

	buf := &bytes.Buffer{}
	enc := codec.NewEncoder(buf, &codec.CborHandle{})
	if err := enc.Encode(mf); err != nil {
		t.Fatal(err.Error())
	}

	size := uint64(0)
	for _, s := range mf.Sizes {
		size += s
	}

	t.Logf("manifest representing %d nodes and %s of content is %s as CBOR", len(mf.Nodes), fileSize(size), fileSize(buf.Len()))
}

func verifyManifest(t *testing.T, mf *Manifest) {
	if len(mf.Nodes) != len(mf.Sizes) {
		t.Errorf("nodes/sizes length mismatch. %d != %d", len(mf.Nodes), len(mf.Sizes))
	}
}

const (
	kb = 1000
	mb = kb * 1000
	gb = mb * 1000
	tb = gb * 1000
	pb = tb * 1000
)

type fileSize uint64

func (f fileSize) String() string {
	if f < kb {
		return fmt.Sprintf("%d bytes", f)
	} else if f < mb {
		return fmt.Sprintf("%fkb", float32(f)/float32(kb))
	} else if f < gb {
		return fmt.Sprintf("%fMB", float32(f)/float32(mb))
	} else if f < tb {
		return fmt.Sprintf("%fGb", float32(f)/float32(gb))
	} else if f < pb {
		return fmt.Sprintf("%fTb", float32(f)/float32(tb))
	}
	return "NaN"
}
