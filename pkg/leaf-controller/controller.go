// Copyright 2020 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package leafcontroller

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	// Load all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const Version = "0.1.0"

const (
	DiscoverSubject       = "_SAT.discover"
	StatusSubject         = "_SAT.%s.status"
	ConfigGetSubject      = "_SAT.%s.config.get"
	ConfigUpdateSubject   = "_SAT.%s.config.put"
	DefaultQueueGroupName = "controllers"
)

// Options are the options for the controller.
type Options struct {
	// ClusterName is the NATS cluster name.
	ClusterName string

	// NoSignals marks whether to enable the signal handler.
	NoSignals bool

	// NatsServerURL is the address of the ground NATS Server.
	NatsServerURL string

	// LeafRemoteCredentials are the auth keys for the NATS Leaf Server.
	NatsCredentials string

	// ConfigMapName is the config for this NATS Cluster that will be managed.
	ConfigMapName string

	// PodNamespace is the namespace where this controller is running.
	PodNamespace string
}

// Controller manages NATS Leaf node clusters running in Kubernetes.
type Controller struct {
	mu sync.Mutex

	// client to interact with Kubernetes resources.
	kc kubernetes.Interface

	// opts is the set of options.
	opts *Options

	// nc is the NATS connection.
	nc *nats.Conn

	// quit stops the controller.
	quit func()

	// shutdown is to shutdown the controller.
	shutdown bool
}

// NewController creats a new Controller.
func NewController(opts *Options) *Controller {
	if opts == nil {
		opts = &Options{}
	}
	if opts.ConfigMapName == "" {
		opts.ConfigMapName = fmt.Sprintf("%s-config", opts.ClusterName)
	}
	ns := os.Getenv("POD_NAMESPACE")
	if ns != "" {
		opts.PodNamespace = ns
	} else {
		opts.PodNamespace = "default"
	}
	return &Controller{
		opts: opts,
	}
}

// Run starts the controller.
func (c *Controller) Run(ctx context.Context) error {
	err := c.setupK8S()
	if err != nil {
		return err
	}

	err = c.setupNATS()
	if err != nil {
		return err
	}

	// Run until context is cancelled via a signal.x
	ctx, cancel := context.WithCancel(ctx)
	c.quit = func() {
		// Signal cancellation of the main context.
		cancel()
	}
	if !c.opts.NoSignals {
		go c.SetupSignalHandler(ctx)
	}

	// Wait for context to get cancelled or get a signal.
	select {
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (c *Controller) setupNATS() error {
	// Create subscriptions that can be used to make updates,
	// to the configuration map.
	nopts := make([]nats.Option, 0)
	nopts = append(nopts, nats.Name(fmt.Sprintf("leaf-controller:%s", c.opts.ClusterName)))
	nc, err := nats.Connect(c.opts.NatsServerURL)
	if err != nil {
		return err
	}

	_, err = nc.QueueSubscribe(fmt.Sprintf(ConfigGetSubject, c.opts.ClusterName),
		DefaultQueueGroupName, c.processConfigGetRequest)
	if err != nil {
		return err
	}

	_, err = nc.QueueSubscribe(fmt.Sprintf(ConfigUpdateSubject, c.opts.ClusterName),
		DefaultQueueGroupName, c.processConfigUpdateRequest)
	if err != nil {
		return err
	}

	_, err = nc.QueueSubscribe(fmt.Sprintf(StatusSubject, c.opts.ClusterName),
		DefaultQueueGroupName, c.processStatusRequest)
	if err != nil {
		return err
	}

	_, err = nc.Subscribe(DiscoverSubject, c.processDiscoverRequest)
	if err != nil {
		return err
	}

	c.nc = nc
	return nil
}

func (c *Controller) setupK8S() error {
	// Creates controller cluster config.
	var err error
	var config *rest.Config
	if kubeconfig := os.Getenv("KUBERNETES_CONFIG_FILE"); kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return err
	}

	// Client for the K8S API.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	c.kc = clientset

	return nil
}

// SetupSignalHandler enables handling process signals.
func (c *Controller) SetupSignalHandler(ctx context.Context) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for sig := range sigCh {
		log.Debugf("Trapped '%v' signal", sig)

		// If main context already done, then just skip
		select {
		case <-ctx.Done():
			continue
		default:
		}

		switch sig {
		case syscall.SIGINT:
			log.Infof("Exiting...")
			os.Exit(0)
			return
		case syscall.SIGTERM:
			// Gracefully shutdown the operator.  This blocks
			// until all controllers have stopped running.
			c.Shutdown()
			return
		}
	}
}

// Shutdown stops the operator controller.
func (c *Controller) Shutdown() {
	c.mu.Lock()
	if c.shutdown {
		c.mu.Unlock()
		return
	}
	c.shutdown = true
	nc := c.nc
	c.mu.Unlock()

	err := nc.Drain()
	log.Errorf("Error disconnecting from NATS: %s", err)

	c.quit()
	log.Infof("Bye...")
	return
}
