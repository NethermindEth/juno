package utils

import "reflect"

// IsNil checks if the underlying value of the interface is nil.
//
// In Golang, an interface is boxed by an underlying type and value; both of them need to be nil for the interface to be
// nil. See the following examples:
//
//	var i any
//	fmt.Println(i == nil) // true
//
//	var p *int
//	var i any = p
//	fmt.Println(i == nil) // false!
//
// A solution for this is to use i == nil || reflect.ValueOf(i).IsNil()), however, this can cause a panic as not all
// reflect.Value has IsNil() defined. Therefore, default is to return false. For example, reflect.Array cannot be nil,
// hence calling IsNil() will cause a panic.
func IsNil(i any) bool {
	if i == nil {
		return true
	}

	v := reflect.ValueOf(i)
	k := v.Kind()

	switch k {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
