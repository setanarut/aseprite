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

	for i, l := range ase.Layers {
		fmt.Println("--------------------------")
		fmt.Println("Layer Name:", l.Name)
		fmt.Println("Layer Type:", l.Type)
		fmt.Println("Cel Pos:", ase.Frames[0].Cels[i].Pos)
		fmt.Println("Cel Size:", ase.Frames[0].Cels[i].Size)
		switch l.Type {
		case aseprite.Tilemap:
			fmt.Println("Tileset ID:", l.Tileset.ID)
			fmt.Println("TileSize:", l.Tileset.TileSize)
			fmt.Println("Frame 0 Tiles:", ase.Frames[0].Cels[i].Tiles)
			fmt.Println("Num tiles:", l.Tileset.NumTiles)
		}
	}
}
