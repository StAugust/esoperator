package controller

import (
	"fmt"
	"log"
	"sync"
	"time"
	
	"k8s.io/api/core/v1"
	"k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ESController watches the kubernetes api for changes to namespaces and
// creates a RoleBinding for that particular namespace.
type ESController struct {
	namespaceInformer cache.SharedIndexInformer
	kclient           *kubernetes.Clientset
}

// Run starts the process for listening for namespace changes and acting upon those changes.
func (c *ESController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	// When this function completes, mark the go function as done
	defer wg.Done()
	
	// Increment wait group as we're about to execute a go function
	wg.Add(1)
	
	// Execute go function
	go c.namespaceInformer.Run(stopCh)
	
	// Wait till we receive a stop signal
	<-stopCh
}

// NewNamespaceController creates a new NewNamespaceController
func NewESController(kclient *kubernetes.Clientset) *ESController {
	namespaceWatcher := &ESController{}
	
		
	// Create informer for watching Namespaces
	namespaceInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return kclient.CoreV1().Namespaces().List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kclient.CoreV1().Namespaces().Watch(options)
			},
		},
		&v1.Namespace{},
		3*time.Minute,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	
	namespaceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: showInfo("ADD"),
		UpdateFunc: func(oldObj, newObj interface{}) {
			fmt.Printf("OLDOBJ --> %v\n", oldObj)
			fmt.Printf("NEWOBJ --> %v\n", newObj)
		},
		DeleteFunc: showInfo("DELETE"),
	})
	namespaceWatcher.kclient = kclient
	namespaceWatcher.namespaceInformer = namespaceInformer
	
	return namespaceWatcher
}

func showInfo(prefix string) func(obj interface{}) {
	return func(obj interface{}) {
		fmt.Printf("%s --> %v\n", prefix, obj)
	}
}

func (c *ESController) createRoleBinding(obj interface{}) {
	namespaceObj := obj.(*v1.Namespace)
	namespaceName := namespaceObj.Name
	
	roleBinding := &v1beta1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("ad-kubernetes-%s", namespaceName),
			Namespace: namespaceName,
		},
		Subjects: []v1beta1.Subject{
			v1beta1.Subject{
				Kind: "Group",
				Name: fmt.Sprintf("ad-kubernetes-%s", namespaceName),
			},
		},
		RoleRef: v1beta1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "edit",
		},
	}
	
	_, err := c.kclient.RbacV1beta1().RoleBindings(namespaceName).Create(roleBinding)
	
	if err != nil {
		log.Println(fmt.Sprintf("Failed to create Role Binding: %s", err.Error()))
	} else {
		log.Println(fmt.Sprintf("Created AD RoleBinding for Namespace: %s", roleBinding.Name))
	}
}
