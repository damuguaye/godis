package main

import (
	"errors"
	"log"
	"math"
	"math/rand"
)

const (
	INIT_SIZE    int64 = 8
	FORCE_RATIO  int64 = 2
	GROW_RATIO   int64 = 2
	DEFAULT_STEP int   = 1
)

var (
	EP_ERR = errors.New("expand error")
	EX_ERR = errors.New("key exists error")
	NK_ERR = errors.New("key does not exist error")
)

type Entry struct {
	Key  *GObj
	Val  *GObj
	next *Entry
}

type htable struct {
	table []*Entry
	size  int64
	mask  int64
	used  int64
}

type DictType struct {
	HashFunc  func(key *GObj) int64
	EqualFunc func(k1, k2 *GObj) bool
}

type Dict struct {
	DictType
	hts       [2]*htable
	rehashidx int64
}

func DictCreate(dictType DictType) *Dict {
	var dict Dict
	dict.DictType = dictType
	dict.rehashidx = -1
	var ht htable
	ht.size = INIT_SIZE
	ht.mask = INIT_SIZE - 1
	ht.used = 0
	ht.table = make([]*Entry, INIT_SIZE)
	dict.hts[0] = &ht
	return &dict
}

func (dict *Dict) isRehashing() bool {
	return dict.rehashidx != -1
}

func (dict *Dict) rehash(step int) {

	for step > 0 {

		if dict.hts[0].used == 0 { //完成rehash

			dict.hts[0] = dict.hts[1]
			dict.hts[1] = nil
			dict.rehashidx = -1
			return
		}

		for dict.hts[0].table[dict.rehashidx] == nil {
			dict.rehashidx++
		}
		entry := dict.hts[0].table[dict.rehashidx]
		var nextEntry *Entry
		var idx int64
		for entry != nil {
			nextEntry = entry.next
			idx = dict.HashFunc(entry.Key) & dict.hts[1].mask
			entry.next = dict.hts[1].table[idx]
			dict.hts[1].table[idx] = entry
			dict.hts[0].used--
			dict.hts[1].used++
			entry = nextEntry
		}
		dict.hts[0].table[dict.rehashidx] = nil
		dict.rehashidx++
		step--
	}
}

func (dict *Dict) rehashStep() {
	dict.rehash(DEFAULT_STEP)
}

func nextPower(size int64) int64 {
	for i := INIT_SIZE; i < math.MaxInt64; i *= 2 {
		if i >= size {
			return i
		}
	}
	return -1
}

func (dict *Dict) expand(size int64) error {
	sz := nextPower(size)
	log.Printf("new size %d\n", sz)
	if dict.isRehashing() || (dict.hts[0] != nil && dict.hts[0].size >= sz) {
		return EP_ERR
	}

	var ht htable
	ht.size = sz
	ht.mask = sz - 1
	ht.table = make([]*Entry, sz)
	ht.used = 0

	dict.hts[1] = &ht
	dict.rehashidx = 0
	return nil
}

func (dict *Dict) expandIfNeeded() error {
	if dict.isRehashing() {
		return nil
	}
	if (dict.hts[0].used > dict.hts[0].size) && (dict.hts[0].used/dict.hts[0].size > FORCE_RATIO) {
		return dict.expand(dict.hts[0].size * GROW_RATIO)
	}
	return nil
}

// func (dict *Dict) KeyIndex(key *GObj) int64 {
// 	err := dict.expandIfNeeded()
// 	if err != nil {
// 		return -1
// 	}
// 	hk := dict.HashFunc(key)
// 	var idx int64
// 	var entry *Entry
// 	i := 1
// 	if !dict.isRehashing() {
// 		i = 0
// 	}
// 	for i >= 0 {
// 		idx = hk & dict.hts[i].mask
// 		entry = dict.hts[i].table[idx]
// 		for entry != nil {
// 			if dict.EqualFunc(entry.Key, key) {
// 				return -1 //已经有了这个Key
// 			}
// 			entry = entry.next
// 		}
// 		i--
// 	}
// 	return idx
// }

func (dict *Dict) keyIndex(key *GObj) int64 {
	err := dict.expandIfNeeded()
	if err != nil {
		return -1
	}
	h := dict.HashFunc(key)
	var idx int64
	for i := 0; i <= 1; i++ {
		idx = h & dict.hts[i].mask
		e := dict.hts[i].table[idx]
		for e != nil {
			if dict.EqualFunc(e.Key, key) {
				return -1
			}
			e = e.next
		}
		if !dict.isRehashing() { // dict is not rehashing, don't search hts[1]
			break
		}
	}
	return idx
}

func (dict *Dict) AddNew(key *GObj) *Entry {
	if dict.isRehashing() {
		dict.rehashStep()
	}
	idx := dict.keyIndex(key)
	if idx == -1 {
		return nil
	}

	ht := dict.hts[0]
	if dict.isRehashing() {
		ht = dict.hts[1]
	}

	var entry Entry
	entry.Key = key
	key.IncrRefCount()
	entry.next = ht.table[idx]
	ht.table[idx] = &entry
	ht.used++
	return &entry
}

func (dict *Dict) Add(key, val *GObj) error {
	entry := dict.AddNew(key)
	if entry == nil {
		return EX_ERR
	}
	entry.Val = val
	val.IncrRefCount()
	return nil
}

func (dict *Dict) Find(key *GObj) *Entry {
	if dict.hts[0] == nil {
		return nil
	}
	if dict.isRehashing() {
		dict.rehashStep()
	}

	hk := dict.HashFunc(key)

	i := 1
	if !dict.isRehashing() {
		i = 0
	}
	for i >= 0 {
		idx := hk & dict.hts[i].mask
		entry := dict.hts[i].table[idx]
		for entry != nil {
			if dict.EqualFunc(entry.Key, key) {
				return entry
			}
			entry = entry.next
		}
		i--
	}
	return nil
}
func (dict *Dict) FindIdx(key *GObj) *Entry {
	if dict.hts[0] == nil {
		return nil
	}
	if dict.isRehashing() {
		dict.rehashStep()
	}

	hk := dict.HashFunc(key)

	i := 1
	if !dict.isRehashing() {
		i = 0
	}
	for i >= 0 {
		idx := hk & dict.hts[i].mask
		entry := dict.hts[i].table[idx]
		if entry != nil {
			return entry
		}
		i--
	}
	return nil
}

func (dict *Dict) Set(key, val *GObj) {
	if err := dict.Add(key, val); err == nil {
		return
	}
	entry := dict.Find(key)
	entry.Val.DecrRefCount()
	entry.Val = val
	val.IncrRefCount()

}

func freeEntry(e *Entry) {
	e.Key.DecrRefCount()
	e.Val.DecrRefCount()
}

func (dict *Dict) Delete(key *GObj) error {
	if dict.hts[0] == nil {
		return NK_ERR
	}
	if dict.isRehashing() {
		dict.rehashStep()
	}

	hk := dict.HashFunc(key)
	var idx int64
	var entry *Entry
	var preEntry *Entry
	i := 1
	if !dict.isRehashing() {
		i = 0
	}
	for i >= 0 {
		idx = hk & dict.hts[i].mask
		entry = dict.hts[i].table[idx]
		for entry != nil {
			if dict.EqualFunc(entry.Key, key) {
				if preEntry != nil {
					preEntry.next = entry.next
				} else {
					dict.hts[i].table[idx] = entry.next
				}
				//entry.next = nil
				freeEntry(entry)
				return nil
			}
			preEntry = entry
			entry = entry.next
		}
		i--
	}
	return NK_ERR
}

func (dict *Dict) Get(key *GObj) *GObj {
	entry := dict.Find(key)
	if entry == nil {
		return nil
	}
	return entry.Val
}

func (dict *Dict) RandomGet() *Entry {
	if dict.hts[0] == nil {
		return nil
	}
	var sum int64
	if !dict.isRehashing() {
		sum = dict.hts[0].size
	} else {
		dict.rehashStep()
		if !dict.isRehashing() {
			sum = dict.hts[0].size
		} else {
			sum = dict.hts[0].size - dict.rehashidx + dict.hts[1].size
		}
	}
	var idx int64
	cnt := 0
	t := 0
	for cnt < 1000 {
		t = 0
		idx = rand.Int63n(sum)
		if dict.isRehashing() {
			idx += dict.rehashidx
		}
		if idx >= dict.hts[0].size {
			idx -= dict.hts[0].size
			t = 1
		}
		if dict.hts[t].table[idx] != nil {
			break
		}
		cnt++
	}
	if dict.hts[t].table[idx] == nil {
		return nil
	}
	entryLen := int64(0)
	entry := dict.hts[t].table[idx]
	for entry != nil {
		entryLen++
		entry = entry.next
	}
	entryIdx := rand.Int63n(entryLen)
	entry = dict.hts[t].table[idx]
	for i := int64(0); i < entryIdx; i++ {
		entry = entry.next
	}
	return entry
}
