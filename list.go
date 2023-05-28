package main

import (
	"fmt"
)

type ListNode struct {
	val  *GObj
	next *ListNode
	prev *ListNode
}

type ListType struct {
	EqualFunc func(a, b *GObj) bool
}

type List struct {
	ListType
	head   *ListNode
	tail   *ListNode
	length int
}

func ListCreate(listType ListType) *List {
	var list List
	list.ListType = listType
	return &list
}
func ReListCreate(listType ListType) List {
	var list List
	list.ListType = listType
	return list
}

func (list *List) Length() int {
	return list.length
}

func (list *List) First() *ListNode {
	return list.head
}

func (list *List) PrintList() {
	fmt.Println("begin", list.Length())
	node := list.First()
	var i int = 0
	for node != nil {
		i++
		fmt.Println(i)
		if node.val != nil {
			fmt.Println(node.val.Type)
			if node.val.Val == nil {
				fmt.Println("NULL")
			}

		}

		node = node.next
	}
}

func (list *List) Last() *ListNode {
	return list.tail
}

func (list *List) Find(val *GObj) *ListNode {
	curr := list.head
	for curr != nil {
		if list.EqualFunc(curr.val, val) {
			break
		}
		curr = curr.next
	}
	return curr
}

func (list *List) Append(val *GObj) {
	var ln ListNode
	ln.val = val
	if list.head == nil {
		list.head = &ln
		list.tail = &ln
	} else {
		ln.prev = list.tail
		list.tail.next = &ln
		list.tail = list.tail.next
	}
	list.length++
}

func (list *List) LPush(val *GObj) {
	var ln ListNode
	ln.val = val
	if list.head == nil {
		list.head = &ln
		list.tail = &ln
	} else {
		ln.next = list.head
		list.head.prev = &ln
		list.head = &ln
	}
	list.length++
}

func (list *List) Lpop() *ListNode {
	var retLN *ListNode
	if list.length == 0 {
		return nil
	} else if list.length == 1 {
		retLN = list.head
		retLN.next = nil
		retLN.prev = nil
		list.head = nil
		list.tail = nil
	} else {
		retLN = list.head
		list.head = retLN.next
		list.head.prev = nil
		retLN.next = nil
	}
	list.length--
	return retLN
}

func (list *List) DelNode(ln *ListNode) {
	if ln == nil {
		return
	}
	if list.head == ln {
		if ln.next != nil {
			ln.next.prev = nil
		}
		list.head = ln.next
		ln.next = nil
	} else if list.tail == ln {
		if ln.prev != nil {
			ln.prev.next = nil
		}
		list.tail = ln.prev
		ln.prev = nil
	} else {
		if ln.prev != nil {
			ln.prev.next = ln.next
		}
		if ln.next != nil {
			ln.next.prev = ln.prev
		}
		ln.prev = nil
		ln.next = nil
	}
	list.length--
}

func (list *List) Delete(val *GObj) {
	list.DelNode(list.Find(val))
}
