package main

import (
	"github.com/ssgo/deploy/service"
	"github.com/ssgo/s"
)

type CC struct {
	HttpVersion int
}

func setSSKey(key, iv []byte) {
	service.SetEncryptKeys(key, iv)
}

func main() {
	service.Init()
	s.Start()
}
