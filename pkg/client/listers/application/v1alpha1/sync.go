// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// SyncLister helps list Syncs.
type SyncLister interface {
	// List lists all Syncs in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.Sync, err error)
	// Syncs returns an object that can list and get Syncs.
	Syncs(namespace string) SyncNamespaceLister
	SyncListerExpansion
}

// syncLister implements the SyncLister interface.
type syncLister struct {
	indexer cache.Indexer
}

// NewSyncLister returns a new SyncLister.
func NewSyncLister(indexer cache.Indexer) SyncLister {
	return &syncLister{indexer: indexer}
}

// List lists all Syncs in the indexer.
func (s *syncLister) List(selector labels.Selector) (ret []*v1alpha1.Sync, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Sync))
	})
	return ret, err
}

// Syncs returns an object that can list and get Syncs.
func (s *syncLister) Syncs(namespace string) SyncNamespaceLister {
	return syncNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// SyncNamespaceLister helps list and get Syncs.
type SyncNamespaceLister interface {
	// List lists all Syncs in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.Sync, err error)
	// Get retrieves the Sync from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.Sync, error)
	SyncNamespaceListerExpansion
}

// syncNamespaceLister implements the SyncNamespaceLister
// interface.
type syncNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Syncs in the indexer for a given namespace.
func (s syncNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.Sync, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Sync))
	})
	return ret, err
}

// Get retrieves the Sync from the indexer for a given namespace and name.
func (s syncNamespaceLister) Get(name string) (*v1alpha1.Sync, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("sync"), name)
	}
	return obj.(*v1alpha1.Sync), nil
}
