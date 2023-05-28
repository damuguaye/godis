package main

import (
	"errors"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
	"time"
)

type CmdType = byte

const (
	COMMAND_UNKNOWN CmdType = 0x00
	COMMAND_INLINE  CmdType = 0x01
	COMMAND_BULK    CmdType = 0x02
)

const (
	GODIS_IO_BUF     int = 1024 * 16
	GODIS_MAX_BULK   int = 1024 * 4
	GODIS_MAX_INLINE int = 1024 * 4
)

type GodisDB struct {
	data   *Dict
	expire *Dict
}

type GodisClient struct {
	fd       int
	db       *GodisDB
	args     []*GObj
	reply    *List
	sentLen  int
	queryBuf []byte //以byte存储读取到的命令
	queryLen int    //读到命令长度
	cmdTy    CmdType
	bulkNum  int //有几个bulk
	bulkLen  int //单个bulk有几个byte
}

type GodisServer struct {
	fd      int
	port    int
	db      *GodisDB
	cmd     []GodisCommand
	clients map[int]*GodisClient
	aeloop  *AeLoop

	child_pid           int
	dirty               int64
	stat_rdb_saves      int64
	dirty_before_bgsave int64
	last_bgsave_try     int64 //time
}

type CommandProc func(c *GodisClient)

type GodisCommand struct {
	name  string
	proc  CommandProc
	arity int //args length
}

func (server *GodisServer) expireIfNeeded(key *GObj) {
	entry := server.db.expire.Find(key)
	if entry == nil {
		return
	}
	when := entry.Val.IntVal()
	if when > GetMsTime() {
		return //没到时间
	}
	server.db.expire.Delete(key)
	server.db.data.Delete(key)
}

func (server *GodisServer) findKeyRead(key *GObj) *GObj {
	server.expireIfNeeded(key)
	return server.db.data.Get(key)
}

func (server *GodisServer) lookupCommand(cmdStr string) *GodisCommand {
	for _, c := range server.cmd {
		if c.name == cmdStr {
			return &c
		}
	}
	return nil
}

func (server *GodisServer) AddReply(c *GodisClient, o *GObj) {
	c.reply.Append(o)
	o.IncrRefCount()
	server.aeloop.AddFileEvent(c.fd, AE_WRITABLE, server.SendReplyToClient, c)
}

func (server *GodisServer) AddReplyStr(c *GodisClient, str string) {
	o := CreateObject(GSTR, str)
	server.AddReply(c, o)
	o.DecrRefCount()
}

func freeArgs(client *GodisClient) {
	for _, v := range client.args {
		v.DecrRefCount()
	}
}

func freeReplyList(client *GodisClient) {
	for client.reply.length != 0 {
		n := client.reply.head
		client.reply.DelNode(n)
		n.val.DecrRefCount()
	}
}

func (server *GodisServer) freeClient(client *GodisClient) {
	freeArgs(client)
	delete(server.clients, client.fd)
	server.aeloop.RemoveFileEvent(client.fd, AE_READABLE)
	server.aeloop.RemoveFileEvent(client.fd, AE_WRITABLE)
	freeReplyList(client)
	Close(client.fd)
}
func resetClient(client *GodisClient) {
	freeArgs(client)
	client.cmdTy = COMMAND_UNKNOWN
	client.bulkLen = 0
	client.bulkNum = 0

}

func (server *GodisServer) ProcessCommand(c *GodisClient) {
	cmdStr := strings.ToLower(c.args[0].StrVal())
	log.Printf("process command: %v\n", cmdStr)
	// if cmdStr == "quit" {
	// 	server.freeClient(c)
	// 	return
	// }
	cmd := server.lookupCommand(cmdStr)
	if cmd == nil {
		server.AddReplyStr(c, "-ERR: unknow command\r\n")
		resetClient(c)
		return
	}
	if cmd.arity == 0 {
		goto deal
	}
	if cmd.arity != len(c.args) {
		server.AddReplyStr(c, "-ERR: wrong number of args\r\n")
		resetClient(c)
		return
	}
deal:
	cmd.proc(c)
	resetClient(c)
}

func (client *GodisClient) findLineInQuery() (int, error) {
	idx := strings.Index(string(client.queryBuf[:client.queryLen]), "\r\n")
	if idx < 0 && client.queryLen > GODIS_MAX_INLINE {
		return idx, errors.New("too big inline cmd")
	}
	return idx, nil
}

func (client *GodisClient) getNumInQuery(b, e int) (int, error) {
	num, err := strconv.Atoi(string(client.queryBuf[b:e]))
	client.queryBuf = client.queryBuf[e+2:]
	client.queryLen -= e + 2
	return num, err
}

func handleInlineBuf(client *GodisClient) (bool, error) {
	idx, err := client.findLineInQuery()
	if idx < 0 || err != nil {
		return false, err
	}
	subs := strings.Split(string(client.queryBuf[:idx]), " ")
	client.queryBuf = client.queryBuf[idx+2:]
	client.queryLen -= idx + 2
	client.args = make([]*GObj, len(subs))
	for i, v := range subs {
		client.args[i] = CreateObject(GSTR, v)
	}
	return true, nil
}

func handleBulkBuf(client *GodisClient) (bool, error) {
	// read bulk num
	if client.bulkNum == 0 {
		idx, err := client.findLineInQuery()
		if idx < 0 || err != nil {
			return false, err
		}
		bnum, err := client.getNumInQuery(1, idx)
		if err != nil {
			return false, err
		}
		if bnum == 0 {
			return true, nil
		}
		client.bulkNum = bnum
		//log.Println("BUM: ", bnum)
		client.args = make([]*GObj, bnum)
	}
	// read every bulk string
	for client.bulkNum > 0 {
		// read bulk length
		if client.bulkLen == 0 { //have not parse ${num}
			idx, err := client.findLineInQuery()
			if idx < 0 || err != nil {

				return false, err
			}
			if client.queryBuf[0] != '$' {
				return false, errors.New("expect $ for bulk length")
			}
			blen, err := client.getNumInQuery(1, idx)
			if err != nil || blen == 0 {

				return false, err
			}
			if blen > GODIS_MAX_BULK {
				return false, errors.New("too big bulk")
			}
			client.bulkLen = blen
		}

		// read bulk string
		if client.queryLen < client.bulkLen+2 {

			return false, nil
		}
		idx := client.bulkLen
		if client.queryBuf[idx] != '\r' || client.queryBuf[idx+1] != '\n' {

			return false, errors.New("expect CRLF for bulk end")
		}
		client.args[len(client.args)-client.bulkNum] = CreateObject(GSTR, string(client.queryBuf[:idx]))
		client.queryBuf = client.queryBuf[idx+2:]
		client.queryLen -= idx + 2
		client.bulkLen = 0
		client.bulkNum -= 1
	}
	return true, nil
}

func (server *GodisServer) ProcessQueryBuf(client *GodisClient) error {
	//log.Println("\033[1;33m", string(client.queryBuf[:client.queryLen]), "\033[0m")

	for client.queryLen > 0 {
		if client.cmdTy == COMMAND_UNKNOWN {
			if client.queryBuf[0] == '*' {
				client.cmdTy = COMMAND_BULK
			} else {
				client.cmdTy = COMMAND_INLINE
			}
		}
		var ok bool
		var err error
		if client.cmdTy == COMMAND_INLINE {
			ok, err = handleInlineBuf(client)
		} else if client.cmdTy == COMMAND_BULK {
			log.Println("BULKCOMMAND")
			ok, err = handleBulkBuf(client)
		} else {
			return errors.New("unknow godis command type")
		}
		if err != nil {
			return err
		}

		if ok {
			if len(client.args) == 0 {
				resetClient(client)
			} else {
				server.ProcessCommand(client)
			}
		} else {
			resetClient(client) //
			break
		}
	}
	return nil
}

func (server *GodisServer) ReadQueryFromClient(loop *AeLoop, fd int, extra interface{}) {
	client := extra.(*GodisClient)
	if len(client.queryBuf)-client.queryLen < GODIS_MAX_BULK {
		client.queryBuf = append(client.queryBuf, make([]byte, GODIS_MAX_BULK)...)
	}
	n, err := Read(fd, client.queryBuf[client.queryLen:])
	if err != nil || n == 0 {
		log.Printf("client %v read error: %v\n", n, err)
		server.freeClient(client)
		return
	}

	client.queryLen += n
	//log.Printf("read %v bytes from client: %v\n", n, client.fd)
	//log.Printf("ReadQueryFromClient, queryBuf: %v\n", string(client.queryBuf))
	err = server.ProcessQueryBuf(client)
	if err != nil {
		log.Printf("process query buf err: %v\n", err)
		server.freeClient(client)
		return
	}
}

func (server *GodisServer) SendReplyToClient(loop *AeLoop, fd int, extra interface{}) {
	client := extra.(*GodisClient)
	log.Printf("SendReplyToClient, reply len: %v\n", client.reply.Length())
	for client.reply.Length() > 0 {
		rep := client.reply.First()
		buf := []byte(rep.val.StrVal())
		bufLen := len(buf)
		if client.sentLen < bufLen {
			n, err := Write(fd, buf[client.sentLen:])
			if err != nil {
				log.Printf("send reply err: %v\n", err)
				server.freeClient(client)
				return
			}
			client.sentLen += n
			log.Printf("send %v bytes to client: %v\n", n, client.fd)
			if client.sentLen == bufLen {
				client.reply.DelNode(rep)
				rep.val.DecrRefCount()
				client.sentLen = 0
			} else {
				break
			}
		}
	}
	if client.reply.Length() == 0 {
		client.sentLen = 0
		loop.RemoveFileEvent(fd, AE_WRITABLE)
	}
}

func GStrEqual(a, b *GObj) bool {
	if a.Type != GSTR || b.Type != GSTR {
		return false
	}
	return a.StrVal() == b.StrVal()
}
func GStrLess(a, b *GObj) bool {
	if a.Type != GSTR || b.Type != GSTR {
		return false
	}
	return a.StrVal() < b.StrVal()
}

func GStrHash(key *GObj) int64 {
	if key.Type != GSTR {
		return 0
	}
	hash := fnv.New64()
	hash.Write([]byte(key.StrVal()))
	return int64(hash.Sum64())
}

func (server *GodisServer) CreateClient(fd int) *GodisClient {
	var client GodisClient
	client.fd = fd
	client.db = server.db
	client.queryBuf = make([]byte, GODIS_IO_BUF)
	client.reply = ListCreate(ListType{EqualFunc: GStrEqual})
	return &client
}

func (server *GodisServer) AcceptHandler(loop *AeLoop, fd int, extra interface{}) {
	cfd, err := Accept(fd)
	if err != nil {
		log.Printf("accept err: %v\n", err)
		return
	}
	client := server.CreateClient(cfd)
	server.clients[cfd] = client
	server.aeloop.AddFileEvent(cfd, AE_READABLE, server.ReadQueryFromClient, client)
	log.Printf("accept client, fd: %v\n", cfd)
}

const EXPIRE_CHECK_COUNT int = 100

func (server *GodisServer) ServerCron(loop *AeLoop, id int, extra interface{}) {
	for i := 0; i < EXPIRE_CHECK_COUNT; i++ {
		entry := server.db.expire.RandomGet()
		if entry == nil {
			break
		}
		if entry.Val.IntVal() < time.Now().Unix() {
			server.db.data.Delete(entry.Key)
			server.db.expire.Delete(entry.Key)
		}
	}
}

func (server *GodisServer) initServer(config *Config) error {
	server.port = config.Port
	server.clients = make(map[int]*GodisClient)
	server.db = &GodisDB{
		data:   DictCreate(DictType{HashFunc: GStrHash, EqualFunc: GStrEqual}),
		expire: DictCreate(DictType{HashFunc: GStrHash, EqualFunc: GStrEqual}),
	}
	var err error
	if server.aeloop, err = AeLoopCreate(); err != nil {
		return err
	}
	server.fd, err = TcpServer(server.port)
	return err
}
