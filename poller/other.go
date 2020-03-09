// +build !linux,!darwin,!netbsd,!freebsd,!openbsd,!dragonfly

package poller

func New(handler EventHandler) (Poller, error) {
	panic("your system does not support epoll or kqueue")
	return nil, nil
}
