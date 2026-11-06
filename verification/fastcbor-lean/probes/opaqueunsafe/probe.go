package opaqueunsafe

import "unsafe"

type Reader func([]byte, unsafe.Pointer) error

func Cast[T any](read func([]byte, *T) error) Reader { return *(*Reader)(unsafe.Pointer(&read)) }

type Header struct {
	Data     unsafe.Pointer
	Len, Cap int
}

func HeaderOf(s *[]byte) *Header { return (*Header)(unsafe.Pointer(s)) }

type Iface struct{ Type, Data unsafe.Pointer }

func InterfaceOf(x *any) *Iface { return (*Iface)(unsafe.Pointer(x)) }
