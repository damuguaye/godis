package main

import (
	"log"
	"time"

	"golang.org/x/sys/unix"
)

type FeType int //file event
type TeType int //time event

const (
	AE_READABLE FeType = 1
	AE_WRITABLE FeType = 2
)

const (
	AE_NORMAL TeType = 1
	AE_ONCE   TeType = 2
)

type FileProc func(loop *AeLoop, fd int, extra interface{})
type TimeProc func(loop *AeLoop, id int, extra interface{})

type AeFileEvent struct {
	fd    int
	mask  FeType
	proc  FileProc
	extra interface{}
}

type AeTimeEvent struct {
	id       int
	mask     TeType
	when     int64
	interval int64
	proc     TimeProc
	extra    interface{}
	next     *AeTimeEvent
}

type AeLoop struct {
	FileEvents      map[int]*AeFileEvent
	TimeEvents      *AeTimeEvent
	fileEventFd     int
	timeEventNextId int
	stop            bool
}

var fe2ep [3]uint32 = [3]uint32{0, unix.EPOLLIN, unix.EPOLLOUT}

//unix.EPOLLIN 表示对应的文件描述符可以读
//unix.EPOLLOUT 表示对应的文件描述符可以写

func getFeKey(fd int, mask FeType) int {
	if mask == AE_READABLE {
		return fd
	} else {
		return fd * -1
	} //将可写事件fd * -1，从而将可读可写放在一个map中
}

func (loop *AeLoop) getEpollMask(fd int) uint32 {
	var ev uint32
	if loop.FileEvents[getFeKey(fd, AE_READABLE)] != nil {
		ev |= fe2ep[AE_READABLE]
	}
	if loop.FileEvents[getFeKey(fd, AE_WRITABLE)] != nil {
		ev |= fe2ep[AE_WRITABLE]
	}
	return ev
}

func (loop *AeLoop) AddFileEvent(fd int, mask FeType, proc FileProc, extra interface{}) {
	ev := loop.getEpollMask(fd)
	if ev&fe2ep[mask] != 0 {
		return
	}
	op := unix.EPOLL_CTL_ADD
	if ev != 0 {
		op = unix.EPOLL_CTL_MOD
	}
	ev |= fe2ep[mask]
	err := unix.EpollCtl(loop.fileEventFd, op, fd, &unix.EpollEvent{Fd: int32(fd), Events: ev})
	if err != nil {
		log.Printf("epoll ctr err: %v\n", err)
		return
	}
	var fe AeFileEvent
	fe.fd = fd
	fe.mask = mask
	fe.proc = proc
	fe.extra = extra
	loop.FileEvents[getFeKey(fd, mask)] = &fe
	log.Printf("ae add file event fd: %v, mask: %v\n", fd, mask)
}

func (loop *AeLoop) RemoveFileEvent(fd int, mask FeType) {
	op := unix.EPOLL_CTL_DEL
	ev := loop.getEpollMask(fd)
	ev &= ^fe2ep[mask]
	if ev != 0 {
		op = unix.EPOLL_CTL_MOD
	}
	err := unix.EpollCtl(loop.fileEventFd, op, fd, &unix.EpollEvent{Fd: int32(fd), Events: ev})

	if err != nil {
		log.Printf("epoll del err: %v\n", err)
	}
	loop.FileEvents[getFeKey(fd, mask)] = nil
	log.Printf("ae remove file event fd: %v, mask: %v\n", fd, mask)
}

func GetMsTime() int64 {
	return time.Now().UnixNano() / 1e6 //ms
}

func (loop *AeLoop) AddTimeEvent(mask TeType, interval int64, proc TimeProc, extra interface{}) int {
	id := loop.timeEventNextId
	loop.timeEventNextId++
	var te AeTimeEvent
	te.id = id
	te.mask = mask
	te.interval = interval
	te.when = GetMsTime() + interval
	te.proc = proc
	te.extra = extra
	te.next = loop.TimeEvents
	loop.TimeEvents = &te
	return id
}

func (loop *AeLoop) RemoveTimeEvent(id int) {
	curr := loop.TimeEvents
	var pre *AeTimeEvent
	for curr != nil {
		if curr.id == id {
			if pre == nil {
				loop.TimeEvents = curr.next
			} else {
				pre.next = curr.next
			}
			curr.next = nil
			break
		}
		pre = curr
		curr = curr.next
	}
}

func AeLoopCreate() (*AeLoop, error) {
	epollFd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &AeLoop{
		FileEvents:      make(map[int]*AeFileEvent),
		fileEventFd:     epollFd,
		timeEventNextId: 1,
		stop:            false,
	}, nil
}

func (loop *AeLoop) nearestTime() int64 {
	var nearest int64 = GetMsTime() + 1000
	curr := loop.TimeEvents
	for curr != nil {
		if curr.when < nearest {
			nearest = curr.when
		}
		curr = curr.next
	}
	return nearest
}

func (loop *AeLoop) AeWait() (tes []*AeTimeEvent, fes []*AeFileEvent) {
	timeout := loop.nearestTime() - GetMsTime()
	if timeout <= 0 {
		timeout = 10
	}
	var events [128]unix.EpollEvent
	n, _ := unix.EpollWait(loop.fileEventFd, events[:], int(timeout))

	if n > 0 {
		log.Printf("ae get %v epoll events\n", n)
	}

	for i := 0; i < n; i++ {
		if events[i].Events&unix.EPOLLIN != 0 {
			fe := loop.FileEvents[getFeKey(int(events[i].Fd), AE_READABLE)]
			if fe != nil {
				fes = append(fes, fe)
			}
		}
		if events[i].Events&unix.EPOLLOUT != 0 {
			fe := loop.FileEvents[getFeKey(int(events[i].Fd), AE_WRITABLE)]
			if fe != nil {
				fes = append(fes, fe)
			}
		}
	}
	now := GetMsTime()
	curr := loop.TimeEvents
	for curr != nil {
		if curr.when <= now {
			tes = append(tes, curr)
		}
		curr = curr.next
	}
	return
}

func (loop *AeLoop) AeProcess(tes []*AeTimeEvent, fes []*AeFileEvent) {
	for _, te := range tes {
		te.proc(loop, te.id, te.extra)
		if te.mask == AE_ONCE {
			loop.RemoveTimeEvent(te.id)
		} else {
			te.when = GetMsTime() + te.interval
		}
	}
	if len(fes) > 0 {
		//log.Printf("ae is processing file events\n")
		for _, fe := range fes {
			fe.proc(loop, fe.fd, fe.extra)
		}
	}
}

func (loop *AeLoop) AeMain() {
	for loop.stop != true {
		tes, fes := loop.AeWait()
		loop.AeProcess(tes, fes)
	}
}
