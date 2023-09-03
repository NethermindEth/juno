package vm

import "C"
import "unsafe"

func cstring(data []byte) *C.char {
	str := unsafe.String(unsafe.SliceData(data), len(data))
	return C.CString(str)
}
