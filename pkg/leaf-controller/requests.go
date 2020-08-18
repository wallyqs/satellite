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
	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) processConfigGetRequest(m *nats.Msg) {
	log.Infof("Processing get request: %+v", string(m.Data))

	c.mu.Lock()
	kc := c.kc
	ns := c.opts.PodNamespace
	cmName := c.opts.ConfigMapName
	c.mu.Unlock()

	// Get latest configmap, then apply the changes.
	cm, err := kc.CoreV1().ConfigMaps(ns).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Kubernetes API request failed on get: %s", err)
		m.Respond([]byte("Error:" + err.Error()))
		return
	}
	m.Respond([]byte(cm.Data["routes.json"]))
}

func (c *Controller) processConfigUpdateRequest(m *nats.Msg) {
	log.Infof("Processing update request: %+v", string(m.Data))

	c.mu.Lock()
	kc := c.kc
	ns := c.opts.PodNamespace
	cmName := c.opts.ConfigMapName
	c.mu.Unlock()

	// Get latest configmap, then apply the changes.
	cm, err := kc.CoreV1().ConfigMaps(ns).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Kubernetes API request failed on get: %s", err)
		m.Respond([]byte("Error:" + err.Error()))
		return
	}

	// TODO: Add validation
	cm.Data["routes.json"] = string(m.Data)

	cm, err = kc.CoreV1().ConfigMaps(ns).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Kubernetes API request failed on update: %s", err)
		m.Respond([]byte("Error:" + err.Error()))
		return
	}
	m.Respond([]byte("Successfully update ConfigMap"))
}

func (c *Controller) processDiscoverRequest(m *nats.Msg) {
	log.Infof("Processing discover request: %+v", string(m.Data))
	// TODO
	m.Respond([]byte("OK"))
}

func (c *Controller) processStatusRequest(m *nats.Msg) {
	log.Infof("Processing status request: %+v", string(m.Data))
	// TODO
	m.Respond([]byte("OK"))
}
