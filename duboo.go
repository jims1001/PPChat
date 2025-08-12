package main

//"dubbo.apache.org/dubbo-go/v3/common"

import (
	pb "PProject/gen/user"
	"PProject/module/user"
	_ "dubbo.apache.org/dubbo-go/v3/imports"
	"dubbo.apache.org/dubbo-go/v3/protocol"
	_ "dubbo.apache.org/dubbo-go/v3/protocol/dubbo"
	"dubbo.apache.org/dubbo-go/v3/server"
	"github.com/nacos-group/nacos-sdk-go/v2/common/logger"
)

func main() {

	//server.beginImport();
	//server.addProcol();
	//

	srv, err := server.NewServer(
		server.WithServerProtocol(
			protocol.WithPort(20000),
			protocol.WithTriple(),
		),
	)
	if err != nil {
		panic(err)
	}

	if err := pb.RegisterUserServiceHandler(srv, &user.UserServiceProvider{}); err != nil {
		panic(err)
	}

	if err := srv.Serve(); err != nil {
		logger.Error(err)
	}
}
