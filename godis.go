package main

import (
	"log"
	"os"
)

var server GodisServer
var cmdTable []GodisCommand = []GodisCommand{
	{"quit", server.quitCommand, 1},
	{"get", server.getCommand, 2},
	{"set", server.setCommand, 3},
	{"expire", server.expireCommand, 3},
	{"command", server.commandCommand, 2},
	{"lpush", server.lpushCommand, 3},
	{"lpop", server.lpopCommand, 2},
	{"zadd", server.zaddCommand, 4},
	{"zrange", server.zrangeCommand, 0},
}

func main() {
	path := os.Args[1]

	config, err := LoadConfig(path)

	if err != nil {
		log.Printf("config error: %v\n", err)
	}

	err = server.initServer(config)
	if err != nil {
		log.Printf("init server error: %v\n", err)
	}

	server.cmd = cmdTable
	server.aeloop.AddFileEvent(server.fd, AE_READABLE, server.AcceptHandler, nil)
	server.aeloop.AddTimeEvent(AE_NORMAL, 100, server.ServerCron, nil)
	log.Println("godis server is up.")
	server.aeloop.AeMain()
}
