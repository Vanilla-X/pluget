package logx

import (
	"fmt"
	"os"
)

func Infof(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "info: "+format+"\n", args...)
}

func Warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

func Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}
