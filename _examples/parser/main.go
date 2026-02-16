package main

import (
	"fmt"
	"strconv"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/setanarut/aseprite"
)

func main() {

	ase, err := aseprite.Read("../../test_files/test_paletted.ase", false)

	if err != nil {
		panic(err)
	}

	fmt.Println(len(ase.Tags))

	tag2 := ase.GetTagByName("tag2")
	myImageLayer := ase.GetLayerByName("my image layer")
	imgio.Save(
		myImageLayer.Name+"_"+tag2.Name+"_start.png",
		myImageLayer.Cel(tag2.Start).Image,
		imgio.PNGEncoder(),
	)

	ase.BuildTilemapImages()
	frame := 0
	for i, layer := range ase.Layers {
		if !layer.IsCelEmpty(frame) && layer.IsTilemapLayer() {
			imgio.Save(strconv.Itoa(i)+"_"+layer.Name+".png", layer.Cel(frame).Image, imgio.PNGEncoder())
		}
	}

}
