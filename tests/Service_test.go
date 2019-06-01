package tests

import (
	"fmt"
	"github.com/ssgo/config"
	"github.com/ssgo/deploy/service"
	"github.com/ssgo/s"
	"github.com/ssgo/u"
	"os"
	"testing"
)

var as *s.AsyncServer
var dataPath = os.TempDir() + "data1"

func TestStart(t *testing.T) {
	_ = os.RemoveAll(dataPath)
	fmt.Println("dataPath:", dataPath)

	_ = os.Setenv("deploy_dataPath", dataPath)
	_ = os.Setenv("service_listen", ":")
	_ = os.Setenv("service_httpVersion", "2")
	config.ResetConfigEnv()

	ctxFile := fmt.Sprintf("%s%caaa%c_context", dataPath, os.PathSeparator, os.PathSeparator)
	_ = u.Save(ctxFile, service.ContextInfo{
		Token: "token-aaa",
		Desc:  "desc-aaa",
		Vars: map[string]string{
			"AA": "11",
			"BB": "22",
		},
	})

	projFile := fmt.Sprintf("%s%caaa%cp1", dataPath, os.PathSeparator, os.PathSeparator)
	_ = u.Save(projFile, service.ProjectInfo{
		Desc: "desc-p1",
	})

	service.Init()
	as = s.AsyncStart()
}

func TestGlobal(t *testing.T) {
	r := as.Get("/global")
	if r.Response.StatusCode != 403 {
		t.Fatal("auth failed", r.Response.StatusCode)
	}

	postGlobal := service.GlobalInfo{
		Vars: map[string]string{
			"aaa": "111",
			"bbb": "222",
		},
	}
	_ = as.Post("/global", postGlobal, "Access-Token", service.EncodeToken("91deploy"))

	g := struct {
		service.GlobalInfo
		PublicKey string
	}{}
	_ = as.Get("/global", "Access-Token", service.EncodeToken("91deploy")).To(&g)
	if g.Vars["aaa"] != "111" || g.PublicKey == "" {
		t.Fatal("set global failed", g)
	}
}

func TestContext(t *testing.T) {
	//r := as.Post("/context/bbb", nil, "Access-Token", service.EncodeToken("91deploy"))
	//if r.Response.StatusCode != 403 {
	//	t.Fatal("post auth failed", r.Response.StatusCode)
	//}

	_ = as.Post("/context/bbb", service.ContextInfo{
		Vars: map[string]string{
			"bbb": "2222",
			"ccc": "333",
		},
		Token: "BBB2BBB",
	}, "Access-Token", service.EncodeToken("91deploy"))

	contexts := make([]string, 0)
	_ = as.Get("/contexts", "Access-Token", service.EncodeToken("91deploy")).To(&contexts)
	if len(contexts) < 2 || contexts[0] != "aaa" || contexts[1] != "bbb" {
		t.Fatal("load contexts failed", contexts)
	}

	ctx := service.ContextInfo{}
	_ = as.Get("/context/aaa", "Access-Token", service.EncodeToken("token-aaa")).To(&ctx)
	if ctx.Token != "token-aaa" {
		t.Fatal("get context failed", ctx)
	}

	ctx.Token = "token-AAA"
	ctx.Projects = map[string]*service.ProjectInfo{
		"p1": {
			Desc: "Project 1",
		},
	}

	_ = as.Post("/context/aaa", ctx, "Access-Token", service.EncodeToken("token-aaa"))

	ctx = service.ContextInfo{}
	_ = as.Get("/context/aaa", "Access-Token", service.EncodeToken("91deploy")).To(&ctx)
	if ctx.Token != "token-AAA" || ctx.Projects["p1"].Desc != "Project 1" {
		t.Fatal("get changed context failed", ctx)
	}

	r := as.Delete("/context/aaa", nil, "Access-Token", service.EncodeToken("token-AAA"))
	if r.Response.StatusCode != 403 {
		t.Fatal("delete aaa auth failed", r.Response.StatusCode)
	}

	r = as.Delete("/context/aaa", nil, "Access-Token", service.EncodeToken("91deploy"))
	if r.Response.StatusCode != 200 {
		t.Fatal("delete aaa failed", r.Response.StatusCode)
	}

	ctx = service.ContextInfo{}
	_ = as.Get("/context/aaa", "Access-Token", service.EncodeToken("91deploy")).To(&ctx)
	if ctx.Token != "" {
		t.Fatal("review context aaa failed", ctx)
	}

	contexts = make([]string, 0)
	_ = as.Get("/contexts", "Access-Token", service.EncodeToken("91deploy")).To(&contexts)
	if len(contexts) > 1 || contexts[0] == "aaa" {
		t.Fatal("load contexts failed", contexts)
	}
}

//func TestProject(t *testing.T) {
//	r := as.Post("/bbb/p1", service.ProjectInfo{
//		Vars: map[string]string{
//			"aaa": "111",
//			"bbb": "222",
//			"ccc": "333",
//		},
//		Desc: "BBB Project",
//	}, "Access-Token", service.EncodeToken("BBB2BBB"))
//	if r.Response.StatusCode != 200 || r.String() != "true" {
//		t.Fatal("post project failed", r)
//	}
//
//	projects := make([]service.ProjectInfo, 0)
//	_ = as.Get("/bbb/projects", "Access-Token", service.EncodeToken("BBB2BBB")).To(&projects)
//	if len(projects) != 1 || len(projects[0].Vars) != 3 || projects[0].Vars["ccc"] != "333" {
//		t.Fatal("get p1 failed", projects)
//	}
//
//	r = as.Delete("/bbb/p1", nil, "Access-Token", service.EncodeToken("BBB2BBB"))
//	if r.Response.StatusCode != 200 || r.String() != "true" {
//		t.Fatal("delete p1 failed", r.Response.StatusCode, r.String())
//	}
//
//	projects = make([]service.ProjectInfo, 0)
//	_ = as.Get("/bbb/projects", "Access-Token", service.EncodeToken("BBB2BBB")).To(&projects)
//	if len(projects) != 0 {
//		t.Fatal("review p1 failed", projects)
//	}
//
//	r = as.Post("/bbb/p2", service.ProjectInfo{
//		Vars: map[string]string{
//			"aaa": "111",
//			"bbb": "222",
//			"ccc": "333",
//		},
//		Desc: "BBB Project",
//	}, "Access-Token", service.EncodeToken("BBB2BBB"))
//	if r.Response.StatusCode != 200 || r.String() != "true" {
//		t.Fatal("post project failed", r)
//	}
//}

func TestEnd(t *testing.T) {
	as.Stop()
	_ = os.RemoveAll(dataPath)
}
