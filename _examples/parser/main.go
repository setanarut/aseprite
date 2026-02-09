package main

import (
	"fmt"

	"github.com/setanarut/aseprite"
)

func main() {
	ase, err := aseprite.Read("test.ase", false)

	if err != nil {
		panic(err)
	}

	fmt.Println(ase.Layers[0].Opacity, ase.Layers[0].BlendMode)

	for i, l := range ase.Layers {
		fmt.Println(l.Name, l.IsVisible, l.Type, l.ChildLevel, ase.Frames[3].Cels[i].IsEmpty())
	}

}
