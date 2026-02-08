package main

import (
	"fmt"

	"github.com/setanarut/aseprite"
)

func main() {
	ase, _ := aseprite.Read("test.ase")

	fmt.Println(ase.ColorDepth)
	fmt.Println(ase.Tags)
	fmt.Println(ase.Frames[0].Cels[0].Image.Bounds())
	fmt.Println(ase.Frames[0].Cels[0].Opacity)
}
