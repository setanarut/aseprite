package main

import (
	"strconv"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/setanarut/aseprite"
)

func main() {

	ase, err := aseprite.Read("../../test_files/test_paletted.ase", false)

	if err != nil {
		panic(err)
	}

	ase.BuildTilemapImages()

	frame := 0
	for i, layer := range ase.Layers {
		if layer.IsCelEmpty(0) || layer.Type == aseprite.Group {
			continue
		}
		imgio.Save(strconv.Itoa(i)+"_"+layer.Name+".png", layer.Cel(frame).Image, imgio.PNGEncoder())
	}

}
