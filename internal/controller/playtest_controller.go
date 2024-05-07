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
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gamev1alpha1 "github.com/believer-oss/f11r-operator/api/v1alpha1"
)

// PlaytestReconciler reconciles a Playtest object
type PlaytestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=game.believer.dev,resources=playtests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=game.believer.dev,resources=playtests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=game.believer.dev,resources=playtests/finalizers,verbs=update
//+kubebuilder:rbac:groups=game.believer.dev,resources=gameservers,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Playtest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *PlaytestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	playtest := &gamev1alpha1.Playtest{}
	if err := r.Get(ctx, req.NamespacedName, playtest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(playtest, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// No matter what happens during reconciliation, we want to try to patch the object at the end and catch updates
	defer func() {
		if err := patchHelper.Patch(ctx, playtest); err != nil {
			log.Error(err, "error patching object")
		}
	}()

	return r.reconcilePlaytest(ctx, playtest)
}

func (r *PlaytestReconciler) reconcilePlaytest(ctx context.Context, playtest *gamev1alpha1.Playtest) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// First, set up default groups
	if len(playtest.Spec.Groups) == 0 {
		for i := 1; i <= playtest.Spec.MinGroups; i++ {
			playtest.Spec.Groups = append(playtest.Spec.Groups, gamev1alpha1.PlaytestGroup{
				Name:  fmt.Sprintf("Group %d", i),
				Users: []string{},
			})
		}
	} else if len(playtest.Spec.Groups) < playtest.Spec.MinGroups {
		for i := len(playtest.Spec.Groups); i < playtest.Spec.MinGroups; i++ {
			log.Info("adding group", "group", fmt.Sprintf("Group %d", i))

			playtest.Spec.Groups = append(playtest.Spec.Groups, gamev1alpha1.PlaytestGroup{
				Name:  fmt.Sprintf("Group %d", i+1),
				Users: []string{},
			})
		}
	} else if len(playtest.Spec.Groups) > playtest.Spec.MinGroups {
		playtest.Spec.Groups = playtest.Spec.Groups[:playtest.Spec.MinGroups]
	}

	// Then, auto assign any users, requeue each time for sanity
	if playtest.Spec.UsersToAutoAssign != nil && len(playtest.Spec.UsersToAutoAssign) > 0 {
		user := playtest.Spec.UsersToAutoAssign[0]

		openGroups := []int{}
		for i := 0; i < len(playtest.Spec.Groups); i++ {
			group := &playtest.Spec.Groups[i]
			if len(group.Users) < playtest.Spec.PlayersPerGroup {
				openGroups = append(openGroups, i)
			}
		}

		// Shuffle for random assignment
		if len(openGroups) == 0 {
			log.Error(errors.New("no open groups"), "no open groups")

			return ctrl.Result{}, nil
		}

		r := rand.New(rand.NewSource((time.Now().UnixNano())))
		groupIndex := r.Intn(len(openGroups))

		group := &playtest.Spec.Groups[openGroups[groupIndex]]
		group.Users = append(group.Users, user)

		playtest.Spec.UsersToAutoAssign = playtest.Spec.UsersToAutoAssign[1:]

		return ctrl.Result{Requeue: true}, nil
	}

	// Create a gameserver for each group, if it doesn't exist
	shouldRequeue := false
	for _, group := range playtest.Spec.Groups {
		var err error

		shouldRequeue, err = r.reconcileGroupServer(ctx, playtest, group)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if len(playtest.Status.Groups) > playtest.Spec.MinGroups {
		playtest.Status.Groups = playtest.Status.Groups[:playtest.Spec.MinGroups]
	}

	return ctrl.Result{Requeue: shouldRequeue}, nil
}

func (r *PlaytestReconciler) reconcileGroupServer(ctx context.Context, playtest *gamev1alpha1.Playtest, group gamev1alpha1.PlaytestGroup) (bool, error) {
	log := log.FromContext(ctx)

	// Find the group in the status
	groupStatus := getGroupStatus(playtest, group.Name)
	if groupStatus == nil {
		groupStatus = &gamev1alpha1.PlaytestGroupStatus{
			Name: group.Name,
		}
		playtest.Status.Groups = append(playtest.Status.Groups, *groupStatus)
	}

	groupStatus.Users = group.Users

	playtestServerVersion := playtest.Spec.Version
	if len(playtest.Spec.Version) == 8 {
		playtestServerVersion = fmt.Sprintf("linux-server-%s", playtest.Spec.Version)
	}

	if time.Now().UTC().Add(10 * time.Minute).After(playtest.Spec.StartTime.Time) {
		if groupStatus.ServerRef != nil {
			gameServer := &gamev1alpha1.GameServer{}
			if err := r.Client.Get(ctx, client.ObjectKey{
				Name:      groupStatus.ServerRef.Name,
				Namespace: playtest.GetNamespace(),
			}, gameServer); err != nil {
				if !apierrors.IsNotFound(err) {
					return false, err
				} else {
					groupStatus.ServerRef = nil
				}
			} else {
				if gameServer.Spec.Version != playtestServerVersion || gameServer.Spec.Map != playtest.Spec.Map {
					log.Info("deleting gameserver for group", "group", group.Name)

					if err := r.Client.Delete(ctx, gameServer); err != nil {
						return false, err
					}

					groupStatus.ServerRef = nil

					return true, nil
				}
			}

			return false, nil
		}

		log.Info("creating gameserver for group", "group", group.Name)

		formattedGroupName := strings.ReplaceAll(strings.ToLower(group.Name), " ", "-")
		gameServer := &gamev1alpha1.GameServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", playtest.GetName(), formattedGroupName),
				Namespace: playtest.GetNamespace(),
				Labels: map[string]string{
					"believer.dev/playtest": playtest.GetName(),
					"believer.dev/commit":   playtest.Spec.Version,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: gamev1alpha1.GroupVersion.String(),
						Kind:       "Playtest",
						Name:       playtest.GetName(),
						UID:        playtest.GetUID(),
						Controller: pointer.Bool(true),
					},
				},
			},
			Spec: gamev1alpha1.GameServerSpec{
				Version: playtestServerVersion,
				Map:     playtest.Spec.Map,
			},
		}

		if err := r.Client.Create(ctx, gameServer); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return false, err
			}
		}

		groupStatus.ServerRef = &corev1.LocalObjectReference{
			Name: gameServer.GetName(),
		}

		return false, nil
	}

	if groupStatus.ServerRef != nil {
		gameServer := &gamev1alpha1.GameServer{}
		if err := r.Client.Get(ctx, client.ObjectKey{
			Name:      groupStatus.ServerRef.Name,
			Namespace: playtest.GetNamespace(),
		}, gameServer); err != nil {
			if !apierrors.IsNotFound(err) {
				return false, err
			} else {
				groupStatus.ServerRef = nil
			}
		}

		if err := r.Client.Delete(ctx, gameServer); err != nil {
			// It's unclear how much we care about an error here - most errors
			// are probably because the object no longer exists, which is fine.
			log.Error(err, "error deleting gameserver")
		}

		groupStatus.ServerRef = nil
	}

	return false, nil
}

func getGroupStatus(playtest *gamev1alpha1.Playtest, groupName string) *gamev1alpha1.PlaytestGroupStatus {
	for i := 0; i < len(playtest.Status.Groups); i++ {
		group := &playtest.Status.Groups[i]
		if group.Name == groupName {
			return group
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlaytestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamev1alpha1.Playtest{}).
		Owns(&gamev1alpha1.GameServer{}).
		Complete(r)
}
