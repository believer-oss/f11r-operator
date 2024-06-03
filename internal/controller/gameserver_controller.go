/*
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
*/

package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gamev1alpha1 "github.com/believer-oss/f11r-operator/api/v1alpha1"
)

const (
	ErrPortConflict = "node(s) didn't have free ports for the requested pod ports"
)

// GameServerReconciler reconciles a GameServer object
type GameServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	GameServerImage string
	GamePortMin     int32
	GamePortMax     int32
	NetImguiPortMin int32
}

//+kubebuilder:rbac:groups=game.believer.dev,resources=gameservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=game.believer.dev,resources=gameservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=game.believer.dev,resources=gameservers/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GameServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *GameServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// At this point the Reconcile function has been handed a name and a namespace. Now we need to fetch the object.
	gameServer := &gamev1alpha1.GameServer{}
	if err := r.Client.Get(ctx, req.NamespacedName, gameServer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(gameServer, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// No matter what happens during reconciliation, we want to try to patch the object at the end and catch updates
	defer func() {
		if err := patchHelper.Patch(ctx, gameServer); err != nil {
			log.Error(err, "error patching object")
		}
	}()

	// backwards compatible DisplayName
	if val := gameServer.GetLabels()["believer.dev/name"]; val != "" && gameServer.Spec.DisplayName == "" {
		gameServer.Spec.DisplayName = val
	}

	if gameServer.Spec.DisplayName == "" {
		gameServer.Spec.DisplayName = gameServer.GetName()
	}

	return r.reconcilePod(ctx, gameServer)
}

func (r *GameServerReconciler) reconcilePod(ctx context.Context, gameServer *gamev1alpha1.GameServer) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if gameServer.Status.PodRef != nil {
		pod := &corev1.Pod{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: gameServer.GetNamespace(), Name: gameServer.Status.PodRef.Name}, pod); err != nil {
			// if the Pod is missing, make sure we don't have a PodRef
			if apierrors.IsNotFound(err) {
				log.Info("missing Pod for GameServer, requeuing for a fresh one")
				gameServer.Status.PodRef = nil

				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}

		// check Pod conditions
		switch pod.Status.Phase {
		case corev1.PodPending:
			// if unschedulable because of port conflict, delete the Pod and requeue
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodScheduled && condition.Reason == corev1.PodReasonUnschedulable && strings.Contains(condition.Message, ErrPortConflict) {
					log.Info("port conflict detected, rescheduling pod")
					if err := r.Client.Delete(ctx, pod); err != nil {
						return ctrl.Result{}, err
					}

					gameServer.Status.PodRef = nil

					return ctrl.Result{Requeue: true}, nil
				}
			}
		case corev1.PodSucceeded:
			// if the pod has exited successfully, we should delete the GameServer object
			if err := r.Client.Delete(ctx, gameServer); err != nil {
				if apierrors.IsNotFound(err) {
					return ctrl.Result{}, nil
				}

				log.Error(err, "Error deleting GameServer")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		if len(pod.Spec.Containers[0].Ports) == 0 {
			return ctrl.Result{Requeue: true}, nil
		}

		if gameServer.Status.Port != pod.Spec.Containers[0].Ports[0].HostPort {
			gameServer.Status.Port = pod.Spec.Containers[0].Ports[0].HostPort
		}

		if gameServer.Status.NetImguiPort != pod.Spec.Containers[0].Ports[1].HostPort {
			gameServer.Status.NetImguiPort = pod.Spec.Containers[0].Ports[1].HostPort
		}

		// requeue until we've got a node
		if pod.Spec.NodeName == "" {
			return ctrl.Result{Requeue: true}, nil
		}

		ip, err := r.getExternalIPForNode(ctx, pod.Spec.NodeName)
		if err != nil {
			return ctrl.Result{}, err
		}

		if ip == "" {
			log.Error(errors.New("Node does not have a public IP"), "probably no point in requeuing")
			return ctrl.Result{}, nil
		}

		if gameServer.Status.IP != ip {
			gameServer.Status.IP = ip
		}

		return ctrl.Result{}, nil
	}

	image := fmt.Sprintf("%s:%s", r.GameServerImage, gameServer.Spec.Version)

	args := []string{}

	if gameServer.Spec.Map != "" {
		args = append(args, gameServer.Spec.Map)
	}

	// Select port randomly from our range
	// We do not need to keep track of which ports we've assigned. This controller watches
	// Pods that are children of GameServers, and can detect when one is unschedulable due to a port
	// conflict, then delete the Pod. Yes, as we start to run out of overall ports there will be some
	// thrashing, but doing it this way allows us to scale out nodes without needing to maintain a
	// mapping of nodes and ports.
	port := rand.Int31n(r.GamePortMax-r.GamePortMin) + r.GamePortMin
	portArg := fmt.Sprintf("-port=%d", port)

	netimguiPort := (port - r.GamePortMin) + r.NetImguiPortMin
	netimguiPortArg := fmt.Sprintf("-NetImguiClientPort=%d", netimguiPort)

	args = append(args, portArg)
	args = append(args, netimguiPortArg)

	storageKey := gameServer.Spec.DisplayName
	if storageKey == "" {
		storageKey = gameServer.GetName()
	}
	storageKey = sanitizeStorageKey(storageKey)

	// Add StorageKey argument
	args = append(args, fmt.Sprintf("-StorageKey=%s", storageKey))

	// We need to create a Pod.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"karpenter.sh/do-not-disrupt": "true",
			},
			Name:      gameServer.GetName(),
			Namespace: gameServer.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: gamev1alpha1.GroupVersion.String(),
					Kind:       "GameServer",
					Name:       gameServer.GetName(),
					UID:        gameServer.GetUID(),
					Controller: pointer.Bool(true),
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "game-server",
					Image: image,
					Args:  args,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: port,
							Protocol:      corev1.ProtocolUDP,
						},
						{
							ContainerPort: netimguiPort,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
			HostNetwork: true,
			NodeSelector: map[string]string{
				"builddev.believer.dev/nodetype": "game",
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Tolerations: []corev1.Toleration{
				{
					Key:    "builddev.believer.dev/game",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	if err := r.Client.Create(ctx, pod); err != nil {
		// get the pod again and check if it's completed, then delete it if so
		if apierrors.IsAlreadyExists(err) {
			pod := &corev1.Pod{}
			if err := r.Client.Get(ctx, types.NamespacedName{Namespace: gameServer.GetNamespace(), Name: gameServer.GetName()}, pod); err != nil {
				if apierrors.IsNotFound(err) {
					return ctrl.Result{Requeue: true}, nil
				}

				return ctrl.Result{}, err
			}

			switch pod.Status.Phase {
			case corev1.PodSucceeded:
				if err := r.Client.Delete(ctx, pod); err != nil {
					if apierrors.IsNotFound(err) {
						return ctrl.Result{Requeue: true}, nil
					}

					return ctrl.Result{}, err
				}
			default:
				log.Info("Pod already exists and has not completed, requeuing")
				gameServer.Status.PodRef = &corev1.LocalObjectReference{
					Name: pod.GetName(),
				}

				return ctrl.Result{Requeue: true}, nil
			}
		}
		return ctrl.Result{}, err
	}

	gameServer.Status.PodRef = &corev1.LocalObjectReference{
		Name: pod.GetName(),
	}

	return ctrl.Result{}, nil
}

func (r *GameServerReconciler) getExternalIPForNode(ctx context.Context, nodeName string) (string, error) {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		return "", err
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeExternalIP {
			return addr.Address, nil
		}
	}

	return "", nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GameServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamev1alpha1.GameServer{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func buildStorageKeyFromServerName(gameServer *gamev1alpha1.GameServer) string {
	storageKey := gameServer.Spec.DisplayName
	if storageKey == "" {
		storageKey = gameServer.GetName()
	}

	return sanitizeStorageKey(storageKey)
}

func sanitizeStorageKey(key string) string {
	foundNonSlash := false
	var santized bytes.Buffer
	for _, c := range key {
		if foundNonSlash == false && c == '/' {
			santized.WriteRune('_')
		} else if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '/' && c != '_' && c != '-' && c != '.' {
			foundNonSlash = true
			santized.WriteRune('_')
		} else {
			santized.WriteRune(c)
			foundNonSlash = true
		}
	}

	return santized.String()
}
