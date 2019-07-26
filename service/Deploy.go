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

var projectContainerPath = "/root"

func lock(name string) {
	if gitLocks[name] == nil {
		gitLocks[name] = new(sync.Mutex)
	}
	gitLocks[name].Lock()
}

func unlock(name string) {
	gitLocks[name].Unlock()
}

func getTags(in struct {
	ContextName string
	ProjectName string
	Clean       bool
}, logger *log.Logger) []string {
	outTags := make([]string, 1)
	outTags[0] = "master"

	ctx, proj := loadProject(in.ContextName, in.ProjectName, logger)
	if ctx == nil || proj == nil {
		return outTags
	}

	lock(proj.Repository)
	gitPath := checkout(proj.Repository, "master", true, in.Clean)
	gitTags, err := u.RunCommand("git", "-C", gitPath, "tag", "-l", "--sort=-taggerdate")
	unlock(proj.Repository)

	if err != nil {
		logger.Error(err.Error())
		return outTags
	}

	if gitPath != "" && len(outTags) > 0 {
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
			logger.Error("outTags err:" + tagErr)
		}
		outTags = append(outTags, routs...)
	}

	return outTags
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

	str, err := u.ReadFile(dataPath(in.ContextName, in.ProjectName, "builds", in.Build[0:7], in.Build), 2048000)
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
	gitPath := checkout(proj.Repository, "master", true, in.Clean)
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

	// 载入 CI
	if proj.Repository == "" {
		logger.Error("no repository")
		return nil, nil, nil
	}

	vars["CONTEXT"] = contextName
	vars["PROJECT"] = projectName
	vars["TAG"] = tag

	ciStr, err := u.ReadFile(ciFile(contextName, projectName), 204800)
	if err != nil {
		logger.Error(err.Error())
		return nil, nil, nil
	}

	for k, v := range vars {
		ciStr = replaceVar(ciStr, k, v)
	}
	//for k, v := range vars {
	//	ciStr = strings.ReplaceAll(ciStr, fmt.Sprintf("{$%s}", k), v)
	//}
	//for k, v := range vars {
	//	ciStr = strings.ReplaceAll(ciStr, fmt.Sprintf("$%s", k), v)
	//}

	ci := LoadCI(ciStr)
	if len(ci.Build) == 0 && len(ci.Deploy) == 0 {
		logger.Error("no build and deploy")
		return nil, nil, nil
	}
	if ci.CacheTag == "" {
		ci.CacheTag = contextName + "-" + projectName
	} else {
		ci.CacheTag = strings.ReplaceAll(ci.CacheTag, "/", "-")
	}

	return vars, proj, &ci
}

func build(in struct {
	ContextName string
	ProjectName string
	Token       string
	Tag         string
}, logger *log.Logger, response *s.Response, conn *websocket.Conn) {
	der := Deployer{
		logger:   logger,
		response: response,
		conn:     conn,
	}

	der.outs = make([]string, 0)
	succeed := false
	defer func() {
		der.Output(u.StringIf(succeed, "# Done", "# Failed"))
		if conn != nil {
			_ = conn.Close()
		}
		_ = u.WriteFile(buildLogFile(in.ContextName, in.ProjectName, succeed), strings.Join(der.outs, "\n"))
	}()

	vars, proj, ci := loadDeployInfo(in.ContextName, in.ProjectName, in.Tag, logger)

	//cacheTagValue := strings.ReplaceAll(ci.CacheTag, "$CONTEXT", in.ContextName)
	//cacheTagValue = strings.ReplaceAll(cacheTagValue, "$PROJECT", in.ProjectName)
	//cacheTagValue = strings.ReplaceAll(cacheTagValue, "$TAG", in.Tag)

	// 检查敏感内容
	if strings.Index(u.Json(vars)+u.Json(ci), ".poo_info_a") != -1 {
		der.Error("has sensitive info")
		return
	}

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
	lock(proj.Repository)
	gitPath := checkout(proj.Repository, in.Tag, true, false)
	der.Info("cp -r " + gitPath + "/* " + buildPath)
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
				return
			}
		}
	}
	unlock(proj.Repository)
	if err != nil {
		return
	}
	_ = os.Chdir(buildPath)

	//// 克隆仓库
	//if der.Run("git", "clone", proj.Repository, ".") != nil {
	//	return
	//}
	//if proj.Tag != "" && der.Run("git", "checkout", proj.Tag) != nil {
	//	return
	//}
	der.Info()
	var mountStr string
	// 初始化 cache
	if ci.Cache != "" {
		caches := strings.Split(ci.Cache, " ")
		for _, cachePath := range caches {
			if len(cachePath) == 0 {
				continue
			}
			var cachedPath string
			var singleCache string
			if cachePath[0] != '/' {
				singleCache = cachePath
				cachedPath = dataPath("_caches", ci.CacheTag, cachePath)
				cachePath = fmt.Sprintf("%s%c%s", buildPath, os.PathSeparator, cachePath)
			} else {
				singleCache = cachePath[1:]
				cachedPath = dataPath("_caches", ci.CacheTag, singleCache)
			}
			mountStr += " -v " + cachedPath + ":" + projectContainerPath + "/" + singleCache
			if !u.FileExists(cachedPath) {
				_ = os.MkdirAll(cachedPath, 0700)
			}
			if u.FileExists(cachePath) {
				_ = os.RemoveAll(cachePath)
			}
			//ln made node_modules problem
			//_ = der.Run("ln", "-s", cachedPath, cachePath)
			//_ = der.Run("cp", "-r", cachedPath, cachePath)
		}
	}

	shellFile := der.makeGetShellFile()
	if shellFile == "" {
		return
	}

	defer func() {
		//if ci.Cache != "" {
		//	caches := strings.Split(ci.Cache, " ")
		//	for _, cachePath := range caches {
		//		if len(cachePath) == 0 {
		//			continue
		//		}
		//		var cachedPath string
		//		if cachePath[0] != '/' {
		//			cachedPath = dataPath("_caches", ci.CacheTag, cachePath)
		//			cachePath = fmt.Sprintf("%s%c%s", buildPath, os.PathSeparator, cachePath)
		//		} else {
		//			cachedPath = dataPath("_caches", ci.CacheTag, cachePath[1:])
		//		}
		//		if u.FileExists(cachePath) {
		//			_ = os.RemoveAll(cachedPath)
		//			u.CheckPath(cachedPath)
		//			_ = der.Run("cp", "-r", cachePath, cachedPath)
		//		}
		//	}
		//}
		_ = os.RemoveAll(buildPath)
	}()

	// 构建
	for i, b := range ci.Build {
		// 创建脚本
		buildFile := makeScriptFile(vars, i, b.Script, &der, "build")
		dockerBuildFile := der.makeDockerBuildFile(buildFile)
		if buildFile == "" || dockerBuildFile == "" {
			return
		}
		if b.From == "" || b.From == "local" {
			// 从本地构建
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
			if der.BuildBySSH(b.From, buildId, shellFile, buildFile) == false {
				return
			}
		} else {
			// 从Docker构建
			if !der.BuildByDocker(b.From, buildPath, dockerBuildFile, dataPath("_caches"), mountStr) {
				return
			}
		}
		der.Info()
	}

	// zoneinfo for alpine
	if len(ci.Deploy) > 0 {
		if u.FileExists("/usr/share/zoneinfo") {
			_ = der.Run("cp", "-r", "/usr/share/zoneinfo", buildPath)
		}
	}

	// 部署
	for i, d := range ci.Deploy {
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
			if !der.BuildByDocker(d.From, buildPath, dockerBuildFile, dataPath("_caches"), "") {
				return
			}
		}

		der.Info()
	}

	succeed = true
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
	scripts := "cd " + projectContainerPath + "\n$(sh _getShell.sh) " + buildScript
	err := u.WriteFile("_dockerBuild.sh", scripts)
	if err != nil {
		der.Error(err.Error())
		return ""
	}
	return "_dockerBuild.sh"
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
	err := u.WriteFile("_getShell.sh", scripts)
	if err != nil {
		der.Error(err.Error())
		return ""
	}
	return "_getShell.sh"
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
		printLine := line
		if strings.HasPrefix(line, "scp ") {
			newLine := "scp"
			if strings.Index(line, " -i ") == -1 {
				newLine += fmt.Sprint(" -i ", ".poo_info_a")
			}
			if strings.Index(line, "StrictHostKeyChecking") == -1 {
				newLine += fmt.Sprint(" -o StrictHostKeyChecking=no")
			}
			line = newLine + line[3:]
			printLine = strings.ReplaceAll(line, ".poo_info_a", "****")

			if !u.FileExists(".poo_info_a") {
				_, err := SimpleRun("cp", dataPath(".ssh", "id_dsa"), ".poo_info_a")
				if err != nil {
					der.Error("make key failed")
					return ""
				}
			}
		}

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
			scripts = append(scripts, "echo '$ "+strings.ReplaceAll(printLine, "'", "\\\\'")+"'")
			scripts = append(scripts, line+" || exit -1")
		}

		if sskeyFile != "" {
			scripts = append(scripts, "rm -f "+sskeyFile)
		}
	}

	buildFile := fmt.Sprintf("_%s%d.sh", stage, i)
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
	varRegexp, err := regexp.Compile("(?i:{?\\$" + k + "}?)")
	if err != nil {
		return s
	}

	return varRegexp.ReplaceAllString(s, v)
}

func (der *Deployer) BuildBySSH(from, buildId, shellFile, buildFile string) bool {
	//sshBuildFile := der.makeSSHBuildFile(buildId, buildFile)
	//if sshBuildFile == "" {
	//	return false
	//}

	sshBaseArgs := append(make([]string, 0), "-i", dataPath(".ssh", "id_dsa"), "-o", "StrictHostKeyChecking=no")
	scpBaseArgs := append(make([]string, 0), "-i", dataPath(".ssh", "id_dsa"), "-o", "StrictHostKeyChecking=no", "-r")

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

func (der *Deployer) BuildByDocker(from, buildPath, dockerBuildFile, cachesPath string, mountStr string) bool {
	args := append(make([]string, 0), "run", "--rm", "-v", buildPath+":"+projectContainerPath, mountStr)
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
		der.Error(err.Error())
	}
}

func (der *Deployer) Run(command string, args ...string) error {
	printCmd := fmt.Sprintln("#", command, strings.Join(args, " "))
	if command == "ssh" || command == "scp" {
		printCmd = strings.ReplaceAll(printCmd, dataPath(".ssh", "id_dsa"), "****")
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
