package main

import "strconv"

type GType uint8
type GVal interface{}

const (
	GSTR  GType = 0x00
	GLIST GType = 0x01
	GSET  GType = 0x02
	GZSET GType = 0x03
	GDICT GType = 0x04
)

type GObj struct {
	Type     GType
	Val      GVal
	refCount int
}

func (o *GObj) IncrRefCount() {
	o.refCount++
}

func (o *GObj) DecrRefCount() {
	o.refCount--
	if o.refCount == 0 {
		o.Val = nil //GC
	}
}

func (o *GObj) IntVal() int64 {
	if o.Type != GSTR {
		return 0
	}
	val, _ := strconv.ParseInt(o.Val.(string), 10, 64)
	return val
}

func (o *GObj) FloatVal() float64 {
	if o.Type != GSTR {
		return 0
	}
	val, _ := strconv.ParseFloat(o.Val.(string), 64)
	return val
}

func (o *GObj) StrVal() string {
	if o.Type != GSTR {
		return ""
	}
	return o.Val.(string)
}

func (o *GObj) ListVal() *List {
	if o.Type != GLIST {
		return nil
	}
	return o.Val.(*List)
}

func (o *GObj) ZsetVal() *Zskiplist {
	if o.Type != GZSET {
		return nil
	}
	return o.Val.(*Zskiplist)
}

func CreateFromList() *GObj {
	return &GObj{
		Type:     GLIST,
		Val:      ListCreate(ListType{EqualFunc: GStrEqual}),
		refCount: 1,
	}
}

func CreateFromInt(val int64) *GObj {
	return &GObj{
		Type:     GSTR,
		Val:      strconv.FormatInt(val, 10),
		refCount: 1,
	}
}

func CreateObject(typ GType, ptr interface{}) *GObj {
	return &GObj{
		Type:     typ,
		Val:      ptr,
		refCount: 1,
	}
}
