package main

import (
	"os"

	"time"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Usage: "Kube config for outside of cluster access",
		},
	}

	app.Action = func(c *cli.Context) error {
		clientset, err := getClient(c.String("config"))
		if err != nil {
			return err
		}
		for {
			nodes, err := clientset.Core().Nodes().List(v1.ListOptions{})
			if err != nil {
				return err
			}
			for _, node := range nodes.Items {
				logrus.Infof("Node is %v", node.Name)
			}
			time.Sleep(5 * time.Second)
		}
	}
	app.Run(os.Args)
}

func getClient(cfg string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if cfg == "" {
		logrus.Info("Using in cluster config")
		config, err = rest.InClusterConfig()
		// in cluster access
	} else {
		logrus.Info("Using out of cluster config")
		config, err = clientcmd.BuildConfigFromFlags("", cfg)
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}
