package controller

// import (
// 	"context"

// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// )

// func (c *Controller) List() {
// 	esi := c.clientSet.DiscoveryV1().EndpointSlices(c.Namespace)
// 	esl, _ := esi.List(context.Background(), metav1.ListOptions{})
// 	for _, eps := range esl.Items {

// 	}
// }

// func (c *Controller) Watch() {
// 	esi := c.clientSet.DiscoveryV1().EndpointSlices(c.Namespace)
// 	wi, _ := esi.Watch(context.Background(), metav1.ListOptions{})
// 	for event := range wi.ResultChan() {

// 	}
// }
