package quieoo

import (
	"runtime"
)

func PrintMyName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}
func PrintCallerName() string {
	pc, _, _, _ := runtime.Caller(2)
	return runtime.FuncForPC(pc).Name()
}

func Trace(){
	//fmt.Printf("我是 %s, %s 在调用我!\n", quieoo.PrintMyName(),quieoo.PrintCallerName())
}