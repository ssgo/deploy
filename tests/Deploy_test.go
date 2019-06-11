package tests

import (
	"bufio"
	"fmt"
	"github.com/ssgo/config"
	"github.com/ssgo/deploy/service"
	"github.com/ssgo/httpclient"
	"github.com/ssgo/s"
	"github.com/ssgo/u"
	"os"
	"path"
	"strings"
	"testing"
)

var as2 *s.AsyncServer
var deployDataPath = os.TempDir() + "data2"
var decryptorFile = ""

func TestDeployStart(t *testing.T) {
	_ = os.RemoveAll(deployDataPath)
	_ = os.Setenv("deploy_dataPath", deployDataPath)
	_ = os.Setenv("service_listen", ":")
	_ = os.Setenv("service_httpVersion", "2")
	config.ResetConfigEnv()

	service.Init()
	service.WorkPath = "/Volumes/Star/com.isstar/ssgo/deploy"
	as2 = s.AsyncStart()
}

func TestInitCI(t *testing.T) {
	_ = os.Setenv("CGO_ENABLED", "0")
	_, _ = u.RunCommand("go", "build", "-o", "../.decryptor", "../decryptor/decryptor.go")
	wd, _ := os.Getwd()
	decryptorFile = path.Dir(wd) + "/.decryptor"

	g1Path := fmt.Sprintf("%s%c_g1", deployDataPath, os.PathSeparator)
	_ = os.MkdirAll(g1Path, 0700)
	_ = os.Chdir(g1Path)
	_, _ = u.RunCommand("git", "init")
	_ = u.WriteFile("abc.txt", "123")
	_, _ = u.RunCommand("git", "add", "abc.txt")
	_, _ = u.RunCommand("git", "commit", "-a", "-m", "first")
	_, _ = u.RunCommand("git", "tag", "v0.0.1")
	_ = u.WriteFile("abc.txt", "12345")
	_, _ = u.RunCommand("git", "commit", "-a", "-m", "add 45")

	_ = as2.Post("/global", service.GlobalInfo{
		Vars: map[string]string{
			"globalTitle": "GG",
			"flag":        "!",
			"warpVar":     "aaa\nbbb",
		},
	}, "Access-Token", service.EncodeToken("91deploy"))

	_ = as2.Post("/sskeys", s.Map{
		"aaa": "EpUwhuhSi+EfUCteh/uNVQrlqy0sYrLALHqmRnqTSg/pgISDYRrz/tAsihDKFigZPhWj3KS6gsf3HHcWQm4TKs59lJmOK9tn1KEm9y/aR29GCWlO/aqW5b31yw89m42X6c/zUXhfIBWgodHYpARW5g==",
	}, "Access-Token", service.EncodeToken("91deploy"))

	_ = as2.Post("/context/c1", service.ContextInfo{
		Vars: map[string]string{
			"globalTitle": "CC1",
			"flag":        "*",
			"check1":      "CC1",
			"check2":      "CC1*",
			"checkabc":    "12345",
		},
		Token: "C1TTT",
		Projects: map[string]*service.ProjectInfo{
			"p1": {
				Token:      "P1TTT",
				Repository: g1Path,
			},
			"p2": {
				Token:      "P2TTT",
				Repository: g1Path,
			},
		},
	}, "Access-Token", service.EncodeToken("91deploy"))

	yml1 := `
cachetag: $CONTEXT
cache: abc cache node_modules
build:
 - from: local
   script:
     - mkdir -p dist
     - mkdir -p cache
     - cp abc.txt dist/
     - sskey-go:aaa echo 'go build'
     - echo -n "$globalTitle" >> cache/stars
     - echo $(cat cache/stars) = $check1
     - test "$(cat cache/stars)" = $check1
     - cp cache/stars dist/stars

 - from: local # docker@192.168.0.61
   script:
     - echo -n "$flag" >> cache/stars
     - test "$(cat cache/stars)" = $check2
     - cp cache/stars dist/stars

deploy:
 from: local
 dockerfile:
   - FROM alpine:latest
   - ADD dist/ /opt/
   - ENTRYPOINT /opt/server
   - RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/' /etc/apk/repositories
     && apk add openssh-client
     && rm -f /var/cache/apk/*
   - HEALTHCHECK --interval=10s --timeout=3s CMD /opt/server check
 script:
   - cp dist/stars dist/stars2
   - echo -n "!!!" >> cache/stars2
   - echo "$(cat dist/abc.txt)" = {$checkABC}
   - test "$(cat dist/abc.txt)" = $checkABC
`
	_ = as2.Post("/ci/c1/p1", s.Map{"ci": yml1}, "Access-Token", service.EncodeToken("C1TTT"))

	_ = as2.Post("/ci/c1/p2", s.Map{"ci": yml1}, "Access-Token", service.EncodeToken("C1TTT"))
}

func TestDeploy(t *testing.T) {

	cli := httpclient.GetClientH2C(30000)
	cli.NoBody = true

	r := cli.Get(fmt.Sprintf("http://%s/build/c1/p1?token="+"P1TTT", as2.Addr))
	reader := bufio.NewReader(r.Response.Body)
	lastLine := ""
	for {
		lineBuf, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		lastLine = strings.TrimRight(string(lineBuf), "\r\n")
		fmt.Println(lastLine)
	}
	_ = r.Response.Body.Close()
	if lastLine != "# Done" {
		t.Error("build p1 failed", lastLine)
	}

	c1 := service.ContextInfo{}
	_ = as2.Get("/context/c1", "Access-Token", service.EncodeToken("91deploy")).To(&c1)
	c1.Vars = map[string]string{
		"globalTitle": "CC1",
		"flag":        "@",
		"check1":      "CC1*CC1",
		"check2":      "CC1*CC1@",
		"CHECKABC":    "123",
	}
	_ = as2.Post("/context/c1", c1, "Access-Token", service.EncodeToken("91deploy"))

	r = cli.Get(fmt.Sprintf("http://%s/build/c1/p2/v0.0.1?token="+"P2TTT", as2.Addr))
	reader = bufio.NewReader(r.Response.Body)
	lastLine = ""
	//r := cli.Get(fmt.Sprintf("http://%s/build/c1/p2/v0.0.1?token="+"P1TTT", as2.Addr), "Access-Token", service.EncodeToken("C1TTT"))
	//reader := bufio.NewReader(r.Response.Body)
	//lastLine := ""
	for {
		lineBuf, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		lastLine = strings.TrimRight(string(lineBuf), "\r\n")
		fmt.Println(string(lineBuf))
	}
	_ = r.Response.Body.Close()
	if lastLine != "# Done" {
		t.Error("build p2 failed", lastLine)
	}

}

func TestDeployEnd(t *testing.T) {
	as2.Stop()
	_ = os.RemoveAll(deployDataPath)
}
