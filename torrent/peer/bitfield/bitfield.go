package bitfield

import (
	"sync"
)

type BitField struct {
	bitfield []byte
	mu       sync.RWMutex
}

func New(length int) *BitField {
	return &BitField{bitfield: make([]byte, length)}
}

func (bf *BitField) Set(bitfield []byte) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	bf.bitfield = bitfield
}

func (bf *BitField) HasPiece(index uint32) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	byteIndex := index / 8
	offset := index % 8
	return bf.bitfield[byteIndex]>>(7-offset)&1 != 0
}

func (bf *BitField) SetPiece(index uint32) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	byteIndex := index / 8
	offset := index % 8
	bf.bitfield[byteIndex] |= 1 << (7 - offset)
}
