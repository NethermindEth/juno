package reflection

import "reflect"

func IsZero[T any](dst *T) bool { return reflect.ValueOf(dst).Elem().IsZero() }
func Type[T any]() reflect.Type { return reflect.TypeFor[T]() }
