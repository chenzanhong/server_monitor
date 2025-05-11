package logs

import (
	"fmt"
	"runtime"
	"strings"
)

// --------------------------------------------------------------------------------
// GetRelativePath 获取调用者的相对路径和行号
func GetRelativePath() (file string, line int) {
	_, filepath, line, _ := runtime.Caller(0)
	i := strings.Index(filepath, "/server/")
	if i != -1 {
		filepath = "/" + filepath[i+len("/server/"):] // 加1是为了跳过"/"
	} else {
		filepath = "" // 或者其他默认值/错误处理
	}
	return filepath, line
}

func GetLogPrefix(skip int) (logPrefix string) {
	_, filepath, line, _ := runtime.Caller(skip)
	i := strings.Index(filepath, "/server/")
	if i != -1 {
		filepath = "/" + filepath[i+len("/server/"):]
	} else {
		filepath = "" // 或者其他默认值/错误处理
	}
	return fmt.Sprintf("%s %d: ", filepath, line)
}
