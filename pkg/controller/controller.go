package controller

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/tools/record"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	applisters "k8s.io/client-go/listers/apps/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	informers "github.com/staugust/esoperator/pkg/client/informers/externalversions/augusto.cn/v1"
	listers "github.com/staugust/esoperator/pkg/client/listers/augusto.cn/v1"
	esscheme "github.com/staugust/esoperator/pkg/client/clientset/versioned/scheme"
	clientset "github.com/staugust/esoperator/pkg/client/clientset/versioned"
	esv1 "github.com/staugust/esoperator/pkg/apis/augusto.cn/v1"
	"os"
	"strconv"
)

const controllerAgentName = "es-controller"
const (
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	ErrResourceExists     = "ErrResourceExists"
	MessageResourceSynced = "Foo synced successfully"
	SuccessSynced         = "Synced"
)

var deployReplicas = int32(1)
var podTerminationGracePeriodSeconds = int64(30)
var podSecurityContextPrivileged = true
// Controller is the controller implementation for Foo resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	esclientset clientset.Interface
	
	deployLister applisters.DeploymentLister
	deploySynced cache.InformerSynced
	esLister     listers.EsClusterLister
	esSynced     cache.InformerSynced
	
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
	pInformer appsinformers.DeploymentInformer,
	esInformer informers.EsClusterInformer) *Controller {
	
	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	esscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	
	controller := &Controller{
		kubeclientset: kubeclientset,
		esclientset:   esclientset,
		deployLister:  pInformer.Lister(),
		deploySynced:  pInformer.Informer().HasSynced,
		esLister:      esInformer.Lister(),
		esSynced:      esInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EsClusters"),
		recorder:      recorder,
	}
	
	glog.Info("Setting up event handlers")
	// Set up an event handler for when Foo resources change
	esInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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
	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()
	
	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Foo controller")
	
	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploySynced, c.esSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	
	glog.Info("Starting workers")
	// Launch two workers to process Foo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	
	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")
	
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	
	if shutdown {
		return false
	}
	
	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)
	
	if err != nil {
		runtime.HandleError(err)
		return true
	}
	
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	
	// Get the Foo resource with this namespace/name
	escluster, err := c.esLister.EsClusters(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}
		
		return err
	}
	var deploys []*appsv1.Deployment
	for i := int32(1); i <= *escluster.Spec.Replicas; i++ {
		newdeploy := newDeploy(escluster, i)
		
		//TODO check each deployment
		curdeploy, err := c.deployLister.Deployments(escluster.Namespace).Get(newdeploy.Name)
		if errors.IsNotFound(err) {
			curdeploy, err = c.kubeclientset.AppsV1().Deployments(escluster.Namespace).Create(newdeploy)
		}
		if err != nil {
			//TODO maybe I should do more actions, not just break the loop and return
			return err
		}
		if !metav1.IsControlledBy(curdeploy, escluster) {
			msg := fmt.Sprintf(MessageResourceExists, curdeploy.Name)
			c.recorder.Event(escluster, corev1.EventTypeWarning, ErrResourceExists, msg)
			return fmt.Errorf(msg)
		}
		// pod's five status.phase: Pending, Running, Succeeded, Failed, Unknown
		if curdeploy.Status.UnavailableReplicas != 0 {
			//c.kubeclientset.CoreV1().Pods(escluster.Namespace).Delete(dpod.Name, metav1.NewDeleteOptions(30))
			//rpod, err = c.kubeclientset.CoreV1().Pods(escluster.Namespace).Create(dpod)
			curdeploy, err = c.kubeclientset.AppsV1().Deployments(escluster.Namespace).Update(newdeploy)
		}
		if err != nil {
			return err
		}
		deploys = append(deploys, curdeploy)
	}
	
	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	err = c.updateFooStatus(escluster, deploys)
	if err != nil {
		return err
	}
	
	c.recorder.Event(escluster, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateFooStatus(escluster *esv1.EsCluster, deploys []*appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	esCopy := escluster.DeepCopy()
	//TODO update pod status to escluster.PodsStatus
	esCopy.Status.PodsStatus = make([]esv1.EsInstanceStatus, 0)
	
	for _, deploy := range deploys {
		var ls string
		var sep = ""
		for key, val := range deploy.Spec.Selector.MatchLabels {
			ls = sep + key + "=" + val
			sep = ","
		}
		
		var pods, err = c.kubeclientset.CoreV1().Pods(escluster.Namespace).List(metav1.ListOptions{
			LabelSelector: ls,
		})
		if err != nil {
			fmt.Printf("Error: %v --> %s\n", err, deploy.Name)
			continue
		}
		fmt.Printf("Found %d pods for %s \n", len(pods.Items), deploy.Name)
		if len(pods.Items) != 1 {
			fmt.Printf("%s is not in a good state, so just jump to next deployment", deploy.Name)
			continue
		}
		pod := pods.Items[0]
		status := esv1.EsInstanceStatus{
			PodName:     deploy.Name,
			PodHostName: pod.Spec.Hostname,
			NodeName:    pod.Spec.NodeName,
			Status:      pod.Status,
		}
		esCopy.Status.PodsStatus = append(esCopy.Status.PodsStatus, status)
	}
	
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Foo resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.esclientset.AugustoV1().EsClusters(escluster.Namespace).Update(esCopy)
	return err
}

// enqueueFoo takes a Foo resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Foo.
func (c *Controller) enqueueFoo(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Foo, we should not do anything more
		// with it.
		if ownerRef.Kind != esv1.CRD_KIND {
			return
		}
		escluster, err := c.esLister.EsClusters(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of escluster '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}
		c.enqueueFoo(escluster)
		return
	}
}

func newDeploy(escluster *esv1.EsCluster, index int32) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "elastic-search",
		"controller": escluster.Name,
	}
	
	var envArr []corev1.EnvVar = make([]corev1.EnvVar, len(escluster.Spec.Env))
	copy(envArr, escluster.Spec.Env)
	envArr = append(envArr, corev1.EnvVar{
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	})
	
	envArr = append(envArr, corev1.EnvVar{
		Name: "node.name",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})
	
	var deploy *appsv1.Deployment = nil
	if index > 0 && index <= *escluster.Spec.Replicas {
		deploy = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      escluster.Name + "-" + strconv.Itoa(int(index)),
				Namespace: escluster.Namespace,
				Labels:    labels,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(escluster, schema.GroupVersionKind{
						Group:   esv1.SchemeGroupVersion.Group,
						Version: esv1.SchemeGroupVersion.Version,
						Kind:    esv1.CRD_KIND,
					}),
				},
			},
			
			//TODO write pod's spec
			Spec: appsv1.DeploymentSpec{
				Replicas: &deployReplicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      escluster.Name + "-" + strconv.Itoa(int(index)),
						Namespace: escluster.Namespace,
						Labels:    labels,
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(escluster, schema.GroupVersionKind{
								Group:   esv1.SchemeGroupVersion.Group,
								Version: esv1.SchemeGroupVersion.Version,
								Kind:    esv1.CRD_KIND,
							}),
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:      "elasticsearch-logging",
								Image:     escluster.Spec.EsImage,
								Env:       envArr,
								Resources: escluster.Spec.Resource,
								//corev1.ResourceRequirements{
								//	Limits: corev1.ResourceList{
								//		//TODO generate quantity from spec
								//		corev1.ResourceCPU: resource.MustParse("1500m"),
								//	},
								//	Requests: corev1.ResourceList{
								//		corev1.ResourceCPU: resource.MustParse("1500m"),
								//	},
								//},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 9200,
										Name:          "db",
										Protocol:      "TCP",
									},
									{
										ContainerPort: 9300,
										Name:          "transport",
										Protocol:      "TCP",
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "elastic-logging",
										MountPath: "/data",
									},
								},
							},
						},
						InitContainers: []corev1.Container{
							{
								Name: "elasticsearch-logging-init",
								Command: []string{
									"/sbin/sysctl",
									"-w",
									"vm.max_map_count=262144",
								},
								Image: "alpine:3.6",
								SecurityContext: &corev1.SecurityContext{
									Privileged: &podSecurityContextPrivileged,
								},
							},
						},
						Volumes: []corev1.Volume{
							corev1.Volume{
								Name: "elastic-logging",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: escluster.Spec.DataPath + string(os.PathSeparator) + escluster.Name + "-" + strconv.Itoa(int(index)),
									},
								},
							},
						},
						TerminationGracePeriodSeconds: &podTerminationGracePeriodSeconds,
					},
				},
			},
		}
	}
	
	return deploy
}
