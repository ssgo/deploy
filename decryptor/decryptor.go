package main

import (
	"fmt"
	"github.com/ssgo/u"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("need code file name")
		return
	}
	if len(os.Args) < 3 {
		fmt.Println("need key & iv")
		return
	}

	keyiv := u.DecryptAes(os.Args[2], []byte("?GQ$0Kudfia7yfd=f+~L68PLm$uhKr4'=tV"), []byte("VFs7@s1okdsnj^f?HZ"))

	src, err := u.ReadFile("._"+os.Args[1], 20480)
	if err != nil {
		fmt.Println("failed to read code file: " + os.Args[1] + "\n" + err.Error())
		return
	}

	out := u.DecryptAes(src, []byte(keyiv[2:]), []byte(keyiv[45:]))
	err = u.WriteFile(os.Args[1], out)
	if err != nil {
		fmt.Println("failed to write code file: " + os.Args[1] + "\n" + err.Error())
	}

	_ = os.Remove("._" + os.Args[1])
}
