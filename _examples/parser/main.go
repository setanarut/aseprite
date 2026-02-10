package main

import (
	"fmt"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/setanarut/aseprite"
)

func main() {
	ase, err := aseprite.Read("test.ase", false)

	if err != nil {
		panic(err)
	}

	imgio.Save(ase.Tilesets[0].Name+".png", ase.Tilesets[0].Image, imgio.PNGEncoder())
	cel := ase.GetLayerByName("test tilemap").Cel(0)
	tmi := cel.BuildTilemapImage()
	fmt.Println("tmi rect", tmi.Bounds(), cel.Pos)
	imgio.Save("tmi.png", tmi, imgio.PNGEncoder())

	for i, layer := range ase.Layers {
		fmt.Println("--------------------------")
		fmt.Println("Layer Name:", layer.Name)
		fmt.Println("LAYER INDEX:", i)
		fmt.Println("Child level:", layer.ChildLevel)
		fmt.Println("Layer type:", layer.Type)
		fmt.Println("User data:", layer.UserData)
		fmt.Println("Is frame 0 empty:", layer.IsCelEmpty(0))
		switch layer.Type {
		case aseprite.Image:
			fmt.Println("frame 0 ZIndex:", layer.Cels[0].ZIndex)
			fmt.Println("Cel Pos:", layer.Cel(0).Pos)
			fmt.Println("Cel Size:", layer.Cel(0).Size)
		case aseprite.Group:
			fmt.Println("Blend mode:", layer.BlendMode)
			fmt.Println("--LAYER FLAGS --:\n" + layer.LayerFlags.String())
		case aseprite.Tilemap:
			fmt.Println("Tileset ID:", layer.Tileset.ID)
			fmt.Println("TileSize:", layer.Tileset.TileSize)
			fmt.Println("Frame 0 Tiles:", layer.Cel(0).Tiles)
			fmt.Println("Num tiles:", layer.Tileset.NumTiles)
		}
	}
}
