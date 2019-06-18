package main

import (
	"chatApp/server/proto"
	"context"
	"log"
	"net"
	"os"
	"sync"

	"google.golang.org/grpc"
	glog "google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
)

//it's help to classified log info, warning and errors
var grpcLog glog.LoggerV2

func init() {
	//This is used to init the new logger instance
	grpcLog = glog.NewLoggerV2(os.Stdout, os.Stdout, os.Stdout)
}

//Connection struct, which holds the data of connection
type Connection struct {
	stream proto.Broadcast_CreateStreamServer
	id     string
	name   string
	active bool
	error  chan error
}

//Server is used to implement service methods
//this has a connection member, holds all clients
type Server struct {
	Connection []*Connection
}

//CreateStream methods accecpts two args
//first arg is connect struct
//second arg is Broadcast_CreateStreamServer interface, it's attach a client to stream server
func (s *Server) CreateStream(pconn *proto.Connect, stream proto.Broadcast_CreateStreamServer) error {
	conn := &Connection{
		stream: stream,
		id:     pconn.User.Id,
		name:   pconn.User.Name,
		active: true,
		error:  make(chan error),
	}

	s.Connection = append(s.Connection, conn)

	return <-conn.error
}

//BroadcastMessage method forwords message to all active clients
func (s *Server) BroadcastMessage(ctx context.Context, msg *proto.Message) (*proto.Close, error) {
	wait := sync.WaitGroup{}
	done := make(chan int)

	for _, conn := range s.Connection {
		wait.Add(1)

		go func(msg *proto.Message, conn *Connection) {
			defer wait.Done()

			if conn.active {
				err := conn.stream.Send(msg)
				grpcLog.Info("sending message to user :", conn.name)

				if err != nil {
					grpcLog.Errorf("Error with Stream: %v - Error: %v", conn.stream, err)
					conn.active = false
					conn.error <- err
				}
			}
		}(msg, conn)

	}

	go func() {
		wait.Wait()
		close(done)
	}()

	<-done
	return &proto.Close{}, nil
}

func main() {
	var connections []*Connection

	server := &Server{connections}

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterBroadcastServer(s, server)
	reflection.Register(s)

	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
