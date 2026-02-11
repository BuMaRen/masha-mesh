package election

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/klog/v2"
)

func slave(ctx context.Context) {
	<-ctx.Done()
}

func Elect(kc *kubernetes.Clientset, namespace, podName string) error {

	id := podName
	electionContext := context.Background()

	startLeadingCh := make(chan struct{}, 1)

	leaderelection.RunOrDie(electionContext, leaderelection.LeaderElectionConfig{
		Lock:          nil, // Placeholder for actual lock implementation
		LeaseDuration: 15 * 1e9,
		RenewDeadline: 10 * 1e9,
		RetryPeriod:   2 * 1e9,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Infof("start watching kubernetes\n")
				startLeadingCh <- struct{}{}
				// Logic to execute when this pod becomes the leader
			},
			OnStoppedLeading: func() {
				klog.Infof("stop watching kubernetes\n")
				// Logic to execute when this pod stops being the leader
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					klog.Infof("I am the leader now\n")
					return
				}

				slaveContext, cancel := context.WithCancel(context.Background())
				go func() {
					<-startLeadingCh
					cancel()
				}()

				klog.Infof("new leader is %v\n", identity)
				klog.Infof("i am slave, watch etcd for changes\n")
				slave(slaveContext)
				klog.Infof("stop being slave\n")
			},
		},
		WatchDog:        &leaderelection.HealthzAdaptor{},
		ReleaseOnCancel: true,
		Name:            "",
	})

	return nil
}
