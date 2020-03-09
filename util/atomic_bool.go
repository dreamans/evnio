package util

import "sync/atomic"

type AtomicBool int32

func (b *AtomicBool) IsSet() bool { return atomic.LoadInt32((*int32)(b)) != 0 }
func (b *AtomicBool) Set()        { atomic.StoreInt32((*int32)(b), 1) }
