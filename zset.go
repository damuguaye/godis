package main

import (
	"math/rand"
)

const (
	ZSKIPLIST_MAXLEVEL = 32
	ZSKIPLIST_P        = 0.25
)

func zslRandomLevel() int {
	level := 1
	for (rand.Float32() < ZSKIPLIST_P) && (level < ZSKIPLIST_MAXLEVEL) {
		level++
	}
	return level
}

type ZsetType struct {
	LessFunc  func(obj1, obj2 *GObj) bool
	EqualFunc func(obj1, obj2 *GObj) bool
}

type ZslNode struct {
	ele   *GObj
	score float64
	prev  *ZslNode //level 1 前一个结点

	zslLevel []struct {
		next *ZslNode //本 level 的下一个结点
		span uint32   //到达该层级的下一个节点，实际跨越了多少个节点
	}
}

type Zskiplist struct {
	ZsetType
	head   *ZslNode
	tail   *ZslNode
	length uint32
	level  int
}

func zslCreateNode(level int, score float64, ele *GObj) *ZslNode {
	var zn ZslNode
	zn.score = score
	zn.ele = ele
	zn.zslLevel = make([]struct {
		next *ZslNode
		span uint32
	}, level)
	return &zn
}

func ZslCreate(zslType ZsetType) *Zskiplist {
	var zsl Zskiplist
	zsl.ZsetType = zslType
	zsl.level = 1
	zsl.length = 0
	zsl.head = zslCreateNode(ZSKIPLIST_MAXLEVEL, 0, nil)
	for i := 0; i < ZSKIPLIST_MAXLEVEL; i++ {
		zsl.head.zslLevel[i].next = nil
		zsl.head.zslLevel[i].span = 0
	}
	zsl.head.prev = nil
	zsl.tail = nil
	return &zsl
}

func (zsl *Zskiplist) ZslInsertNode(score float64, ele *GObj) {

	var update [ZSKIPLIST_MAXLEVEL]*ZslNode
	var rank [ZSKIPLIST_MAXLEVEL]uint32
	var i int
	curr := zsl.head
	if zsl.length == 0 {
		update[0] = zsl.head
		rank[0] = 0
		goto inser
	}
	for i = zsl.level - 1; i >= 0; i-- {
		if i == zsl.level-1 {
			rank[i] = 0
		} else {
			rank[i] = rank[i+1]
		}
		for (curr.zslLevel[i].next != nil) && //curr在update[i]和update[i].zslLevel[i].next 之间
			(curr.zslLevel[i].next.score < score || (curr.zslLevel[i].next.score == score && zsl.LessFunc(curr.zslLevel[i].next.ele, ele))) {
			rank[i] += curr.zslLevel[i].span
			curr = curr.zslLevel[i].next
		}
		update[i] = curr
	}

	if update[0].zslLevel[0].next != nil && zsl.EqualFunc(update[0].zslLevel[0].next.ele, ele) {
		return //遇到相同元素
	}
inser:
	currLevel := zslRandomLevel()
	if currLevel > zsl.level {
		for i = zsl.level; i < currLevel; i++ { //若currLevel 比最大level还大
			rank[i] = 0
			update[i] = zsl.head
			update[i].zslLevel[i].span = zsl.length

		}
		zsl.level = currLevel
	}

	curr = zslCreateNode(currLevel, score, ele)
	for i = 0; i < currLevel; i++ {
		curr.zslLevel[i].next = update[i].zslLevel[i].next //插入
		update[i].zslLevel[i].next = curr

		curr.zslLevel[i].span = update[i].zslLevel[i].span - rank[0] + rank[i]
		update[i].zslLevel[i].span = rank[0] - rank[i] + 1
	}

	for i = currLevel; i < zsl.level; i++ {
		update[i].zslLevel[i].span++
	}

	if update[0] == zsl.head {
		curr.prev = nil
	} else {
		curr.prev = update[0]
	}

	if curr.zslLevel[0].next != nil {
		curr.zslLevel[0].next.prev = curr
	} else {
		zsl.tail = curr
	}
	zsl.length++
}

func (zsl *Zskiplist) Find(score float64, val *GObj) (*ZslNode, *[]*ZslNode) {
	if score <= 0 {
		return nil, nil
	}
	var curr *ZslNode
	update := make([]*ZslNode, zsl.level)
	curr = zsl.head
	for i := zsl.level - 1; i >= 0; i-- {
		for curr.zslLevel[i].next != nil &&
			(curr.zslLevel[i].next.score < score || (curr.zslLevel[i].next.score == score && zsl.LessFunc(curr.zslLevel[i].next.ele, val))) {
			curr = curr.zslLevel[i].next
		}
		update[i] = curr
	}

	curr = curr.zslLevel[0].next

	if zsl.EqualFunc(curr.ele, val) {
		return curr, &update
	}
	return nil, nil
}
func (zsl *Zskiplist) FindRange(start, end *GObj) (s *ZslNode, n int64) {
	st := start.IntVal()
	ed := end.IntVal()
	if st >= int64(zsl.length) {
		return nil, 0
	}
	if ed >= int64(zsl.length) {
		ed = int64(zsl.length) - 1
	}
	if st < (-1 * int64(zsl.length)) {
		st = 0
	}
	if ed < (-1 * int64(zsl.length)) {
		return nil, 0
	}
	st = (st + int64(zsl.length)) % int64(zsl.length)
	ed = (ed + int64(zsl.length)) % int64(zsl.length)
	if ed < st {
		return nil, 0
	}
	var curr *ZslNode = zsl.head
	var idx int64 = -1
	for i := zsl.level - 1; i >= 0; i-- {
		for curr.zslLevel[i].next != nil &&
			(int64(curr.zslLevel[i].span)+idx <= st) {
			idx += int64(curr.zslLevel[i].span)
			curr = curr.zslLevel[i].next
		}
		if idx == st {
			return curr, ed - st + 1
		}
	}
	return curr, ed - st + 1

}
func (zsl *Zskiplist) ZslDeleteNote(curr *ZslNode, update *[]*ZslNode) {
	if curr == zsl.head {
		return
	}

	for i := 0; i < zsl.level; i++ {

		if (*update)[i].zslLevel[i].next == curr {
			(*update)[i].zslLevel[i].span += curr.zslLevel[i].span - 1
			(*update)[i].zslLevel[i].next = curr.zslLevel[i].next
		} else {
			(*update)[i].zslLevel[i].span -= 1
		}
	}
	if curr.zslLevel[0].next != nil {
		curr.zslLevel[0].next.prev = curr.prev
	} else {
		zsl.tail = curr.prev
	}
	for zsl.level > 1 && zsl.head.zslLevel[zsl.level-1].next == nil {
		zsl.level--
	}
	zsl.length--
}

func (zsl *Zskiplist) ZslDelete(score float64, val *GObj) {
	zsl.ZslDeleteNote(zsl.Find(score, val))
}
