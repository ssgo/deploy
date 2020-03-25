package service

import (
	"fmt"
	"github.com/ssgo/log"
	"github.com/ssgo/u"
	"gopkg.in/yaml.v2"
	"strings"
	"time"
)

type ProjectInfo struct {
	Desc       string
	Repository string
	Token      string
}

type CIBuild struct {
	From   string
	Cache  string
	Script []string
}

type CIDeploy struct {
	From       string
	Dockerfile []string
	Script     []string
}

type CI struct {
	//CacheTag string
	//Cache    string
	Build    []CIBuild
	Deploy   []CIDeploy
}

//func getProjects(in struct{ ContextName string }, logger *log.Logger) []ProjectInfo {
//	projects := make([]ProjectInfo, 0)
//
//	if !isValidContextName(in.ContextName) {
//		return projects
//	}
//
//	files, err := ioutil.ReadDir(contextPath(in.ContextName))
//	if err == nil {
//		for _, file := range files {
//			projectName := file.Name()
//			if !isValidProjectName(projectName) || file.IsDir() {
//				continue
//			}
//
//			project := ProjectInfo{}
//			err := u.Load(projectFile(in.ContextName, projectName), &project)
//			if err != nil {
//				logger.Error(err.Error())
//				return nil
//			}
//			projects = append(projects, project)
//		}
//	}
//	return projects
//}
//
//func setProject(in struct {
//	ProjectInfo
//	ContextName string
//	ProjectName string
//}, logger *log.Logger) bool {
//	if !isValidContextName(in.ContextName) || !isValidProjectName(in.ProjectName) {
//		logger.Error("bad context name or project name", "context", in.ContextName, "project", in.ProjectName)
//		return false
//	}
//
//	err := u.Save(projectFile(in.ContextName, in.ProjectName), &in.ProjectInfo)
//	ok := false
//	if err == nil {
//		ok = true
//		err = u.Save(archivedProjectFile(in.ContextName, in.ProjectName), &in.ProjectInfo)
//	}
//	if err != nil {
//		logger.Error(err.Error())
//	}
//	return ok
//}
//
//func removeProject(in struct{ ContextName, ProjectName string }, logger *log.Logger) bool {
//	if !isValidContextName(in.ContextName) || !isValidProjectName(in.ProjectName) {
//		logger.Error("bad context name or project name", "context", in.ContextName, "project", in.ProjectName)
//		return false
//	}
//
//	err := os.Remove(projectFile(in.ContextName, in.ProjectName))
//	if err != nil {
//		logger.Error(err.Error())
//	}
//	return err == nil
//}

func getCI(in struct{ ContextName, ProjectName string }, logger *log.Logger) string {
	s, err := u.ReadFile(ciFile(in.ContextName, in.ProjectName), 2048000)
	if err != nil {
		logger.Error(err.Error())
	}
	return s
}

func setCI(in struct {
	Ci          string
	ContextName string
	ProjectName string
}, logger *log.Logger) bool {
	if !isValidContextName(in.ContextName) || !isValidProjectName(in.ProjectName) {
		logger.Error("bad context name or project name", "context", in.ContextName, "project", in.ProjectName)
		return false
	}

	ciStr := in.Ci
	var err error

	//ci := LoadCI(in.Ci)
	//ciBytes, err = yaml.Marshal(ci)
	//if err != nil {
	//	logger.Error(err.Error())
	//} else {
	//	ciStr = string(ciBytes)
	//}

	err = u.WriteFile(ciFile(in.ContextName, in.ProjectName), ciStr)
	ok := false
	if err == nil {
		ok = true
		err = u.WriteFile(archivedCIFile(in.ContextName, in.ProjectName), ciStr)
	}
	if err != nil {
		logger.Error(err.Error())
	}
	return ok
}

func LoadCIFile(fileName string) CI {
	s, err := u.ReadFile(fileName, 204800)
	if err != nil {
		logError(err.Error())
		return CI{}
	}

	return LoadCI(s)
}

func LoadCI(s string) CI {
	m := map[string]interface{}{}
	conf := CI{}
	err := yaml.Unmarshal([]byte(s), &m)
	if err != nil {
		logError(err.Error())
	} else {
		u.Convert(m, &conf)
	}
	return conf
}

func isValidProjectName(projectName string) bool {
	return projectName != "" && !reservedWords[projectName] && projectName[0] != '.' && projectName[0] != '_' && !strings.HasSuffix(projectName, ".ci") && strings.IndexByte(projectName, '/') == -1
}

//func projectFile(context, project string) string {
//	return dataPath(context, project)
//}
//
//func archivedProjectFile(context, project string) string {
//	t := time.Now()
//	return dataPath("_archived", context, project, fmt.Sprintf("%.4d-%.2d-%.2d %.2d:%.2d:%.2d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()))
//}

func ciFile(context, project string) string {
	return dataPath(context, project, "ci.yml")
}

func archivedCIFile(context, project string) string {
	t := time.Now()
	return dataPath("_archived", context, project, "ci.yml", fmt.Sprintf("%.4d-%.2d-%.2d %.2d:%.2d:%.2d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()))
}
