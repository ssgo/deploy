package main

import (
	"github.com/ssgo/deploy/service"
	"github.com/ssgo/s"
)

type CC struct {
	HttpVersion int
}

func main() {
	service.Init()
	s.Start()
}
