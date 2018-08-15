package k8sutil

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes"
	genclient "k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	
	"github.com/Sirupsen/logrus"
	"strconv"
	"k8s.io/client-go/tools/clientcmd"
	"github.com/staugust/esoperator/pkg/apis/augusto.cn"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
	"fmt"
	"k8s.io/apimachinery/pkg/util/errors"
)

type K8sutil struct {
	Config     *rest.Config
	CrdClient  genclient.Interface
	Kclient    kubernetes.Interface
	KubeExt    apiextensionsclient.Interface
	K8sVersion []int
	MasterHost string
}

func New(kubeCfgFile, masterHost string) (*K8sutil, error) {
	
	crdClient, kubeClient, kubeExt, k8sVersion, err := newKubeClient(kubeCfgFile)
	
	if err != nil {
		logrus.Fatalf("Could not init Kubernetes client! [%s]", err)
	}
	
	k := &K8sutil{
		Kclient:    kubeClient,
		MasterHost: masterHost,
		K8sVersion: k8sVersion,
		CrdClient:  crdClient,
		KubeExt:    kubeExt,
	}
	
	return k, nil
}

func (k *K8sutil) CreateKubernetesCustomResourceDefinition() error {
	
	crd, err := k.KubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Get(esoperator.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			crdObject := &apiextensionsv1beta1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: esoperator.Name,
				},
				Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
					Group:   esoperator.GroupName,
					Version: esoperator.Version,
					Scope:   apiextensionsv1beta1.NamespaceScoped,
					Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
						Plural:     esoperator.ResourcePlural,
						Kind:       esoperator.ResourceKind,
						ShortNames: []string{esoperator.ShortName},
					},
				},
			}
			
			_, err := k.KubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crdObject)
			if err != nil {
				panic(err)
			}
			logrus.Info("Created missing CRD...waiting for it to be established...")
			
			// wait for CRD being established
			err = wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
				createdCRD, err := k.KubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Get(esoperator.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				for _, cond := range createdCRD.Status.Conditions {
					switch cond.Type {
					case apiextensionsv1beta1.Established:
						if cond.Status == apiextensionsv1beta1.ConditionTrue {
							return true, nil
						}
					case apiextensionsv1beta1.NamesAccepted:
						if cond.Status == apiextensionsv1beta1.ConditionFalse {
							return false, fmt.Errorf("Name conflict: %v", cond.Reason)
						}
					}
				}
				return false, nil
			})
			
			if err != nil {
				deleteErr := k.KubeExt.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(esoperator.Name, nil)
				if deleteErr != nil {
					return errors.NewAggregate([]error{err, deleteErr})
				}
				return err
			}
			
			logrus.Info("CRD ready!")
		} else {
			panic(err)
		}
	} else {
		logrus.Infof("SKIPPING: already exists %#v\n", crd.ObjectMeta.Name)
	}
	
	return nil
}

func buildConfig(kubeCfgFile string) (*rest.Config, error) {
	if kubeCfgFile != "" {
		logrus.Infof("Using OutOfCluster k8s config with kubeConfigFile: %s", kubeCfgFile)
		config, err := clientcmd.BuildConfigFromFlags("", kubeCfgFile)
		if err != nil {
			panic(err.Error())
		}
		
		return config, nil
	}
	
	logrus.Info("Using InCluster k8s config")
	return rest.InClusterConfig()
}

func newKubeClient(kubeCfgFile string) (genclient.Interface, kubernetes.Interface, apiextensionsclient.Interface, []int, error) {
	
	// Create the client config. Use kubeconfig if given, otherwise assume in-cluster.
	Config, err := buildConfig(kubeCfgFile)
	if err != nil {
		panic(err)
	}
	
	// Create the kubernetes client
	clientSet, err := clientset.NewForConfig(Config)
	if err != nil {
		panic(err)
	}
	
	kubeClient, err := kubernetes.NewForConfig(Config)
	if err != nil {
		panic(err)
	}
	
	kubeExtCli, err := apiextensionsclient.NewForConfig(Config)
	if err != nil {
		panic(err)
	}
	
	version, err := kubeClient.ServerVersion()
	if err != nil {
		logrus.Error("Could not get version from api server:", err)
	}
	
	majorVer, _ := strconv.Atoi(version.Major)
	minorVer, _ := strconv.Atoi(version.Minor)
	k8sVersion := []int{majorVer, minorVer}
	
	return clientSet, kubeClient, kubeExtCli, k8sVersion, nil
}
