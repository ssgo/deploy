package service

import (
	"fmt"
	"github.com/ssgo/log"
	"github.com/ssgo/u"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"
)

type ContextInfo struct {
	Desc     string
	Vars     map[string]string
	Projects map[string]*ProjectInfo
	Token    string
}

func getContexts(logger *log.Logger) []string {
	out, err := loadContexts("")
	if err != nil {
		logger.Error(err.Error())
	}
	return out
}

func loadContexts(token string) ([]string, error) {
	out := make([]string, 0)
	files, err := ioutil.ReadDir(_config.DataPath)
	if err == nil {
		for _, file := range files {
			contextName := file.Name()
			if !isValidContextName(contextName) || !file.IsDir() {
				continue
			}

			if token != "" {
				ctx := ContextInfo{}
				_ = u.Load(contextFile(contextName), &ctx)
				if EncodeToken(ctx.Token) == token {
					out = append(out, contextName)
				}
			} else {
				out = append(out, contextName)
			}
		}
	}
	return out, err
}

func getContext(in struct{ ContextName string }, logger *log.Logger) (out ContextInfo) {
	if !isValidContextName(in.ContextName) {
		return
	}
	err := u.Load(contextFile(in.ContextName), &out)
	if err != nil {
		logger.Error(err.Error())
	}
	return
}

func setContext(in struct {
	ContextInfo
	ContextName string
}, logger *log.Logger) bool {
	if !isValidContextName(in.ContextName) {
		logger.Error("bad context name", "context", in.ContextName)
		return false
	}

	if in.Token == "" {
		in.Token = u.ShortUniqueId()
	}
	if in.Projects == nil {
		in.Projects = map[string]*ProjectInfo{}
	}
	for _, proj := range in.Projects {
		if proj.Token == "" {
			proj.Token = u.ShortUniqueId()
		}
	}

	err := u.Save(contextFile(in.ContextName), &in.ContextInfo)
	ok := false
	if err == nil {
		ok = true
		err = u.Save(archivedContextFile(in.ContextName), &in.ContextInfo)
	}
	if err != nil {
		logger.Error(err.Error())
	}
	return ok
}

func removeContext(in struct{ ContextName string }, logger *log.Logger) bool {
	if !isValidContextName(in.ContextName) {
		logger.Error("bad context name", "context", in.ContextName)
		return false
	}

	err := os.Remove(contextFile(in.ContextName))
	ok := false
	if err == nil {
		ok = true
		err = os.RemoveAll(contextPath(in.ContextName))
	}
	if err != nil {
		logger.Error(err.Error())
	}
	return ok
}

func isValidContextName(contextName string) bool {
	return contextName != "" && !reservedWords[contextName] && contextName[0] != '.' && contextName[0] != '_' && strings.IndexByte(contextName, '/') == -1
}

var repositoryNameRegex, _ = regexp.Compile("[^a-zA-Z0-9]")

func loadProject(contextName, projectName string, logger *log.Logger) (*ContextInfo, *ProjectInfo) {
	if !isValidContextName(contextName) || !isValidProjectName(projectName) {
		logger.Error(fmt.Sprintln("bad context name or project name", "context:", contextName, "project:", projectName))
		return nil, nil
	}

	// 载入 ContextInfo
	ctx := ContextInfo{}
	err := u.Load(contextFile(contextName), &ctx)
	if err != nil {
		logger.Error(err.Error())
		return nil, nil
	}

	// 载入 ProjectInfo
	proj := ctx.Projects[projectName]
	return &ctx, proj
}

func checkout(repository, tag string, pull bool, clean bool) string {
	if tag == "" {
		tag = "master"
	}
	fixedRepository := repositoryNameRegex.ReplaceAllString(repository, "_")
	gitPath := dataPath("_repositories", fixedRepository)
	if clean {
		_ = os.RemoveAll(gitPath)
	}
	err := os.MkdirAll(gitPath, 0700)
	if err == nil {
		err = os.Chdir(gitPath)
	}
	if err != nil {
		logger.Error(err.Error())
		return ""
	}

	if u.FileExists(".git") {
		logger.Info("git checkout "+tag, "repository", repository, "tag", tag)
		_, err = u.RunCommand("git", "checkout", tag)
		if pull {
			logger.Info("git pull", "repository", repository, "tag", tag)
			_, err = u.RunCommand("git", "pull")
		}
	} else {
		logger.Info("git clone "+repository+" .", "repository", repository, "tag", tag)
		_, err = u.RunCommand("git", "clone", repository, ".")
		logger.Info("git checkout "+tag, "repository", repository, "tag", tag)
		_, err = u.RunCommand("git", "checkout", tag)
	}
	if err != nil {
		logger.Error(err.Error())
		return ""
	}
	return gitPath
}

func contextPath(context string) string {
	return dataPath(context)
}

func contextFile(context string) string {
	return dataPath(context, "_context")
}

func archivedContextFile(context string) string {
	t := time.Now()
	return dataPath("_archived", context, "_context", fmt.Sprintf("%.4d-%.2d-%.2d %.2d:%.2d:%.2d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()))
}
