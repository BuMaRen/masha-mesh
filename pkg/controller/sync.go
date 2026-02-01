package controller

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/BuMaRen/mesh/pkg/api/mesh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	OpTypeAdd    = "ADD"
	OpTypeUpdate = "UPDATE"
	OpTypeDelete = "DELETE"
	OpTypeList   = "LIST"
	OpTypeModify = "MODIFY"
)

func NewSyncer(data *Data) *Syncer {
	return &Syncer{
		data: data,
	}
}

type Syncer struct {
	data *Data
	mesh.UnimplementedControlFaceServer
}

func (s *Syncer) parseEvent(serviceName string, event *watch.Event) (map[string]*mesh.Endpoint, error) {
	// 解析 kubernetes 的 EndpointSliceList 事件，转换成 mesh.Endpoint 格式
	endpointSlice, ok := event.Object.(*discoveryv1.EndpointSlice)
	if !ok {
		return nil, fmt.Errorf("object is not EndpointSlice")
	}
	eventServiceName := endpointSlice.Labels["kubernetes.io/service-name"]
	endpoints := make(map[string]*mesh.Endpoint)
	// 只处理指定服务的 EndpointSlice
	if eventServiceName != serviceName {
		return nil, fmt.Errorf("serviceName not match: event=%s target=%s", eventServiceName, serviceName)
	}
	for _, endpoint := range endpointSlice.Endpoints {
		if endpoint.TargetRef == nil {
			log.Printf("[sync] endpoint without TargetRef in service=%s", serviceName)
			continue
		}
		combineIP := strings.Join(endpoint.Addresses, ",")
		ep := &mesh.Endpoint{
			Ip:   combineIP,
			Port: 0,
		}
		// 唯一标识符使用 endpoint 的 TargetRef.Name
		endpoints[endpoint.TargetRef.Name] = ep
	}
	log.Printf("[sync] parsed endpoints=%d for service=%s", len(endpoints), serviceName)
	return endpoints, nil
}

func (s *Syncer) Subscribe(sr *mesh.SubscribeRequest, sss grpc.ServerStreamingServer[mesh.ServiceUpdate]) error {
	instanceId := sr.InstanceId
	serviceName := sr.ServiceName
	log.Printf("[sync] Subscribe start instance=%s service=%s", instanceId, serviceName)

	// 准备好 channel，注册到 Data 结构体中
	eventCh := make(chan *informer)
	defer close(eventCh)
	s.data.Register(instanceId, ServiceName(serviceName), eventCh)

	for {
		// 等待 Data 面有新的事件推送过来
		info, opened := <-eventCh
		if !opened {
			log.Printf("[sync] event channel closed instance=%s", instanceId)
			break
		}
		log.Printf("[sync] received event type=%s rev=%d for instance=%s", info.event.Type, info.revision, instanceId)

		// 解析 kubernetes 事件，转换成 mesh.Endpoint 格式
		eps, err := s.parseEvent(serviceName, info.event)
		if err != nil {
			log.Printf("[sync] parseEvent warn: %v", err)
			continue
		}

		// 将 instanceid-endpoint 的数据发送给 sidecar
		if err = sss.Send(&mesh.ServiceUpdate{
			OpType:    string(info.event.Type),
			Revision:  info.revision,
			Endpoints: eps,
		}); err != nil {
			log.Printf("[sync] send error: %v", err)
			break
		}
		log.Printf("[sync] sent update endpoints=%d instance=%s", len(eps), instanceId)
	}
	log.Printf("[sync] Subscribe end instance=%s", instanceId)
	return nil
}

func (s *Syncer) Unsubsribe(context.Context, *mesh.SubscribeRequest) (*mesh.ServiceUpdate, error) {
	return nil, status.Error(codes.Unimplemented, "method Unsubsribe not implemented")
}

func (s *Syncer) ListService(context.Context, *mesh.SubscribeRequest) (*mesh.ServiceUpdate, error) {
	return nil, status.Error(codes.Unimplemented, "method ListService not implemented")
}
