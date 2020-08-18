
In this repo a sample implementation of controlling NATS clusters routing
configuration by using the Kubernetes API.

## Example usage

For example, let's say that we have a cluster named `nats` running in K8S,
that we have deployed as a `StatefulSet`:

```sh
$ kubectl apply -f k8s/nats-cluster-A.yaml
configmap/nats-config created
service/nats created
statefulset.apps/nats created
```

The pods in this cluster, will all be sharing a single `nats-config` ConfigMap
that can be updated and reloaded at at anytime by the reloader sidecar
in case it detects changes to it.  This NATS cluster will have a leafnode
connection to another external NATS Server so that it can be controlled
remotely.

In order to do so, connected to the same NATS cluster, there will be a `leaf-controller` process
that will expose a NATS based API that can be used to make updates to the 
remote ConfigMap from a NATS cluster.  

To deploy this `leaf-controller`:

```sh
$ kubectl apply -f k8s/leaf-controller-A.yaml
serviceaccount/nats created
clusterrole.rbac.authorization.k8s.io/nats created
clusterrolebinding.rbac.authorization.k8s.io/nats-binding created
deployment.apps/nats-leaf-controller created
```

Now in order to make an update of the routes, we can send a message
so that the reloader sidecar picks up the change and adds the route
to all the pods from a cluster:

```sh
$ export CLUSTER_NAME="nats"
$ nats-req -s $SOME_NATS_SERVER _SAT.$CLUSTER_NAME.config.put '{
  "name": "CLUSTER_A",
  "routes": [
    "nats://nats-0.nats.default.svc:6222",
    "nats://nats-1.nats.default.svc:6222",
    "nats://nats-2.nats.default.svc:6222",
    "nats://another-route:6222"
  ]
}'
```

Eventually in the logs (it can take about a minute), there would be a message
that the reload was successful:

```sh
$ kubectl logs nats-0 -c nats --follow
[6] 2020/08/18 21:36:40.859351 [INF] Cluster name updated to CLUSTER_A
[6] 2020/08/18 21:36:40.859360 [INF] Reloaded: cluster
[6] 2020/08/18 21:36:40.859425 [INF] Reloaded server configuration
```

## Local Development

To run locally the leaf-controller with the local `kubectl` credentials:

```sh
KUBERNETES_CONFIG_FILE=~/.kube/config go run cmd/leaf-controller/main.go
INFO[2020-08-18T14:05:17-07:00] Starting NATS Leaf Controller v0.1.0         
INFO[2020-08-18T14:05:17-07:00] Go Version: go1.14.2  
```

## Docker Image

To build the Docker image:

```sh
docker build -f docker/leaf-controller/Dockerfile -t wallyqs/satellite:latest .
```
