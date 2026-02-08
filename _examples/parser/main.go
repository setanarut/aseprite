package main

import (
	"fmt"

	"github.com/setanarut/aseprite"
)

func main() {
	ase, err := aseprite.Read("test.ase")

	if err != nil {
		panic(err)
	}

	fmt.Println("Color depth", ase.ColorDepth, "BPP")

	topLayerIndex := len(ase.Layers) - 1
	for i, frame := range ase.Frames {
		fmt.Printf("Layer: %v Cel pos: %v Cel text: %s \n", i,
			frame.Cels[topLayerIndex].Image.Bounds().Min,
			frame.Cels[topLayerIndex].UserData.Text,
		)
	}

	for _, tag := range ase.Tags {
		fmt.Println(tag.Name, tag.UserData.Color)
	}

	for _, l := range ase.Layers {
		fmt.Println(l.Name, l.UserData.Text)
	}

}
