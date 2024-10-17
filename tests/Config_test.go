package tests

import (
	"fmt"
	"github.com/ssgo/deploy/service"
	"github.com/ssgo/u"
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	s, _ := u.ReadFile("succeed.yml")
	ci := service.LoadCI(s)
	if len(ci.Build) == 0 || ci.Build[0].From != "local" {
		t.Fatal("Build config read failed")
	}
	if len(ci.Deploy) == 0 || len(ci.Deploy[0].Script) == 0 || !strings.HasSuffix(ci.Deploy[0].Script[0], "/stars2") {
		t.Fatal("Deploy config read failed")
	}

	a := service.PraseCommandArgs("--network=host ${discover} ${dd_key} -e 'service_accessTokens_ha8yd73h2njklda7122=1 2 \\'3\\'' -e 'service_accessTokens_h873huba12=0'")
	fmt.Println(u.JsonP(a))

	//buf := make([]byte, 100)
	//_, _ = base64.StdEncoding.Decode(buf, []byte("YZ2l2sQzVEpK/Qbpq+D0j5r9BuXHiSrZTXV18u503KaeofHRR8TqYwNk9ani3bGR5vSxp64C9776i3ynr2NqCfbzHc9X3U/0Q9yT9XRidfXZ%"))
	//out1 := u.EncryptAes(string(buf), []byte("?GQ$0K0GgLdO=f+~L68PLm$uhKr4'=tV"), []byte("VFs7@sK61cj^f?HZ"))
	//fmt.Println(out1)
	//out2 := service.MakeSSKeyCode("php", buf[0:40], buf[40:80])
	//fmt.Println(out2)
}
