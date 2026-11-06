package genericarray

func Array[F ~[4]uint64](dst *F, x uint64) { (*dst)[0] = x }
