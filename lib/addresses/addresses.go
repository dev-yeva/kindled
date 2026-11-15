package addresses

import (
	"iter"
	"log/slog"
	"net/netip"
)

type Addresses map[netip.Addr]uint16

func (a Addresses) LogValue() slog.Value {
	data := make([]netip.AddrPort, a.Size())
	for addrPort := range a.Range() {
		data = append(data, addrPort)
	}
	return slog.AnyValue(data)
}

func New(capacity int) Addresses {
	return make(map[netip.Addr]uint16, capacity)
}

func (a Addresses) Add(aps ...netip.AddrPort) {
	for _, ap := range aps {
		if _, ok := a[ap.Addr()]; !ok {
			a[ap.Addr()] = ap.Port()
		}
	}
}

func (a Addresses) Range() iter.Seq[netip.AddrPort] {
	return func(yield func(netip.AddrPort) bool) {
		for key, val := range a {
			ap := netip.AddrPortFrom(key, val)
			if !yield(ap) {
				return
			}
		}
	}
}

func (a Addresses) Size() int {
	return len(a)
}
