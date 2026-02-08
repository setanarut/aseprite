package main

import (
	"fmt"

	"github.com/setanarut/aseprite"
)

func main() {
	ase, _ := aseprite.Read("test.ase")

	for _, tag := range ase.Tags {
		fmt.Println(tag.Name, tag.UserData)
	}

	for _, l := range ase.Layers {
		fmt.Println(l.Name, l.UserData)
	}

	for _, c := range ase.Frames[0].Cels {
		fmt.Println(c)
	}

}
