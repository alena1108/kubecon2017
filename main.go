package main

import (
	"os"

	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var VERSION = "v0.0.0-dev"

var clientset *kubernetes.Clientset

var controller cache.Controller
var store cache.Store

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Usage: "Kube config for outside of cluster access",
		},
	}

	app.Action = func(c *cli.Context) error {
		var err error
		clientset, err = getClient(c.String("config"))
		if err != nil {
			logrus.Error(err)
			return err
		}
		go pollNodes()
		watchNodes()
		for {
			time.Sleep(5 * time.Second)
		}
	}
	app.Run(os.Args)
}

func watchNodes() {
	//Regular informer example
	watchList := cache.NewListWatchFromClient(clientset.Core().RESTClient(), "nodes", v1.NamespaceAll,
		fields.Everything())
	store, controller = cache.NewInformer(
		watchList,
		&api.Node{},
		time.Second*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    handleNodeAdd,
			UpdateFunc: handleNodeUpdate,
		},
	)

	// // Shared informer example
	// informer := cache.NewSharedIndexInformer(
	// 	watchList,
	// 	&api.Node{},
	// 	time.Second*10,
	// 	cache.Indexers{},
	// )

	// informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc:    handleNodeAdd,
	// 	UpdateFunc: handleNodeUpdate,
	// })

	// // More than one handler can be added...
	// informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc:    handleNodeAddExtra,
	// 	UpdateFunc: handleNodeUpdateExtra,
	// })

	stop := make(chan struct{})
	go controller.Run(stop)
}

func handleNodeAdd(obj interface{}) {
	node := obj.(*api.Node)
	logrus.Infof("Node [%s] is added; checking resources...", node.Name)
	checkImageStorage(node)
}

func handleNodeUpdate(old, current interface{}) {

	// nodeInterface, exists, err := store.GetByKey("minikube")
	// if exists && err == nil {
	// 	logrus.Infof("Found the node [%v] in cache", nodeInterface)
	// }

	node := current.(*api.Node)
	logrus.Infof("Node [%s] is updated; checking resources...", node.Name)
	checkImageStorage(node)
}

func pollNodes() error {
	for {
		nodes, err := clientset.Core().Nodes().List(v1.ListOptions{FieldSelector: "metadata.name=minikube"})
		if len(nodes.Items) > 0 {
			node := nodes.Items[0]
			node.Annotations["checked"] = "true"
			_, err := clientset.Core().Nodes().Update(&node)
			if err != nil {
				return err
			}
			// gracePeriod := int64(10)
			// err = clientset.Core().Nodes().Delete(updatedNode.Name,
			// 	&v1.DeleteOptions{GracePeriodSeconds: &gracePeriod})
		}
		if err != nil {
			return err
		}
		for _, node := range nodes.Items {
			checkImageStorage(&node)
		}
		time.Sleep(5 * time.Second)
	}
}

func checkImageStorage(node *api.Node) {
	var storage int64
	for _, image := range node.Status.Images {
		storage = storage + image.SizeBytes
	}
	logrus.Infof("Node [%s] has [%s] occupied by images", node.Name, humanize.Bytes(uint64(storage)))
}

func isNodeUnderPressure(node *api.Node) bool {
	memoryPressure := false
	for _, condition := range node.Status.Conditions {
		if condition.Type == "MemoryPressure" {
			if condition.Status == "True" {
				memoryPressure = true
			}
			break
		}
	}
	return memoryPressure
}

func getClient(pathToCfg string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if pathToCfg == "" {
		logrus.Info("Using in cluster config")
		config, err = rest.InClusterConfig()
		// in cluster access
	} else {
		logrus.Info("Using out of cluster config")
		config, err = clientcmd.BuildConfigFromFlags("", pathToCfg)
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}
