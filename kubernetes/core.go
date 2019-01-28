package kubernetes

import (
	"context"
	"flag"
	"os/user"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/allegro/bigcache"
	"github.com/puppetlabs/wash/datastore"
	"github.com/puppetlabs/wash/log"
	"github.com/puppetlabs/wash/plugin"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// Loads the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type client struct {
	*k8s.Clientset
	cache   *bigcache.BigCache
	mux     sync.Mutex
	reqs    map[string]*datastore.StreamBuffer
	updated time.Time
	root    string
	groups  []string
}

// Defines how quickly we should allow checks for updated content. This has to be consistent
// across files and directories or we may not detect updates quickly enough, especially for files
// that previously were empty.
const validDuration = 100 * time.Millisecond

// Create a new kubernetes client.
func Create(name string) (plugin.DirProtocol, error) {
	me, err := user.Current()
	if err != nil {
		return nil, err
	}

	var kubeconfig *string
	if me.HomeDir != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(me.HomeDir, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}

	// create the clientset
	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// TODO: this should be a helper, or passed to Create.
	cacheconfig := bigcache.DefaultConfig(5 * time.Second)
	cacheconfig.CleanWindow = 100 * time.Millisecond
	cache, err := bigcache.NewBigCache(cacheconfig)
	if err != nil {
		return nil, err
	}

	groups := []string{"namespaces", "pods"}
	sort.Strings(groups)

	reqs := make(map[string]*datastore.StreamBuffer)
	return &client{clientset, cache, sync.Mutex{}, reqs, time.Now(), name, groups}, nil
}

// Find container by ID.
func (cli *client) Find(ctx context.Context, name string) (plugin.Node, error) {
	idx := sort.SearchStrings(cli.groups, name)
	if cli.groups[idx] == name {
		log.Debugf("Found group %v", name)
		return plugin.NewDir(&node{cli, name, nil}), nil
	}
	return nil, plugin.ENOENT
}

// List all running pods as files.
func (cli *client) List(ctx context.Context) ([]plugin.Node, error) {
	log.Debugf("Listing %v groups in /kubernetes", len(cli.groups))
	entries := make([]plugin.Node, len(cli.groups))
	for i, v := range cli.groups {
		entries[i] = plugin.NewDir(&node{cli, v, nil})
	}
	return entries, nil
}

// Name returns the root directory of the client.
func (cli *client) Name() string {
	return cli.root
}

// Attr returns attributes of the named resource.
func (cli *client) Attr(ctx context.Context) (*plugin.Attributes, error) {
	// Now that content updates are asynchronous, we can make directory mtime reflect when we get new content.
	latest := cli.updated
	for _, v := range cli.reqs {
		if updated := v.LastUpdate(); updated.After(latest) {
			latest = updated
		}
	}
	return &plugin.Attributes{Mtime: latest, Valid: validDuration}, nil
}

// Xattr returns a map of extended attributes.
func (cli *client) Xattr(ctx context.Context) (map[string][]byte, error) {
	return nil, plugin.ENOTSUP
}