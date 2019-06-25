package service

import (
	"github.com/ssgo/config"
	"github.com/ssgo/s"
	"github.com/ssgo/u"
	"net/http"
	"os"
	"path"
	"time"
)

const (
	GUEST      = 0
	VIEW       = 1
	CONTEXT    = 2
	MANAGE     = 3
	SYNCSSKEYS = 7
	DEPLOY     = 8
)

var WorkPath = ""

func Init() {
	WorkPath = path.Dir(os.Args[0])

	s.SetAuthChecker(auth)
	s.Static("/", "www/")
	s.Restful(GUEST, "POST", "/login", login)

	s.Restful(VIEW, "GET", "/global", getGlobalInfo)
	s.Restful(MANAGE, "POST", "/global", setGlobalInfo)

	s.Restful(SYNCSSKEYS, "POST", "/sskeys/{token}", setSSKeys)

	s.Restful(VIEW, "GET", "/caches", getCacheList)
	s.Restful(MANAGE, "DELETE", "/cache/{cacheName}", removeCache)

	s.Restful(VIEW, "GET", "/contexts", getContexts)
	s.Restful(VIEW, "GET", "/context/{contextName}", getContext)
	s.Restful(CONTEXT, "POST", "/context/{contextName}", setContext)
	s.Restful(MANAGE, "DELETE", "/context/{contextName}", removeContext)

	s.Restful(VIEW, "GET", "/ci/{contextName}/{projectName}", getCI)
	s.Restful(VIEW, "GET", "/tags/{contextName}/{projectName}", getTags)
	s.Restful(VIEW, "GET", "/histories/{contextName}/{projectName}", getHistoryMonths)
	s.Restful(VIEW, "GET", "/histories/{contextName}/{projectName}/{month}", getHistoryBuilds)
	s.Restful(VIEW, "GET", "/history/{contextName}/{projectName}/{build}", getHistoryBuild)
	s.Restful(CONTEXT, "POST", "/ci/{contextName}/{projectName}", setCI)
	s.Register(DEPLOY, "/build/{contextName}/{projectName}", build)
	s.Register(DEPLOY, "/build/{contextName}/{projectName}/{tag}", build)
	s.Register(DEPLOY, "/update/{contextName}/{projectName}", update)

	s.RegisterWebsocket(DEPLOY, "/ws-build/{contextName}/{projectName}", nil, build, nil, nil, nil)
	s.RegisterWebsocket(DEPLOY, "/ws-build/{contextName}/{projectName}/{tag}", nil, build, nil, nil, nil)

	errs := config.LoadConfig("deploy", &_config)
	if errs != nil && len(errs) > 0 {
		for _, err := range errs {
			logError(err.Error())
		}
	}
	if _config.DataPath == "" {
		_config.DataPath = "/opt/deploy"
	}
	//if _config.AccessToken == "" {
	//	_config.AccessToken = "51deploy"
	//}
	if _config.ManageToken == "" {
		_config.ManageToken = "91deploy"
	}
	//_config.AccessToken = EncodeToken(_config.AccessToken)
	_config.encodedManageToken = EncodeToken(_config.ManageToken)

	pubKeyFile := dataPath(".ssh", "id_ecdsa.pub")
	if !u.FileExists(pubKeyFile) {
		priKeyFile := dataPath(".ssh", "id_ecdsa")
		u.CheckPath(priKeyFile)
		_, err := u.RunCommand("ssh-keygen", "-f", priKeyFile, "-t", "ecdsa", "-N", "", "-C", "ssgo/deploy")
		if err != nil {
			logError(err.Error())
		}
	}

	if !u.FileExists(globalFile()) {
		setGlobalInfo(GlobalInfo{Vars: map[string]string{}}, logger)
	}

	go startChecker()
}

func auth(authLevel int, url *string, in map[string]interface{}, request *http.Request) bool {
	token := request.Header.Get("Access-Token")
	switch authLevel {
	//case VIEW:
	//	return allowAccess(&token) || allowManage(&token) || allowContext(&token, in["contextName"])
	case VIEW:
		return allowManage(&token) || allowAnyContext(&token)
	case CONTEXT:
		return allowManage(&token) || allowContext(&token, in["contextName"])
	case MANAGE:
		return allowManage(&token)
	case DEPLOY:
		projectToken := in["token"]
		contextName := in["contextName"]
		projectName := in["projectName"]
		return allowDeploy(projectToken, contextName, projectName)
	case SYNCSSKEYS:
		sskeyToken := in["token"]
		return allowSyncSSKeys(sskeyToken)
	}
	return false
}

//func allowAccess(token *string) bool {
//	return *token != "" && *token == _config.AccessToken
//}

func allowManage(token *string) bool {
	return *token != "" && *token == _config.encodedManageToken
}

func allowAnyContext(token *string) bool {
	if ctxs, _ := loadContexts(""); len(ctxs) > 0 {
		for _, ctxName := range ctxs {
			if allowContext(token, ctxName) {
				return true
			}
		}
	}
	return false
}

func allowContext(token *string, contextName interface{}) bool {
	if contextName == nil {
		return false
	}
	if ctxName, ok := contextName.(string); ok {
		if *token != "" && ctxName != "" {
			ctx := ContextInfo{}
			_ = u.Load(contextFile(ctxName), &ctx)
			return EncodeToken(ctx.Token) == *token
		}
	}
	return false
}

func allowSyncSSKeys(token interface{}) bool {
	if token == nil {
		return false
	}
	if tokenS, ok := token.(string); ok {
		if tokenS != "" {
			glob := GlobalInfo{}
			_ = u.Load(globalFile(), &glob)
			return glob.SskeyToken == tokenS
		}
	}
	return false
}

func allowDeploy(token, contextName, projectName interface{}) bool {
	if token == nil || contextName == nil || projectName == nil {
		return false
	}
	if tokenS, ok := token.(string); ok {
		if contextNameS, ok := contextName.(string); ok {
			if projectNameS, ok := projectName.(string); ok {
				if tokenS != "" && contextNameS != "" && projectNameS != "" {
					ctx := ContextInfo{}
					_ = u.Load(contextFile(contextNameS), &ctx)
					proj := ctx.Projects[projectNameS]
					return proj.Token == tokenS
				}
			}
		}
	}
	return false
}

func login(in struct{ AccessToken string }) int {
	//if allowAccess(&in.AccessToken) {
	//	return VIEW
	//}
	if allowManage(&in.AccessToken) {
		return MANAGE
	}

	if ctxs, _ := loadContexts(in.AccessToken); len(ctxs) > 0 {
		for _, ctxName := range ctxs {
			if allowContext(&in.AccessToken, ctxName) {
				return CONTEXT
			}
		}
	}

	return 0
}

func startChecker() {
	for i := 0; i < 50; i++ {
		time.Sleep(time.Millisecond * 100)
		if s.IsRunning() {
			break
		}
	}

	for {
		checkTags()
		// per hour
		for i := 0; i < 6000; i++ {
			time.Sleep(time.Millisecond * 100)
			if !s.IsRunning() {
				break
			}
		}
	}
}

func checkTags() {
	contexts, err := loadContexts("")
	if err != nil {
		logger.Error(err.Error())
		return
	}

	for _, contextName := range contexts {
		if !s.IsRunning() {
			return
		}

		ctx := ContextInfo{}
		err := u.Load(contextFile(contextName), &ctx)
		if err != nil {
			logger.Error(err.Error())
			continue
		}

		if ctx.Projects != nil {
			for projectName, proj := range ctx.Projects {
				if !s.IsRunning() {
					return
				}

				if proj.Repository == "" {
					continue
				}

				logger.Info("updating", "context", contextName, "project", projectName, "repository", proj.Repository)
				lock(proj.Repository)
				checkout(proj.Repository, "master", true, false)
				unlock(proj.Repository)
				logger.Info("updated", "context", contextName, "project", projectName, "repository", proj.Repository)
			}
		}
	}
}
