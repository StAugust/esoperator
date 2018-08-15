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
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/kubernetes/scheme"
	"github.com/golang/glog"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	appslisters "k8s.io/client-go/listers/apps/v1"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	
	
	clientset "github.com/staugust/esoperator/pkg/client/clientset/versioned"
	esv1 "github.com/staugust/esoperator/pkg/apis/augusto.cn/v1"
	samplescheme "github.com/staugust/esoperator/pkg/client/clientset/versioned/scheme"
	informers "github.com/staugust/esoperator/pkg/client/informers/externalversions/augusto.cn/v1"
	listers "github.com/staugust/esoperator/pkg/client/listers/augusto.cn/v1"
	
)


// Controller is the controller implementation for Foo resources
type ESController struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// esclientset is a clientset for our own API group
	esclientset clientset.Interface
	
	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	esLister        listers.EsClusterLister
	esSynced        cache.InformerSynced
	
	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	esclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	fooInformer informers.EsClusterInformer) *ESController {
	
	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	
	controller := &ESController{
		kubeclientset:     kubeclientset,
		esclientset:   esclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		foosLister:        fooInformer.Lister(),
		foosSynced:        fooInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Foos"),
		recorder:          recorder,
	}
	
	glog.Info("Setting up event handlers")
	// Set up an event handler for when Foo resources change
	fooInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueFoo,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueFoo(new)
		},
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Foo resource will enqueue that Foo resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})
	
	return controller
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
