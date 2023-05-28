package main

// import (
// 	"errors"
// 	"syscall"
// 	"time"
// )

// const (
// 	REDIS_RDB_TYPE_STRING int = 0
// 	REDIS_RDB_TYPE_LIST   int = 1
// 	REDIS_RDB_TYPE_SET    int = 2
// 	REDIS_RDB_TYPE_ZSET   int = 3
// 	REDIS_RDB_TYPE_HASH   int = 4
// )

// func (server *GodisServer) rdbSaveBackground() error {
// 	if server.child_pid != -1 {
// 		return errors.New("save error, has active child process")
// 	}

// 	server.stat_rdb_saves++
// 	server.dirty_before_bgsave = server.dirty
// 	server.last_bgsave_try = time.Now().UnixNano()
// 	pid, _, err := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
// 	if err != 0 {
// 		return errors.New("create pid faild")
// 	}
// 	childpid := int(pid)
// 	if childpid == 0 { //子进程
// 		retval := server.rdbSave()

// 	}

// }

// func (server *GodisServer) rdbSave() int {

// }
