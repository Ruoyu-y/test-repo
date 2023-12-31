/*
 *
 * Copyright 2023 Intel authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	pb "github.com/intel/confidential-cloud-native-primitives/service/eventlog-server/proto"
	pkgerrors "github.com/pkg/errors"
)

var (
	InvalidRequestErr = pkgerrors.New("Invalid Request")
	/*
		    tls               = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
			certFile          = flag.String("cert_file", "", "The TLS cert file")
			keyFile           = flag.String("key_file", "", "The TLS key file")
	*/
)

const (
	RUNTIME_EVENT_LOG_DIR = "/run/ccnp-eventlog/"
	FILENAME              = "eventlog.log"
	protocol              = "unix"
	sockAddr              = "/run/eventlog-server/eventlog.sock"
)

type eventlogServer struct {
	pb.UnimplementedEventlogServer
}

func getContainerLevelEventlog(eventlogReq *pb.GetEventlogRequest) (string, error) {
	// not implemented
	return "", nil
}

func getPaasLevelEventlog(eventlogReq *pb.GetEventlogRequest) (string, error) {
	var category pb.CATEGORY
	var eventlog string
	var err error

	category = eventlogReq.EventlogCategory

	switch category {
	case pb.CATEGORY_TPM_EVENTLOG:
	case pb.CATEGORY_TDX_EVENTLOG:
	default:
		log.Println("Invalid eventlog category.")
		return "", InvalidRequestErr
	}
	return eventlog, err
}

func (*eventlogServer) GetEventlog(ctx context.Context, eventlogReq *pb.GetEventlogRequest) (*pb.GetEventlogReply, error) {
	var eventlog_level pb.LEVEL
	var eventlog string
	var nonce int32
	var err error

	eventlog_level = eventlogReq.EventlogLevel
	nonce = eventlogReq.Nonce

	switch eventlog_level {
	case pb.LEVEL_SAAS:
		eventlog, err = getContainerLevelEventlog(eventlogReq)
	case pb.LEVEL_PAAS:
		eventlog, err = getPaasLevelEventlog(eventlogReq)
	default:
		log.Println("Invalid eventlog level.")
		return &pb.GetEventlogReply{}, InvalidRequestErr
	}

	if err != nil {
		return &pb.GetEventlogReply{}, err
	}

	if _, err := os.Stat(RUNTIME_EVENT_LOG_DIR); os.IsNotExist(err) {
        err := os.MkdirAll(RUNTIME_EVENT_LOG_DIR, os.ModePerm)
        if err != nil {
            return &pb.GetEventlogReply{}, err
        }
	}

	file, err := os.Create(fmt.Sprintf("%s%s", RUNTIME_EVENT_LOG_DIR, FILENAME))
	if err != nil {
		log.Println("Error creating event log file in", RUNTIME_EVENT_LOG_DIR)
		return &pb.GetEventlogReply{}, err
	}
	defer file.Close()

	_, err = file.WriteString(eventlog)
	if err != nil {
		log.Println("Error writing event log file in", RUNTIME_EVENT_LOG_DIR+FILENAME)
		return &pb.GetEventlogReply{}, err
	}

	return &pb.GetEventlogReply{EventlogDataLoc: RUNTIME_EVENT_LOG_DIR + FILENAME, Nonce: nonce}, nil
}

func (*eventlogServer) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

func (*eventlogServer) Watch(in *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	return nil
}

func newServer() *eventlogServer {
	s := &eventlogServer{}
	return s
}

func main() {
	if _, err := os.Stat(sockAddr); !os.IsNotExist(err) {
		if err := os.RemoveAll(sockAddr); err != nil {
			log.Fatal(err)
		}
	}

	lis, err := net.Listen(protocol, sockAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	healthServer := health.NewServer()

	pb.RegisterEventlogServer(grpcServer, newServer())
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	log.Printf("server listening at %v", lis.Addr())
	reflection.Register(grpcServer)
	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
