package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	engine := gin.Default()

	engine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	//certFile：服务器证书文件（通常是 .crt / .pem），
	// 里面是公开信息，包括域名、颁发者、有效期、服务器公钥等；
	// 客户端用它来验证“你是谁”。
	//keyFile：服务器私钥文件（通常是 .key），
	// 是和证书里公钥配对的私密信息；
	// TLS 握手时服务器用它来证明自己确实拥有该证书对应的私钥。
	if err := engine.RunTLS(":9987", "server.crt", "server.key"); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}
}
