package design

// // 从 controller 订阅
// func Subscribe() {
// 	instanceId := "本实例的ID（podid-containerName）"
// 	serverName := "要订阅的服务名称"
// 	sequence := int64(0)

// 	// ch 是controller 推送的通道
// 	ch := sub(instanceId, serverName)

// 	for msg := range ch {
// 		// 处理消息
// 		msgType = msg.Type
// 		msgObject = msg.Object
// 		msgSequence = msg.Sequence

// 		if msgSequence-sequence > 1 {
// 			// 丢失消息，重新拉取全量数据
// 			sequence = msgSequence
// 			List()
// 			continue
// 		}

// 		switch msgType {
// 		case "ADD":
// 			// 处理新增
// 		case "UPDATE":
// 			// 处理更新
// 		case "DELETE":
// 			// 处理删除
// 		}
// 		sequence = msgSequence
// 	}

// 	// ch 断开，尝试重新订阅
// }
