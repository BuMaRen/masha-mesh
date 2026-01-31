package design

// data 存储完整的 kubernetes endpoints 信息

// data 模块负责存储从 kubernetes 订阅得到数据
// 提供查询接口
// 负责向 sync 或者 logic 注册的 ch 中推送消息
type ControllerData struct {
	// 当前数据的序列号

	// 全量数据存储
	data_mp map[string]service // key: serviceName, value: service 对象

	// 全局订阅存储
	global_mp map[string]chan service // key: instanceId, value: chan
}

// 提供全部信息的查询
func List() {

}

func Update(eventType string, svc service) {
	// 更新全量数据
	lock()
	defer unlock()

	// 合并变更
	switch eventType {
	case "ADD":
		controllerData.data_mp[svc.Name] = svc
	case "UPDATE":
		controllerData.data_mp[svc.Name] = svc
	case "DELETE":
		delete(controllerData.data_mp, svc.Name)
	}
}
