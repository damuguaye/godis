package main

import (
	"fmt"
	"strings"
)

func (server *GodisServer) getCommand(c *GodisClient) {
	key := c.args[1]
	val := server.findKeyRead(key)
	if val == nil {
		server.AddReplyStr(c, "$-1\r\n")
	} else if val.Type != GSTR {
		server.AddReplyStr(c, "-ERR: wrong type\r\n")
	} else {
		str := val.StrVal()
		server.AddReplyStr(c, fmt.Sprintf("$%d\r\n%v\r\n", len(str), str))
	}
}

func (server *GodisServer) setCommand(c *GodisClient) {
	key := c.args[1]
	val := c.args[2]
	if val.Type != GSTR {
		server.AddReplyStr(c, "-ERR: wrong type\r\n")
	}
	server.db.data.Set(key, val)
	server.db.expire.Delete(key)
	server.AddReplyStr(c, "+OK\r\n")
}

func (server *GodisServer) expireCommand(c *GodisClient) {
	key := c.args[1]
	val := c.args[2]
	if val.Type != GSTR {
		//TODO: extract shared.strings
		server.AddReplyStr(c, "-ERR: wrong type\r\n")
	}
	expire := GetMsTime() + (val.IntVal() * 1000)
	expObj := CreateFromInt(expire)
	server.db.expire.Set(key, expObj)
	expObj.DecrRefCount()
	server.AddReplyStr(c, "+OK\r\n")
}

func (server *GodisServer) commandCommand(c *GodisClient) {
	//c.AddReplyStr("*3\r\n$3\r\nset\r\n$3\r\nget\r\n$6\r\nexpire\r\n")
	server.AddReplyStr(c, "+get\r\n")
	//c.AddReplyStr("+OK\r\n")
	//resetClient(c)
}

func (server *GodisServer) quitCommand(c *GodisClient) {
	server.AddReplyStr(c, "+OK\r\n")
	server.freeClient(c)
}

func (server *GodisServer) lpushCommand(c *GodisClient) {
	key := c.args[1]
	val := c.args[2]
	// if val.Type != GSTR {
	// 	server.AddReplyStr(c, "-ERR: wrong type\r\n")
	// }
	en := server.db.data.Find(key)
	if en == nil {
		en := server.db.data.AddNew(key)
		var relist List
		relist.head = nil
		relist.tail = nil
		relist.length = 0
		relist.EqualFunc = GStrEqual
		relist.LPush(val)
		val.IncrRefCount()
		en.Val = CreateObject(GLIST, &relist)
	} else {
		if en.Val.Type != GLIST {
			server.AddReplyStr(c, "-ERR: wrong type\r\n")
			return
		}
		list := en.Val.ListVal()
		list.LPush(val)
		val.IncrRefCount()
	}
	// list := en.Val.ListVal()
	// list.LPush(val)

	//server.db.expire.Delete(key)
	server.AddReplyStr(c, "+OK\r\n")
}

func (server *GodisServer) lpopCommand(c *GodisClient) {
	key := c.args[1]

	en := server.db.data.Find(key)
	if en == nil {
		server.AddReplyStr(c, "$-1\r\n")
		return
	}
	if en.Val.Type != GLIST {
		server.AddReplyStr(c, "-ERR: wrong type\r\n")
		return
	}
	list := en.Val.ListVal()
	ln := list.Lpop()
	if ln == nil {
		server.AddReplyStr(c, "$-1\r\n")
		return
	}

	str := ln.val.StrVal()
	ln.val.DecrRefCount()
	server.AddReplyStr(c, fmt.Sprintf("$%d\r\n%v\r\n", len(str), str))
}

func (server *GodisServer) zaddCommand(c *GodisClient) {
	key := c.args[1]
	score := c.args[2]
	val := c.args[3]
	// if val.Type != GSTR {
	// 	server.AddReplyStr(c, "-ERR: wrong type\r\n")
	// }
	en := server.db.data.Find(key)
	if en == nil {
		en := server.db.data.AddNew(key)

		zsl := ZslCreate(ZsetType{LessFunc: GStrLess,
			EqualFunc: GStrEqual,
		})
		zsl.ZslInsertNode(score.FloatVal(), val)
		val.IncrRefCount()
		en.Val = CreateObject(GZSET, zsl)

	} else {
		if en.Val.Type != GZSET {
			server.AddReplyStr(c, "$-1\r\n")
			return
		}
		zz := en.Val.ZsetVal()
		zz.ZslInsertNode(score.FloatVal(), val)
		val.IncrRefCount()

	}
	server.AddReplyStr(c, "+OK\r\n")
}

func (server *GodisServer) zrangeCommand(c *GodisClient) {
	if len(c.args) < 4 || len(c.args) > 5 {
		server.AddReplyStr(c, "-ERR: args too less\r\n")
		return
	}
	key := c.args[1]
	start := c.args[2]
	end := c.args[3]
	withscore := false
	if len(c.args) == 5 {
		args5 := c.args[4].StrVal()
		if strings.ToLower(args5) == "withscores" {
			withscore = true
		}
	}
	en := server.db.data.Find(key)
	if en == nil {
		server.AddReplyStr(c, "$-1\r\n")
		return
	}
	if en.Val.Type != GZSET {
		server.AddReplyStr(c, "-ERR: wrong type\r\n")
		return
	}
	zset := en.Val.ZsetVal()
	s, n := zset.FindRange(start, end)

	if s == nil || n == 0 {
		server.AddReplyStr(c, ":0\r\n")
		return
	}
	num := n
	if withscore {
		num = 2 * n
	}
	var str string
	server.AddReplyStr(c, fmt.Sprintf("*%d\r\n", num))
	for i := int64(0); i < n; i++ {
		str = s.ele.StrVal()
		server.AddReplyStr(c, fmt.Sprintf("$%d\r\n%v\r\n", len(str), str))
		if withscore {
			str = fmt.Sprintf("%.17f", s.score)
			if s.score-float64(int(s.score)) < 1e-8 {
				str = fmt.Sprintf("%d", int(s.score))
			}
			server.AddReplyStr(c, fmt.Sprintf("$%d\r\n%v\r\n", len(str), str))
		}
		s = s.zslLevel[0].next
	}

}
