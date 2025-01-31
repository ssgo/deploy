package service

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/ssgo/log"
	"github.com/ssgo/s"
	"github.com/ssgo/tool/sskey/sskeylib"
	"github.com/ssgo/u"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

var gitLocks = map[string]*sync.Mutex{}

var projectContainerPath = "/build"

func lock(name string) {
	if gitLocks[name] == nil {
		gitLocks[name] = new(sync.Mutex)
	}
	gitLocks[name].Lock()
}

func unlock(name string) {
	gitLocks[name].Unlock()
}

type getTagsOut struct {
	Tags       []string
	CustomTags string
}

func getTags(in struct {
	ContextName string
	ProjectName string
	Clean       bool
}, logger *log.Logger) getTagsOut {
	out := getTagsOut{}
	out.Tags = make([]string, 0)

	ctx, proj := loadProject(in.ContextName, in.ProjectName, logger)
	if ctx == nil || proj == nil {
		return out
	}

	var gitPath string
	var gitTags []string
	var err error
	if proj.Repository != "" {
		out.Tags = append(out.Tags, "master")
		lock(proj.Repository)
		gitPath = checkout(proj.Repository, "master", true, in.Clean, nil)
		gitTags, err = u.RunCommand("git", "-C", gitPath, "tag", "-l", "--sort=-taggerdate")
		unlock(proj.Repository)
	}

	if err != nil {
		logger.Error(err.Error())
		return out
	}

	if gitPath != "" && len(out.Tags) > 0 {
		routs := make([]string, 0)
		tagErr := ""
		lenGitTags := len(gitTags)
		for j := 0; j < lenGitTags; j++ {
			gitTag := strings.TrimSpace(gitTags[j])
			if len(gitTag) < 1 {
				continue
			}
			if strings.Index(gitTag, " ") != -1 {
				tagErr += gitTag + "   "
				continue
			}
			if strings.Index(gitTag, "\t") != -1 {
				tagErr = gitTag + "   "
				continue
			}

			routs = append(routs, gitTag)
		}
		if tagErr != "" {
			logger.Error("out.Tags err:" + tagErr)
		}
		out.Tags = append(out.Tags, routs...)
	}

	customTags := make([]string, 0)
	u.Load(dataPath(in.ContextName, in.ProjectName, "custom_tags.json"), &customTags)
	if len(customTags) > 0 {
		out.Tags = append(out.Tags, customTags...)
	}

	out.CustomTags = strings.Join(customTags, ",")

	return out
}

func saveCustomTags(in struct {
	ContextName string
	ProjectName string
	CustomTags  string
}, logger *log.Logger) bool {
	tags := strings.Split(in.CustomTags, ",")
	customTags := make([]string, 0)
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		logger.Info("1", "tag", tag)
		if tag == "" {
			continue
		}
		customTags = append(customTags, tag)
	}
	u.Save(dataPath(in.ContextName, in.ProjectName, "custom_tags.json"), customTags)
	return true
}

func getHistoryMonths(in struct {
	ContextName string
	ProjectName string
}, logger *log.Logger) []string {
	outs := make([]string, 0)

	files, err := ioutil.ReadDir(dataPath(in.ContextName, in.ProjectName, "builds"))
	if err == nil {
		for _, file := range files {
			fileName := file.Name()
			if len(fileName) == 7 {
				outs = append(outs, fileName)
			}
		}
	} else {
		logger.Error(err.Error())
	}

	routs := make([]string, 0)
	for j := len(outs) - 1; j >= 0; j-- {
		routs = append(routs, outs[j])
	}
	return routs
}

func getHistoryBuilds(in struct {
	ContextName string
	ProjectName string
	Month       string
}, logger *log.Logger) []string {
	outs := make([]string, 0)

	files, err := ioutil.ReadDir(dataPath(in.ContextName, in.ProjectName, "builds", in.Month))
	if err == nil {
		for _, file := range files {
			fileName := file.Name()
			if len(fileName) == 21 {
				outs = append(outs, fileName)
			}
		}
	} else {
		logger.Error(err.Error())
	}

	routs := make([]string, 0)
	for j := len(outs) - 1; j >= 0; j-- {
		routs = append(routs, outs[j])
	}
	return routs
}

func getHistoryBuild(in struct {
	ContextName string
	ProjectName string
	Build       string
}, logger *log.Logger) string {
	if len(in.Build) != 21 {
		return ""
	}

	str, err := u.ReadFile(dataPath(in.ContextName, in.ProjectName, "builds", in.Build[0:7], in.Build))
	if err != nil {
		logger.Error(err.Error())
	}
	return str
}

func update(in struct {
	ContextName string
	ProjectName string
	Clean       bool
}, logger *log.Logger) bool {
	ctx, proj := loadProject(in.ContextName, in.ProjectName, logger)
	if ctx == nil || proj == nil {
		return false
	}

	lock(proj.Repository)
	gitPath := checkout(proj.Repository, "master", true, in.Clean, nil)
	unlock(proj.Repository)

	return gitPath != ""
}

func loadDeployInfo(contextName, projectName, tag string, logger *log.Logger) (map[string]string, *ProjectInfo, *CI) {
	ctx, proj := loadProject(contextName, projectName, logger)
	if ctx == nil || proj == nil {
		return nil, nil, nil
	}

	// 载入 Global
	glob := GlobalInfo{}
	err := u.Load(globalFile(), &glob)
	if err != nil {
		logger.Error("failed to load global info\n")
		return nil, nil, nil
	}
	vars := glob.Vars
	if vars == nil {
		vars = map[string]string{}
	}

	if ctx.Vars != nil {
		for k, v := range ctx.Vars {
			vars[k] = v
		}
	}

	vars["CONTEXT"] = contextName
	vars["PROJECT"] = projectName
	vars["TAG"] = tag

	ciStr, err := u.ReadFile(ciFile(contextName, projectName))
	if err != nil {
		logger.Error(err.Error())
		return nil, nil, nil
	}

	for k, v := range vars {
		ciStr = replaceVar(ciStr, k, v)
	}

	ci := LoadCI(ciStr)
	if len(ci.Build) == 0 && len(ci.Deploy) == 0 {
		logger.Error("no build and deploy")
		return nil, nil, nil
	}

	return vars, proj, &ci
}

func build(in struct {
	ContextName string
	ProjectName string
	Token       string
	Tag         string
	Ref         string
	Repository  struct {
		Git_ssh_url string
	}
	Project struct {
		Name      string
		Namespace string
	}
}, logger *log.Logger, response *s.Response, conn *websocket.Conn) {
	startTime := time.Now()

	der := Deployer{
		logger:   logger,
		response: response,
		conn:     conn,
	}

	if in.Tag == "" && in.Ref != "" {
		in.Tag = strings.ReplaceAll(in.Ref, "refs/tags/", "")
		in.Tag = strings.ReplaceAll(in.Tag, "refs/heads/", "")
		in.Tag = strings.ReplaceAll(in.Tag, "/", "_")
	}

	der.outs = make([]string, 0)
	succeed := false
	defer func() {
		endTime := time.Now()
		der.Info("# End", endTime.Format("2006-01-02 15:04:05"), "Used:", endTime.Unix()-startTime.Unix())
		der.Info(u.StringIf(succeed, "# Done", "# Failed"))
		if conn != nil {
			_ = conn.Close()
		}
		_ = u.WriteFile(buildLogFile(in.ContextName, in.ProjectName, succeed), strings.Join(der.outs, "\n"))
	}()

	vars, proj, ci := loadDeployInfo(in.ContextName, in.ProjectName, in.Tag, logger)
	if proj == nil {
		return
	}

	if proj.Repository == "" {
		// 尝试从 gitlab、github 的请求中获取仓库地址
		proj.Repository = in.Repository.Git_ssh_url
	}

	vars["GIT_PROJECT_NAME"] = in.Project.Name
	vars["GIT_PROJECT_NAMESPACE"] = in.Project.Namespace
	if vars["GIT_PROJECT_NAME"] == "" {
		a := strings.Split(proj.Repository, "/")
		if len(a) > 2 {
			vars["GIT_PROJECT_NAMESPACE"] = a[len(a)-2]
			vars["GIT_PROJECT_NAME"] = strings.Replace(a[len(a)-1], ".git", "", 1)
		}
	}

	//cacheTagValue := strings.ReplaceAll(ci.CacheTag, "$CONTEXT", in.ContextName)
	//cacheTagValue = strings.ReplaceAll(cacheTagValue, "$PROJECT", in.ProjectName)
	//cacheTagValue = strings.ReplaceAll(cacheTagValue, "$TAG", in.Tag)

	der.Info("Project", in.ContextName+":"+in.ProjectName, "@", proj.Repository, proj.Desc)
	der.Info("Script", u.JsonP(ci))
	der.Info("Vars", u.JsonP(ci))
	der.Info("# Start", startTime.Format("2006-01-02 15:04:05"))

	//// 检查敏感内容
	//if strings.Index(u.Json(vars)+u.Json(ci), ".poo_info_a") != -1 {
	//	der.Error("has sensitive info")
	//	return
	//}

	// 初始化 build 目录
	buildId := u.UniqueId()
	buildPath := dataPath("_builders", buildId)
	//u.CheckPath(buildPath)
	der.Info("# mkdir -p", buildPath)
	err := os.MkdirAll(buildPath, 0700)
	if err != nil {
		der.Error(err.Error())
		return
	}

	_ = os.Chdir(WorkPath)
	decrptFile := ".decryptor"
	if u.FileExists(decrptFile) {
		_, err = SimpleRun("cp", decrptFile, buildPath)
		if err != nil {
			der.Error("add decryptor failed", err.Error())
			return
		}
		_, err = SimpleRun("chmod", "+x", path.Join(buildPath, decrptFile))
		if err != nil {
			der.Error("add privilege for decryptor failed", err.Error())
			return
		}
	}

	vars["BUILD_PATH"] = buildPath

	// 拉去代码
	if proj.Repository != "" {
		lock(proj.Repository)
		gitPath := checkout(proj.Repository, in.Tag, true, false, nil)
		//der.Info("cp -r " + gitPath + "/* " + buildPath)
		//err = der.Run("cp", "-r", path.Join(gitPath, "/*"), buildPath)
		// exec.Command not support cp -r xxx/* xxxx
		files, err := ioutil.ReadDir(gitPath)
		if err == nil {
			for _, file := range files {
				fileName := file.Name()
				if fileName == "." || fileName == ".." || fileName == ".git" {
					continue
				}
				_, err = SimpleRun("cp", "-r", path.Join(gitPath, fileName), buildPath)
				if err != nil {
					der.Error("cp -r failed", err.Error())
					unlock(proj.Repository)
					return
				}
			}
		}
		unlock(proj.Repository)
		if err != nil {
			return
		}
	}

	_ = os.Chdir(buildPath)

	endTime := time.Now()
	der.Info("# Git clone done", endTime.Format("2006-01-02 15:04:05"), "used", endTime.Unix()-startTime.Unix(), "seconds")
	der.Info()
	var mounts []string

	shellFile := der.makeGetShellFile()
	if shellFile == "" {
		return
	}

	defer func() {
		if !_config.KeepBuildPath {
			_ = os.RemoveAll(buildPath)
		}
	}()

	// 构建
	for i, b := range ci.Build {
		// 创建脚本
		startTime := time.Now()
		der.Info("# Build", i, "from", b.From, startTime.Format("2006-01-02 15:04:05"))

		buildFile := makeScriptFile(vars, i, b.Script, &der, "build")
		dockerBuildFile := der.makeDockerBuildFile(buildFile)
		if buildFile == "" || dockerBuildFile == "" {
			return
		}

		// 初始化 cache
		cachedPaths := make([]string, 0)
		cachePaths := make([]string, 0)
		if b.Cache != "" {
			caches := strings.Split(b.Cache, " ")
			for _, cacheStr := range caches {
				if cacheStr == "" {
					continue
				}

				cachedPath := ""
				cachePath := ""
				if len(cacheStr) > 1 && cacheStr[0] == '^' {
					if len(cacheStr) > 2 && cacheStr[1] == '^' {
						cachePath = cacheStr[2:]
						cachedPath = dataPath("_caches", strings.ReplaceAll(cacheStr, "/", "_"))
					} else {
						cachePath = cacheStr[1:]
						cachedPath = dataPath("_caches", in.ContextName+"/"+strings.ReplaceAll(cachePath, "/", "_"))
					}
				} else {
					cachePath = cacheStr
					cachedPath = dataPath("_caches", in.ContextName+"-"+in.ProjectName+"/"+strings.ReplaceAll(cachePath, "/", "_"))
				}
				if !u.FileExists(cachedPath) {
					_ = os.MkdirAll(cachedPath, 0700)
				}
				cachedPaths = append(cachedPaths, cachedPath)
				cachePaths = append(cachePaths, cachePath)
			}
		}

		if b.From == "" || b.From == "local" {
			// 从本地构建
			for cacheIndex := range cachePaths {
				cacheTargetPath := fmt.Sprintf("%s%c%s", buildPath, os.PathSeparator, strings.ReplaceAll(cachePaths[cacheIndex], "/", "_"))
				if !u.FileExists(cacheTargetPath) {
					der.Run("ln", "-s", cachedPaths[cacheIndex], cacheTargetPath)
				}
			}

			shell, err := SimpleRun("sh", shellFile)
			if err != nil {
				der.Error(err.Error())
				return
			}
			if shell == "" || der.Run(shell, buildFile) != nil {
				return
			}
		} else if strings.IndexByte(b.From, '@') != -1 {
			// 从远端构建
			for cacheIndex := range cachePaths {
				der.Run("cp", "-r", cachedPaths[cacheIndex], fmt.Sprintf("%s%c%s", buildPath, os.PathSeparator, strings.ReplaceAll(cachePaths[cacheIndex], "/", "_")))
			}
			if der.BuildBySSH(b.From, buildId, shellFile, buildFile) == false {
				return
			}
			for cacheIndex := range cachePaths {
				der.Run("cp", "-r", fmt.Sprintf("%s%c%s", buildPath, os.PathSeparator, strings.ReplaceAll(cachePaths[cacheIndex], "/", "_")), cachedPaths[cacheIndex])
			}
		} else {
			// 从Docker构建
			for cacheIndex := range cachePaths {
				cacheTargetPath := cachePaths[cacheIndex]
				if cachePaths[cacheIndex][0] != '/' {
					cacheTargetPath = projectContainerPath + "/" + cachePaths[cacheIndex]
				}
				mounts = append(mounts, "-v", cachedPaths[cacheIndex]+":"+cacheTargetPath)
			}
			if !der.BuildByDocker(b.From, buildPath, dockerBuildFile, dataPath("_caches"), mounts) {
				return
			}
		}

		endTime := time.Now()
		der.Info("# Build", i, "done", endTime.Format("2006-01-02 15:04:05"), "used", endTime.Unix()-startTime.Unix(), "seconds")
		der.Info()
	}

	// zoneinfo for alpine
	if len(ci.Deploy) > 0 {
		if u.FileExists("/usr/share/zoneinfo") {
			_ = der.Run("cp", "-r", "/usr/share/zoneinfo", path.Join(buildPath, ".zoneinfo"))
		}
	}

	// 部署
	for i, d := range ci.Deploy {
		startTime := time.Now()
		der.Info("# Deploy", i, "from", d.From, startTime.Format("2006-01-02 15:04:05"))

		// 创建 Dockerfile
		if len(d.Dockerfile) > 0 {
			err := u.WriteFile("Dockerfile", strings.Join(d.Dockerfile, "\n"))
			if err != nil {
				der.Error("Make Dockerfile failed", err.Error())
				return
			}
		}

		// 创建脚本
		buildFile := makeScriptFile(vars, i, d.Script, &der, "deploy")
		dockerBuildFile := der.makeDockerBuildFile(buildFile)
		if buildFile == "" || dockerBuildFile == "" {
			return
		}

		if d.From == "" || d.From == "local" {
			// 从本地构建
			shell, err := SimpleRun("sh", shellFile)
			if err != nil {
				der.Error("sh", shellFile, "failed", err.Error())
				return
			}
			if shell == "" || der.Run(shell, buildFile) != nil {
				return
			}
		} else if strings.IndexByte(d.From, '@') != -1 {
			// 从远端构建
			if !der.BuildBySSH(d.From, buildId, shellFile, buildFile) {
				return
			}
		} else {
			// 从Docker构建
			//args := append(make([]string, 0), "run", "--rm", "-v", buildPath+":/opt")
			//args = append(args, PraseCommandArgs(d.From)...)
			//args = append(args, "sh", "/opt/"+dockerBuildFile)
			//if der.Run("docker", args...) != nil {
			//	return
			//}
			var mounts []string
			if !der.BuildByDocker(d.From, buildPath, dockerBuildFile, dataPath("_caches"), mounts) {
				return
			}
		}

		endTime := time.Now()
		der.Info("# Deploy", i, "done", endTime.Format("2006-01-02 15:04:05"), "used", endTime.Unix()-startTime.Unix(), "seconds")
		der.Info()
	}

	succeed = true
}

func checkout(repository, tag string, pull bool, clean bool, der *Deployer) string {
	if tag == "" {
		tag = "master"
	}
	fixedRepository := repositoryNameRegex.ReplaceAllString(repository, "_")
	gitPath := dataPath("_repositories", fixedRepository)
	if clean {
		_ = os.RemoveAll(gitPath)
	}
	err := os.MkdirAll(gitPath, 0700)
	//if err == nil {
	//	err = os.Chdir(gitPath)
	//}
	if err != nil {
		if der != nil {
			der.Error(err.Error())
		} else {
			logger.Error(err.Error())
		}
		return ""
	}

	if u.FileExists(path.Join(gitPath, ".git")) {
		if der != nil {
			der.Info("git checkout "+tag, "repository", repository, "tag", tag)
		} else {
			logger.Info("git checkout "+tag, "repository", repository, "tag", tag)
		}
		_, err = u.RunCommand("git", "-C", gitPath, "checkout", tag)
		if pull {
			if der != nil {
				der.Info("git pull", "repository", repository, "tag", tag)
			} else {
				logger.Info("git pull", "repository", repository, "tag", tag)
			}
			_, err = u.RunCommand("git", "-C", gitPath, "pull")
			_, err = u.RunCommand("git", "-C", gitPath, "fetch -t -p -f")
		}
	} else {
		if der != nil {
			der.Info("git pull", "repository", repository, "tag", tag)
			der.Info("git checkout "+tag, "repository", repository, "tag", tag)
		} else {
			logger.Info("git clone "+repository+" .", "repository", repository, "tag", tag)
			logger.Info("git checkout "+tag, "repository", repository, "tag", tag)
		}
		_, err = u.RunCommand("git", "-C", gitPath, "clone", repository, ".")
		_, err = u.RunCommand("git", "-C", gitPath, "checkout", tag)
	}
	if err != nil {
		if der != nil {
			der.Error(err.Error())
		} else {
			logger.Error(err.Error())
		}
		return ""
	}
	return gitPath
}

//func (der *Deployer) makeSSHBuildFile(buildId, buildScript string) string {
//	// 创建脚本
//	scripts := "cd " + buildId + "\n$(sh _getShell.sh) " + buildScript
//	err := u.WriteFile("_sshBuild.sh", scripts)
//	if err != nil {
//		der.Error(err.Error())
//		return ""
//	}
//	return "_sshBuild.sh"
//}

func (der *Deployer) makeDockerBuildFile(buildScript string) string {
	// 创建脚本
	scripts := "cd " + projectContainerPath + "\n$(sh ._getShell.sh) " + buildScript
	err := u.WriteFile("._dockerBuild.sh", scripts)
	if err != nil {
		der.Error(err.Error())
		return ""
	}
	return "._dockerBuild.sh"
}

func (der *Deployer) makeGetShellFile() string {
	// 创建脚本
	scripts := `
if [ -f /bin/bash ]; then
        echo /bin/bash
elif [ -f /bin/ash ]; then
        echo /bin/ash
elif [ -f /bin/zsh ]; then
        echo /bin/zsh
else
        echo /bin/sh
fi
`
	err := u.WriteFile("._getShell.sh", scripts)
	if err != nil {
		der.Error(err.Error())
		return ""
	}
	return "._getShell.sh"
}

func makeScriptFile(vars map[string]string, i int, buildCommands []string, der *Deployer, stage string) string {
	// 创建脚本
	scripts := make([]string, 0)
	for k, v := range vars {
		if strings.IndexByte(v, '\n') != -1 {
			continue
		}
		line := fmt.Sprintf("export %s='%s'", k, strings.ReplaceAll(v, "'", "\\\\'"))
		scripts = append(scripts, "echo '$ "+line+"'")
		scripts = append(scripts, line)
	}

	for _, line := range buildCommands {
		//printLine := line
		//if strings.HasPrefix(line, "scp ") || strings.HasPrefix(line, "ssh ") {
		//	newLine := line[0:3]
		//	if strings.Index(line, " -i ") == -1 {
		//		newLine += fmt.Sprint(" -i ", dataPath(".ssh", "id_ecdsa"))
		//	}
		//	if strings.Index(line, "StrictHostKeyChecking") == -1 {
		//		newLine += fmt.Sprint(" -o StrictHostKeyChecking=no")
		//	}
		//	line = newLine + line[3:]
		//	printLine = strings.ReplaceAll(line, dataPath(".ssh", "id_ecdsa"), "****")
		//}

		sskeyFile := ""
		if strings.HasPrefix(line, "sskey-") {
			pos := strings.IndexByte(line, ' ')
			if pos >= 0 {
				langName := line[6:pos]
				line = line[pos+1:]

				if strings.Index(line, "cp ") != -1 || strings.Index(line, "mv ") != -1 {
					der.Error("not allow to use cp、scp、mv with sskey")
					return ""
				}

				sskeyInfo := strings.Split(langName, ":")
				keyName := ""
				if len(sskeyInfo) > 1 {
					langName = sskeyInfo[0]
					keyName = sskeyInfo[1]
				}
				if len(sskeyInfo) > 2 {
					sskeyFile = sskeyInfo[2]
				}

				if sskeyFile == "" || sskeyFile[len(sskeyFile)-1] == '/' {
					switch langName {
					case "go":
						sskeyFile += u.UniqueId() + ".go"
					case "php":
						sskeyFile += "sskeyStarter.php"
					case "java":
						sskeyFile += "SSKeyStarter.java"
					}
				}

				if sskeyFile != "" {
					sskeys := map[string]string{}
					err := u.Load(sskeysFile(), &sskeys)
					if err == nil {
						if sskeys[keyName] == "" {
							err = errors.New("sskey not exists: " + keyName)
						}

						if err == nil {
							keyData := u.DecryptAes(sskeys[keyName], settedKey, settedIv)
							if len(keyData) < 80 {
								err = errors.New("sskey not valid: " + keyName)
							}

							if err == nil {
								code := ""
								code, err = sskeylib.MakeCode(langName, []byte(keyData[0:40]), []byte(keyData[40:80]))
								tmpKeyIv := make([]byte, 80)
								for i := 0; i < 40; i++ {
									tmpKeyIv[i] = byte(u.GlobalRand1.Intn(255))
									tmpKeyIv[40+i] = byte(u.GlobalRand2.Intn(255))
								}
								if err == nil {
									err = u.WriteFile("._"+sskeyFile, u.EncryptAes(code, tmpKeyIv[2:], tmpKeyIv[45:]))
								}

								scripts = append(scripts, "./.decryptor "+sskeyFile+" "+u.EncryptAes(string(tmpKeyIv), []byte("?GQ$0Kudfia7yfd=f+~L68PLm$uhKr4'=tV"), []byte("VFs7@s1okdsnj^f?HZ"))+" || exit -1")
							}
						}
					}
					if err != nil {
						der.Error(err.Error())
						sskeyFile = ""
					}
				}

				if langName == "php" {
					sskeyFile = ""
				}
			}
		}

		if line != "" {
			//scripts = append(scripts, "echo '$ "+strings.ReplaceAll(printLine, "'", "\\\\'")+"'")
			scripts = append(scripts, "echo '$ "+strings.ReplaceAll(line, "'", "\\\\'")+"'")
			scripts = append(scripts, line+" || exit -1")
		}

		if sskeyFile != "" {
			scripts = append(scripts, "rm -f "+sskeyFile)
		}
	}

	buildFile := fmt.Sprintf("._%s%d.sh", stage, i)
	der.Info("# make", buildFile)
	err := u.WriteFile(buildFile, strings.Join(scripts, "\n"))
	if err != nil {
		der.Error(buildFile, "write file failed", err.Error())
		return ""
	}
	return buildFile
}

type Deployer struct {
	logger   *log.Logger
	response *s.Response
	conn     *websocket.Conn
	outs     []string
}

func (der *Deployer) Info(args ...interface{}) {
	der.Output(fmt.Sprintln(args...))
}

func (der *Deployer) Error(args ...interface{}) {
	str := fmt.Sprintln(args...)
	der.logger.Error(str)
	der.Output(str)
}

func replaceVar(s, k, v string) string {
	varRegexp, err := regexp.Compile("(?i:{\\$" + k + "})")
	if err != nil {
		return s
	}
	s = varRegexp.ReplaceAllString(s, v)

	varRegexp2, err := regexp.Compile("(?i:\\${" + k + "})")
	if err != nil {
		return s
	}
	return varRegexp2.ReplaceAllString(s, v)
}

func (der *Deployer) BuildBySSH(from, buildId, shellFile, buildFile string) bool {
	//sshBuildFile := der.makeSSHBuildFile(buildId, buildFile)
	//if sshBuildFile == "" {
	//	return false
	//}

	sshBaseArgs := append(make([]string, 0), "-i", dataPath(".ssh", "id_ecdsa"), "-o", "StrictHostKeyChecking=no")
	scpBaseArgs := append(make([]string, 0), "-i", dataPath(".ssh", "id_ecdsa"), "-o", "StrictHostKeyChecking=no", "-r")

	a := strings.Split(from, " ")
	host := a[0]
	if len(a) > 1 {
		for i := 1; i < len(a); i++ {
			sshBaseArgs = append(sshBaseArgs, a[i])
			if a[i] == "-p" {
				a[i] = "-P"
			}
			scpBaseArgs = append(scpBaseArgs, a[i])
		}
	}

	if der.Run("scp", makeArgs(scpBaseArgs, "./", host+":"+buildId)...) != nil {
		return false
	}
	if der.Run("ssh", makeArgs(sshBaseArgs, host, fmt.Sprintf("cd %s && $(sh %s) %s", buildId, shellFile, buildFile))...) != nil {
		//if der.Run("ssh", makeArgs(sshBaseArgs, host, "sh", sshBuildFile)...) != nil {
		return false
	}
	if der.Run("scp", makeArgs(scpBaseArgs, host+":"+buildId+"/*", "./")...) != nil {
		return false
	}
	if der.Run("ssh", makeArgs(sshBaseArgs, host, "rm -rf "+buildId)...) != nil {
		return false
	}
	return true
}

func (der *Deployer) BuildByDocker(from, buildPath, dockerBuildFile, cachesPath string, mounts []string) bool {
	args := append(make([]string, 0), "run", "-i", "--network=host", "--rm", "-v", buildPath+":"+projectContainerPath)
	args = append(args, mounts...)
	froms := PraseCommandArgs(from)
	if len(froms) > 1 {
		args = append(args, froms[1:]...)
	}
	args = append(args, froms[0], "sh", projectContainerPath+"/"+dockerBuildFile)
	if der.Run("docker", args...) != nil {
		return false
	}
	return true
}

func makeArgs(baseArgs []string, newArgs ...string) []string {
	args := append(make([]string, 0), baseArgs...)
	return append(args, newArgs...)
}

func (der *Deployer) Output(str string) {
	if strings.Index(str, "ignoring symlink") != -1 {
		return
	}

	der.outs = append(der.outs, str)

	var err error
	if der.response != nil {
		_, err = der.response.FlushString(str)
	}
	if der.conn != nil {
		err = der.conn.WriteMessage(websocket.TextMessage, []byte(str))
	}
	if err != nil {
		der.logger.Error(err.Error())
	}
}

func (der *Deployer) Run(command string, args ...string) error {
	printCmd := fmt.Sprintln("#", command, strings.Join(args, " "))
	if command == "ssh" || command == "scp" || command == "rsync" {
		printCmd = strings.ReplaceAll(printCmd, dataPath(".ssh", "id_ecdsa"), "****")
	}
	der.Output(printCmd)

	cmd := exec.Command(command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		der.Error(err.Error())
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		der.Error(err.Error())
		return err
	}

	err = cmd.Start()
	if err != nil {
		der.Error(err.Error())
		return err
	}
	reader := io.MultiReader(stdout, stderr)
	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			//der.Error("Read stdout error", err.Error())
			break
		}
		der.Output(string(buf[0:n]))
	}

	err = cmd.Wait()
	if err != nil {
		der.Error(err.Error())
		return err
	}

	return nil
}

func SimpleRun(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	buf, err := cmd.Output()
	return strings.TrimSpace(string(buf)), err
}

func buildLogFile(context, project string, succeed bool) string {
	succeedFlag := "S"
	if !succeed {
		succeedFlag = "F"
	}
	t := time.Now()
	return dataPath(context, project, "builds", fmt.Sprintf("%.4d-%.2d", t.Year(), t.Month()), fmt.Sprintf("%.4d-%.2d-%.2d %.2d:%.2d:%.2d %s", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), succeedFlag))
}

func PraseCommandArgs(cmd string) []string {
	cmd = strings.TrimSpace(cmd) + " "
	args := make([]string, 0)
	start := -1
	var quota int32 = 0
	for i, c := range cmd {
		if start == -1 {
			start = i
			if c == '"' || c == '\'' {
				quota = c
			}
		} else if c == ' ' {
			if quota == 0 {
				if cmd[start] == cmd[i-1] && (cmd[start] == '"' || cmd[start] == '\'') {
					args = append(args, strings.ReplaceAll(cmd[start+1:i-1], fmt.Sprintf("\\%c", cmd[start]), fmt.Sprintf("%c", cmd[start])))
				} else {
					args = append(args, cmd[start:i])
				}
				start = -1
			}
		} else if c == quota {
			if i > 0 && cmd[i-1] != '\\' {
				quota = 0
			}
		}
	}
	return args
}
