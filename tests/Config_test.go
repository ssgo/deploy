package tests

import (
	"github.com/ssgo/deploy/service"
	"github.com/ssgo/u"
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	s, _ := u.ReadFile("succeed.yml", 1024)
	ci := service.LoadCI(s)
	if len(ci.Build) == 0 || ci.Build[0].From != "local" {
		t.Fatal("Build config read failed")
	}
	if len(ci.Deploy) == 0 || len(ci.Deploy[0].Script) == 0 || !strings.HasSuffix(ci.Deploy[0].Script[0], "/stars2") {
		t.Fatal("Deploy config read failed")
	}
}
