package main

import (
	"os"

	"time"

	"math"

	"github.com/Sirupsen/logrus"
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
	watchList := cache.NewListWatchFromClient(clientset.Core().RESTClient(), "nodes", v1.NamespaceAll,
		fields.Everything())
	cache, controller := cache.NewInformer(
		watchList,
		&api.Node{},
		time.Second*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    handleNodeAdd,
			UpdateFunc: handleNodeUpdate,
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
}

func handleNodeAdd(obj interface{}) {
	node := obj.(*api.Node)
	logrus.Infof("Node [%s] is added; allocated capacity is %v%%", node.Name, getNodeAllocatedCapacity(node))
}

func handleNodeUpdate(old, current interface{}) {
	node := current.(*api.Node)
	logrus.Infof("Node [%s] is updated; allocated capacity is %v%%", node.Name, getNodeAllocatedCapacity(node))
}

func pollNodes() error {
	for {
		nodes, err := clientset.Core().Nodes().List(v1.ListOptions{})
		if err != nil {
			return err
		}
		for _, node := range nodes.Items {
			logrus.Infof("Node [%s] allocated capacity is %v%%", node.Name, getNodeAllocatedCapacity(&node))
		}
		time.Sleep(5 * time.Second)
	}
}

func getNodeAllocatedCapacity(node *api.Node) float64 {
	a := node.Status.Allocatable[api.ResourceMemory]
	c := node.Status.Capacity[api.ResourceMemory]
	allocatable, _ := a.AsInt64()
	capacity, _ := c.AsInt64()
	diff := float64(capacity - allocatable)
	allocated := (diff / float64(capacity)) * 100
	return round(allocated, 0.5, 2)
}

func round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
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
