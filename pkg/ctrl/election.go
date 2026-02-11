package ctrl

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

func (l *Logic) ElectLoop(namespace, podName string) error {

	id := podName
	electionContext := context.Background()

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	clientSet := kubernetes.NewForConfigOrDie(kubeConfig)

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "election-lease",
			Namespace: namespace,
		},
		Client: clientSet.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	grpcServerStarted := false

	leaderelection.RunOrDie(electionContext, leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: 60 * time.Second,
		RenewDeadline: 15 * time.Second,
		RetryPeriod:   5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Infof("start watching kubernetes\n")
				l.Leading(ctx, clientSet)
				klog.Infof("stop being leader\n")
			},
			OnStoppedLeading: func() {
				klog.Infof("stop watching kubernetes\n")
			},
			OnNewLeader: func(identity string) {
				klog.Infof("new leader is %v\n", identity)
				if grpcServerStarted == false {
					klog.Infof("start grpc server following the leader election\n")
					go l.Following(electionContext)
					grpcServerStarted = true
				}
			},
		},
		WatchDog:        &leaderelection.HealthzAdaptor{},
		ReleaseOnCancel: true,
		Name:            "mesh-controller-leader-election",
	})

	return nil
}
