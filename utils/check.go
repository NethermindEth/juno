package utils

import "reflect"

func IsNil(i any) bool {
	return i == nil || reflect.ValueOf(i).IsNil()
}
