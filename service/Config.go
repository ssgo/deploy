package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/ssgo/log"
	"github.com/ssgo/u"
	"os"
	"strings"
)

var _config = struct {
	DataPath string
	//AccessToken string
	KeepBuildPath      bool
	ManageToken        string
	encodedManageToken string
}{}

var reservedWords = map[string]bool{
	"login":    true,
	"global":   true,
	"contexts": true,
	"projects": true,
	"reg":      true,
}

var settedKey = []byte("?GQ$0K0GgLdO=f+~L68PLm$uhKr4'=tV")
var settedIv = []byte("VFs7@sK61cj^f?HZ")
var keysSetted = false

func SetEncryptKeys(key, iv []byte) {
	if !keysSetted {
		keysSetted = true
		settedKey = key
		settedIv = iv
	}
}

func dataPath(names ...string) string {
	return fmt.Sprintf("%s%c%s", _config.DataPath, os.PathSeparator, strings.Join(names, string(os.PathSeparator)))
}

func EncodeToken(s string) string {
	sha1Maker := sha1.New()
	sha1Maker.Write([]byte("SSGO-"))
	sha1Maker.Write([]byte(s))
	sha1Maker.Write([]byte("-Deploy"))
	return hex.EncodeToString(sha1Maker.Sum([]byte{}))
}

var logger = log.New(u.ShortUniqueId())

func logInfo(info string, extra ...interface{}) {
	logger.Info("Deploy: "+info, extra...)
}

func logError(error string, extra ...interface{}) {
	logger.Error("Deploy: "+error, extra...)
}
