package main

import (
    "fmt"

	"github.com/kercre123/wire-pod/chipper/pkg/initwirepod"
	stt "github.com/kercre123/wire-pod/chipper/pkg/wirepod/stt/iflytek"
)

func main() {
	fmt.Println("Starting Iflytek STT...")
	initwirepod.StartFromProgramInit(stt.Init, stt.STT, stt.Name)
}
