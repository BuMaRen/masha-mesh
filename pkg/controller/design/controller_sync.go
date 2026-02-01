package design

// // sync 同步的时候只同步 ip port

// // rpc, 实现 sidecar 的调用
// func Subscribe(stream) {
// 	instanceId := "这个sidecar实例的ID"
// 	ServiceName := "这个sidecar要订阅的服务名称"

// 	// channel 保存起来用于统一通知，注册回调会更好？
// 	ch := make(chan service)
// 	controllerData.global_mp[instanceId] = ch

// 	for stream is not closed {
// 		pubMsg := <-ch
// 		stream.Send(pubMsg)
// 	}
// 	delete(controllerData.global_mp, instanceId)
// }

// // rpc, sidecar 拉取全量数据
// func List() services {
// 	ServiceName := "这个sidecar要订阅的服务名称"
// 	services := controllerData.List()
// 	return services
// }
