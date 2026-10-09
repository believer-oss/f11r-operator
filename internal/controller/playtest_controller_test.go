package controller

import (
	"context"

	gamev1alpha1 "github.com/believer-oss/f11r-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	r            *PlaytestReconciler
	req          ctrl.Request
	testPlaytest *gamev1alpha1.Playtest
	ctx          context.Context
)

var _ = Describe("PlaytestController", func() {
	Describe("Random Group Assignment", func() {
		BeforeEach(func() {
			ctx = context.Background()

			r = &PlaytestReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
			}

			req = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      "test-playtest",
				},
			}

			testPlaytest = &gamev1alpha1.Playtest{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test-playtest",
				},
				Spec: gamev1alpha1.PlaytestSpec{
					MinGroups:       3,
					PlayersPerGroup: 3,
					Groups: []gamev1alpha1.PlaytestGroup{
						{
							Name: "Group 1",
						},
						{
							Name: "Group 2",
						},
						{
							Name: "Group 3",
						},
					},
					UsersToAutoAssign: []string{"test1"},
				},
				Status: gamev1alpha1.PlaytestStatus{},
			}
		})

		JustBeforeEach(func() {
			err := r.Create(ctx, testPlaytest)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := r.Delete(ctx, testPlaytest)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there are players to assign", func() {
			Context("and all groups are empty", func() {
				It("should assign players to groups randomly", func() {
					_, err := r.Reconcile(ctx, req)
					Expect(err).ToNot(HaveOccurred())

					playtest := &gamev1alpha1.Playtest{}
					err = r.Get(ctx, req.NamespacedName, playtest)
					Expect(err).ToNot(HaveOccurred())

					Expect(playtest.Spec.Groups).To(HaveLen(3))

					playerAssigned := false
					for _, group := range playtest.Spec.Groups {
						if len(group.Users) > 0 {
							playerAssigned = true
						}
					}

					Expect(playerAssigned).To(BeTrue())
				})
			})

			Context("and only one group has space", func() {
				BeforeEach(func() {
					testPlaytest.Spec.Groups[0].Users = []string{"test2", "test3", "test4"}
					testPlaytest.Spec.Groups[2].Users = []string{"test5", "test6", "test7"}
				})

				It("should assign players to that open group", func() {
					_, err := r.Reconcile(ctx, req)
					Expect(err).ToNot(HaveOccurred())

					playtest := &gamev1alpha1.Playtest{}
					err = r.Get(ctx, req.NamespacedName, playtest)
					Expect(err).ToNot(HaveOccurred())

					Expect(playtest.Spec.Groups).To(ContainElement(gamev1alpha1.PlaytestGroup{
						Name:  "Group 2",
						Users: []string{"test1"},
					}))
				})
			})
		})
	})
})

var _ = Describe("PlaytestController gameClientCmdArgs", func() {
	It("keeps gameClientCmdArgs through create and reconcile", func() {
		ctx := context.Background()
		key := types.NamespacedName{Namespace: "default", Name: "client-args-playtest"}
		pt := &gamev1alpha1.Playtest{
			ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
			Spec: gamev1alpha1.PlaytestSpec{
				StartTime:          metav1.Now(),
				DisableGameServers: true,
				GameClientCmdArgs:  []string{"-tracefile=test.utrace", "-statnamedevents"},
			},
		}
		Expect(k8sClient.Create(ctx, pt)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, pt)).To(Succeed()) })

		rec := &PlaytestReconciler{Client: k8sClient, Scheme: scheme.Scheme}
		_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		Expect(err).ToNot(HaveOccurred())

		got := &gamev1alpha1.Playtest{}
		Expect(k8sClient.Get(ctx, key, got)).To(Succeed())
		Expect(got.Spec.GameClientCmdArgs).To(Equal([]string{"-tracefile=test.utrace", "-statnamedevents"}))
	})
})
