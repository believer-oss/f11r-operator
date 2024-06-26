# f11r-operator

This controller is intended to help orchestrate prototype game server deployments. 

## Description

This repository defines a `GameServer` custom resource for Kubernetes. Upon creating a `GameServer` object, the controller will create a Pod with a port chosen from a configurable range and configure the underlying container to listen on that port. As of this moment, this system relies on host networking.

```yaml
apiVersion: game.believer.dev/v1alpha1
kind: GameServer
metadata:
  name: gameserver-sample
spec:
  # optional display name override (Friendshipper uses this)
  displayName: my cool server

  # path to Unreal map to load
  map: /Game/Levels/MyMap

  # image tag to use for game server
  version: my-tag-123
```

There is also a `Playtest` custom resource which automatically provisions `GameServer` objects based on some parameters. 

```yaml
apiVersion: game.believer.dev/v1alpha1
kind: Playtest
metadata:
  name: playtest-sample
spec:
  # optional display name override (Friendshipper uses this)
  displayName: my cool playtest

  # URL to Playtest feedback form (optional)
  feedbackURL: example.com

  # path to Unreal map to load
  map: /Game/Levels/MyMap

  # starting number of groups (one GameServer will be provisioned for each)
  minGroups: 2

  # maximum allowed number of players per group
  playersPerGroup: 2

  # playtest start time (servers will be provisioned relative to this time)
  startTime: "2024-01-01T00:00:00.000Z"

  # image tag to use for game server
  version: my-tag-123
```

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/f11r-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/f11r-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

