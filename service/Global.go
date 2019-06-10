package service

import (
	"fmt"
	"github.com/ssgo/log"
	"github.com/ssgo/s"
	"github.com/ssgo/u"
	"io/ioutil"
	"os"
	"time"
)

type GlobalInfo struct {
	Vars map[string]string
}

type CacheInfo struct {
	Name string
	Num  int
	Size int64
}

func getGlobalInfo() (out struct {
	GlobalInfo
	PublicKey string
}) {
	_ = u.Load(globalFile(), &out)
	out.PublicKey, _ = u.ReadFile(dataPath(".ssh", "id_dsa.pub"), 2048)
	return
}

func setGlobalInfo(in GlobalInfo, logger *log.Logger) bool {
	err := u.Save(globalFile(), &in)
	ok := false
	if err == nil {
		ok = true
		err = u.Save(archivedGlobalFile(), &in)
	}
	if err != nil {
		logger.Error(err.Error())
	}
	return ok
}

func setSSKeys(in s.Map, logger *log.Logger) bool {
	err := u.Save(sskeysFile(), in)
	ok := false
	if err == nil {
		ok = true
		err = u.Save(archivedSSKeysFile(), in)
	}
	if err != nil {
		logger.Error(err.Error())
	}
	return ok
}

func getCacheList(logger *log.Logger) []CacheInfo {
	out := make([]CacheInfo, 0)
	files, err := ioutil.ReadDir(dataPath("_caches"))
	if err == nil {
		for _, file := range files {
			fileName := file.Name()
			if fileName == "" || fileName[0] == '.' {
				continue
			}
			var n int = 0
			var size int64 = 0
			countDir(dataPath("_caches", fileName), &n, &size, logger)
			out = append(out, CacheInfo{
				Name: fileName,
				Num:  n,
				Size: size,
			})
		}
	} else {
		logger.Error(err.Error())
	}
	return out
}

func countDir(path string, n *int, size *int64, logger *log.Logger) {
	fmt.Println("  =====!!!!", path)
	files, err := ioutil.ReadDir(path)
	if err == nil {
		for _, file := range files {
			fileName := file.Name()
			fmt.Println("  =====", fileName)
			if fileName != "." && fileName != ".." {
				fmt.Println("  =====;;;;;;;", file.IsDir(), file)
				if file.IsDir() {
					countDir(fmt.Sprintf("%s%c%s", path, os.PathSeparator, fileName), n, size, logger)
				} else {
					fmt.Println("  =====>>", file)
					*n++
					*size += file.Size()
				}
			}
		}
	} else {
		logger.Error(err.Error())
	}
}

func removeCache(in struct{ CacheName string }, logger *log.Logger) bool {
	cachePath := dataPath("_caches", in.CacheName)
	if !u.FileExists(cachePath) {
		return true
	}
	err := os.RemoveAll(cachePath)
	if err != nil {
		logger.Error(err.Error())
		return false
	}
	return true
}

func globalFile() string {
	return dataPath("_global")
}

func sskeysFile() string {
	return dataPath("_sskeys")
}

func archivedGlobalFile() string {
	t := time.Now()
	return dataPath("_archived", "_global", fmt.Sprintf("%.4d-%.2d-%.2d %.2d:%.2d:%.2d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()))
}

func archivedSSKeysFile() string {
	t := time.Now()
	return dataPath("_archived", "_sskeys", fmt.Sprintf("%.4d-%.2d-%.2d %.2d:%.2d:%.2d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second()))
}
