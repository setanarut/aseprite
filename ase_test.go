package aseprite

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAseDataStructure(t *testing.T) {

	expected := Ase{
		ColorDepth:    8,
		Size:          image.Point{X: 64, Y: 64},
		PixelRatio:    image.Point{1, 1},
		HasLayersUUID: true,
		Palette: color.Palette{
			color.NRGBA{0, 0, 0, 0},
			color.NRGBA{223, 62, 35, 156},
			color.NRGBA{255, 213, 65, 255},
			color.NRGBA{156, 219, 67, 255},
			color.NRGBA{255, 255, 255, 255},
		},
		Durations: []time.Duration{
			time.Millisecond * 500,
			time.Millisecond * 500,
			time.Millisecond * 1000,
			time.Millisecond * 1000,
		},
		Layers: []*Layer{
			{
				Name:      "group1",
				Type:      1,
				Flags:     2,
				BlendMode: 0,
				Opacity:   0,
				UserData: UserData{
					Color: color.NRGBA{R: 0, G: 0, B: 0, A: 0},
					Text:  "hi",
				},
				Cels: []*Cel{nil, nil, nil, nil},
			},
			{
				Name:    "test tilemap",
				Type:    2,
				Flags:   2,
				Opacity: 158,
				UserData: UserData{
					Color: color.NRGBA{R: 223, G: 62, B: 35, A: 255},
					Text:  "hello world",
				},
				Cels: []*Cel{
					{
						LayerIndex: 1,
						Pos:        image.Point{X: 4, Y: 2},
						Size:       image.Point{X: 3, Y: 2},
						Opacity:    255,
						Tiles: []Tile{
							{ID: 1, XYD: 7},
							{ID: 3, XYD: 0},
							{ID: 0, XYD: 0},
							{ID: 0, XYD: 0},
							{ID: 2, XYD: 7},
							{ID: 3, XYD: 0},
						},
						Type: 3,
						UserData: UserData{
							Color: color.NRGBA{R: 223, G: 62, B: 35, A: 255},
							Text:  "hello world",
						},
					},
					{
						LayerIndex: 1,
						Pos:        image.Point{X: 16, Y: 16},
						Size:       image.Point{X: 1, Y: 1},
						Opacity:    255,
						Tiles: []Tile{
							{ID: 2, XYD: 3},
						},
						Type: 3,
						UserData: UserData{
							Color: color.NRGBA{},
							Text:  "",
						},
					},
					{
						LayerIndex: 1,
						Pos:        image.Point{X: 32, Y: 32},
						Size:       image.Point{X: 1, Y: 1},
						Opacity:    255,
						Tiles: []Tile{
							{ID: 3, XYD: 0},
						},
						Type: 3,
					},
					{
						LayerIndex: 1,
						Pos:        image.Point{X: 0, Y: 48},
						Size:       image.Point{X: 1, Y: 1},
						Opacity:    255,
						Tiles: []Tile{
							{ID: 2, XYD: 0},
						},
						Type: 3,
					},
				},
				ChildLevel:   1,
				TilesetIndex: 0,
			},
			{
				Name:    "my image layer",
				Type:    0,
				Flags:   3,
				Opacity: 255,
				UserData: UserData{
					Text: "test data",
				},
				Cels: []*Cel{
					{
						LayerIndex: 2,
						ZIndex:     2,
						Pos:        image.Point{X: 10, Y: 8},
						Size:       image.Point{X: 44, Y: 50},
						Type:       2,
						Opacity:    255,
						UserData: UserData{
							Color: color.NRGBA{R: 223, G: 62, B: 35, A: 255},
							Text:  "hello world",
						},
						Image: &image.Paletted{},
					},
					{
						LayerIndex: 2,
						Pos:        image.Point{X: 14, Y: 19},
						Size:       image.Point{X: 34, Y: 30},
						Type:       2,
						Opacity:    255,
						Image:      &image.Paletted{},
					},
					{
						LayerIndex: 2,
						Pos:        image.Point{X: 11, Y: 9},
						Size:       image.Point{X: 39, Y: 43},
						Type:       2,
						Opacity:    255,
						Image:      &image.Paletted{},
						UserData: UserData{
							Text: "tag2_start",
						},
					},
					{
						LayerIndex: 2,
						Pos:        image.Point{X: 7, Y: 11},
						Size:       image.Point{X: 45, Y: 47},
						Type:       2,
						Opacity:    255,
						Image:      &image.Paletted{},
					},
				},
			},
		},
		Slices: []Slice{
			{
				Name: "slice1",
				UserData: UserData{
					Color: color.NRGBA{R: 0, G: 127, B: 255, A: 207},
					Text:  "hello world",
				},
				Frames: []SliceFrame{
					{Bounds: image.Rect(23, 8, 59, 31), Rect9Slices: image.Rect(5, 5, 39, 26), Pivot: image.Point{10, 10}},
					{Bounds: image.Rect(23, 8, 59, 31), Rect9Slices: image.Rect(5, 5, 39, 26), Pivot: image.Point{10, 10}},
					{Bounds: image.Rect(23, 8, 59, 31), Rect9Slices: image.Rect(5, 5, 39, 26), Pivot: image.Point{10, 10}},
					{Bounds: image.Rect(23, 8, 59, 31), Rect9Slices: image.Rect(5, 5, 39, 26), Pivot: image.Point{10, 10}},
				},
			},
		},
		Tags: []*Tag{
			{
				Name: "tag1",
				UserData: UserData{
					Color: color.NRGBA{R: 247, G: 165, B: 71, A: 255},
					Text:  "hello world",
				},
				Start:         0,
				End:           1,
				Repeat:        2,
				LoopDirection: 2,
			},
			{
				Name:  "tag2",
				Start: 2,
				End:   3,
			},
		},
		Tilesets: []*Tileset{
			{
				Name:      "tileset test",
				ID:        0,
				TileSize:  image.Point{X: 16, Y: 16},
				NumTiles:  4,
				Flags:     6,
				BaseIndex: 1,
				Image:     &image.Paletted{},
			},
		},
	}

	actual, err := Read("test_files/test_paletted.ase", false)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	compareAse(t, &expected, &actual)
}

func TestAseProp(t *testing.T) {

	expected := Ase{
		ColorDepth:    8,
		Size:          image.Point{X: 16, Y: 16},
		PixelRatio:    image.Point{1, 2},
		Palette:       color.Palette{color.NRGBA{180, 32, 42, 255}},
		HasLayersUUID: true,
		Layers: []*Layer{
			{
				Name: "Layer 1",
				Type: Image,

				UUID:      uuid.UUID{82, 121, 255, 18, 51, 254, 69, 190, 162, 78, 113, 67, 50, 237, 190, 236},
				Flags:     0,
				BlendMode: Normal,
				Opacity:   255,
				UserData: UserData{
					Color: color.NRGBA{223, 62, 35, 255},
					Text:  "user data text",
				},
			},
		},
	}

	actual, err := Read("test_files/prop.ase", false)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	compareAseProp(t, &expected, &actual)
}

func TestPalettedTilemapRender(t *testing.T) {

	ase, err := Read("test_files/test_paletted.ase", false)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	tag2 := ase.GetTagByName("tag2")
	imlayer := ase.GetLayerByName("my image layer")

	if imlayer.Cel(tag2.Start).UserData.Text != "tag2_start" {
		t.Errorf("tag2 start userdata text is not tag2_start")
	}

	ase.BuildTilemapImages()

	if !ase.GetLayerByName("test tilemap").IsTilemapLayer() {
		t.Errorf("test tilemap layer is not tilemap layer")
	}

	gotImage := ase.GetLayerByName("test tilemap").Cel(0).Image

	gotImageP, ok := gotImage.(*image.Paletted)

	if !ok {
		t.Errorf("Tilemap gotImage image is not *image.Paletted")
	}

	file, err := os.Open("test_files/tilemap_paletted.png")
	if err != nil {
		panic(err)
	}

	expImage, err := png.Decode(file)
	if err != nil {
		panic(err)
	}

	expImageP, ok := expImage.(*image.Paletted)

	if !ok {
		t.Errorf("Tilemap expImage image is not *image.Paletted")
	}

	if !slices.Equal(gotImageP.Pix, expImageP.Pix) {
		t.Errorf("Tilemap image is not equal")
	}

}
func TestNRGBATilemapRender(t *testing.T) {

	a, err := Read("test_files/test_nrgba.ase", false)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	a.BuildTilemapImages()

	gotImage := a.GetLayerByName("test tilemap").Cel(0).Image

	gotImageNRGBA, ok := gotImage.(*image.NRGBA)

	if !ok {
		t.Errorf("Tilemap gotImage image is not *image.NRGBA")
	}

	file, err := os.Open("test_files/tilemap_nrgba.png")
	if err != nil {
		panic(err)
	}

	expImage, err := png.Decode(file)
	if err != nil {
		panic(err)
	}

	expImageNRGBA, ok := expImage.(*image.NRGBA)

	if !ok {
		t.Errorf("Tilemap expImage image is not *image.NRGBA type is %T", expImageNRGBA)
	}

	if !slices.Equal(gotImageNRGBA.Pix, expImageNRGBA.Pix) {
		t.Errorf("Tilemap image is not equal")
	}

}
func TestGrayscaleTilemapRender(t *testing.T) {

	a, err := Read("test_files/test_grayscale.ase", false)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	a.BuildTilemapImages()

	gotImage := a.GetLayerByName("test tilemap").Cel(0).Image

	gotImageNRGBA, ok := gotImage.(*image.NRGBA)

	if !ok {
		t.Errorf("Tilemap gotImage image is not *image.NRGBA")
	}

	file, err := os.Open("test_files/tilemap_grayscale.png")
	if err != nil {
		panic(err)
	}

	expImage, err := png.Decode(file)
	if err != nil {
		panic(err)
	}

	expImageNRGBA, ok := expImage.(*image.NRGBA)

	if !ok {
		t.Errorf("Tilemap expImage image is not *image.NRGBA type is %T", expImageNRGBA)
	}

	if !slices.Equal(gotImageNRGBA.Pix, expImageNRGBA.Pix) {
		t.Errorf("Tilemap image is not equal")
	}

}

func TestParseWrongFile(t *testing.T) {
	_, err := Read("test_files/tilemap_grayscale.png", false)
	if err == nil {
		t.Errorf("Ok to read wrong file: %v", err)
	}

}

func TestReadFs(t *testing.T) {
	testFS := os.DirFS("test_files")
	_, err := ReadFs(testFS, "tilemap_grayscale.png", false)
	if err == nil {
		t.Errorf("Ok to read wrong file: %v", err)
	}
}

func TestParseOldPalette0x0011(t *testing.T) {
	a := &Ase{
		Palette: make(color.Palette, 256),
	}

	raw := []byte{
		0x01, 0x00,
		0x00,
		0x02,
		63, 0, 0,
		0, 63, 0,
	}

	a.parseOldPalette0x0011(raw)

	if a.Palette[0].(color.NRGBA).R != 252 {
		t.Error("İlk renk yanlış")
	}

	a2 := &Ase{
		Palette: make(color.Palette, 256),
	}

	raw2 := make([]byte, 2+2+256*3)
	binary.LittleEndian.PutUint16(raw2[0:2], 1)
	raw2[2] = 0x00
	raw2[3] = 0x00

	a2.parseOldPalette0x0011(raw2)

	if len(a2.Palette) != 256 {
		t.Error("Palette boyutu yanlış")
	}
}

func TestParseColorProfile(t *testing.T) {
	a := &Ase{}

	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[0:2], 1)
	binary.LittleEndian.PutUint32(data[4:8], 65536)

	cp, err := a.parseColorProfile(data)
	if err != nil {
		t.Error(err)
	}
	if cp.Gamma != 1.0 {
		t.Error("Gamma yanlış")
	}

	data2 := make([]byte, 24)
	binary.LittleEndian.PutUint16(data2[0:2], 2)
	binary.LittleEndian.PutUint32(data2[16:20], 4)
	copy(data2[20:24], []byte{1, 2, 3, 4})

	cp2, err := a.parseColorProfile(data2)
	if err != nil || len(cp2.ICC) != 4 {
		t.Error("ICC parse error")
	}

	_, err = a.parseColorProfile([]byte{1, 2, 3})
	if err == nil {
		t.Error("no error error")
	}

	data3 := make([]byte, 18)
	binary.LittleEndian.PutUint16(data3[0:2], 2)
	_, err = a.parseColorProfile(data3)
	if err == nil {
		t.Error("no error error")
	}

	data4 := make([]byte, 20)
	binary.LittleEndian.PutUint16(data4[0:2], 2)
	binary.LittleEndian.PutUint32(data4[16:20], 100)
	_, err = a.parseColorProfile(data4)
	if err == nil {
		t.Error("no error error")
	}
}

func compareTileset(t *testing.T, index int, exp, act *Tileset) {
	t.Helper()
	prefix := fmt.Sprintf("Tileset[%d]('%s')", index, exp.Name)

	if exp.Name != act.Name {
		t.Errorf("%s Name: expected '%s', got '%s'", prefix, exp.Name, act.Name)
	}
	if exp.ID != act.ID {
		t.Errorf("%s ID: expected %d, got %d", prefix, exp.ID, act.ID)
	}
	if exp.TileSize != act.TileSize {
		t.Errorf("%s TileSize: expected %v, got %v", prefix, exp.TileSize, act.TileSize)
	}
	if exp.NumTiles != act.NumTiles {
		t.Errorf("%s NumTiles: expected %d, got %d", prefix, exp.NumTiles, act.NumTiles)
	}
	if exp.Flags != act.Flags {
		t.Errorf("%s Flags: expected %d, got %d", prefix, exp.Flags, act.Flags)
	}
	if exp.BaseIndex != act.BaseIndex {
		t.Errorf("%s BaseIndex: expected %d, got %d", prefix, exp.BaseIndex, act.BaseIndex)
	}

	// Image existence check
	if (exp.Image == nil) != (act.Image == nil) {
		t.Errorf("%s Image: expected nil=%v, got nil=%v", prefix, exp.Image == nil, act.Image == nil)
	}
}

func compareUserData(t *testing.T, prefix string, exp, act *UserData) {
	t.Helper()

	if exp.Text != act.Text {
		t.Errorf("%s UserData.Text: expected '%s', got '%s'", prefix, exp.Text, act.Text)
	}

	if !isColorEqual(exp.Color, act.Color) {
		t.Errorf("%s UserData.Color: expected %+v, got %+v", prefix, exp.Color, act.Color)
	}
}

func isColorEqual(c1, c2 color.NRGBA) bool {
	// Both transparent
	if c1.A == 0 && c2.A == 0 {
		return true
	}

	// Aseprite sometimes returns empty colors as RGB 0, A 255
	if (c1.R == 0 && c1.G == 0 && c1.B == 0 && (c1.A == 0 || c1.A == 255)) &&
		(c2.R == 0 && c2.G == 0 && c2.B == 0 && (c2.A == 0 || c2.A == 255)) {
		return true
	}

	return c1 == c2
}

func compareAse(t *testing.T, exp, act *Ase) {
	t.Helper()

	if !slices.Equal(exp.Palette, act.Palette) {
		t.Errorf("Palette: expected %d, got %d", exp.Palette, act.Palette)
	}

	// Basic properties
	if exp.ColorDepth != act.ColorDepth {
		t.Errorf("ColorDepth: expected %d, got %d", exp.ColorDepth, act.ColorDepth)
	}

	if exp.PixelRatio != act.PixelRatio {
		t.Errorf("PixelRatio: expected %v, got %v", exp.PixelRatio, act.PixelRatio)
	}
	if exp.HasLayersUUID != act.HasLayersUUID {
		t.Errorf("HasUUID: expected %v, got %v", exp.HasLayersUUID, act.HasLayersUUID)
	}

	if exp.Size != act.Size {
		t.Errorf("Size: expected %v, got %v", exp.Size, act.Size)
	}

	// Durations
	if len(exp.Durations) != len(act.Durations) {
		t.Errorf("Durations count: expected %d, got %d", len(exp.Durations), len(act.Durations))
	} else {
		for i := range exp.Durations {
			if exp.Durations[i] != act.Durations[i] {
				t.Errorf("Duration[%d]: expected %v, got %v", i, exp.Durations[i], act.Durations[i])
			}
		}
	}

	// Tags
	if len(exp.Tags) != len(act.Tags) {
		t.Errorf("Tags count: expected %d, got %d", len(exp.Tags), len(act.Tags))
	} else {
		for i := range exp.Tags {
			compareTag(t, i, exp.Tags[i], act.Tags[i])
		}
	}

	// Layers
	if len(exp.Layers) != len(act.Layers) {
		t.Errorf("Layers count: expected %d, got %d", len(exp.Layers), len(act.Layers))
	} else {
		for i := range exp.Layers {
			compareLayer(t, i, exp.Layers[i], act.Layers[i])
		}
	}

	// Slices
	if len(exp.Slices) != len(act.Slices) {
		t.Errorf("Slices count: expected %d, got %d", len(exp.Slices), len(act.Slices))
	} else {
		for i := range exp.Slices {
			compareSlice(t, i, &exp.Slices[i], &act.Slices[i])
		}
	}

	// Tilesets
	if len(exp.Tilesets) != len(act.Tilesets) {
		t.Errorf("Tilesets count: expected %d, got %d", len(exp.Tilesets), len(act.Tilesets))
	} else {
		for i := range exp.Tilesets {
			compareTileset(t, i, exp.Tilesets[i], act.Tilesets[i])
		}
	}
}

func compareTag(t *testing.T, index int, exp, act *Tag) {
	t.Helper()
	prefix := fmt.Sprintf("Tag[%d]('%s')", index, exp.Name)

	if exp.Name != act.Name {
		t.Errorf("%s Name: expected '%s', got '%s'", prefix, exp.Name, act.Name)
	}
	if exp.Start != act.Start {
		t.Errorf("%s Start: expected %d, got %d", prefix, exp.Start, act.Start)
	}
	if exp.End != act.End {
		t.Errorf("%s End: expected %d, got %d", prefix, exp.End, act.End)
	}
	if exp.Repeat != act.Repeat {
		t.Errorf("%s Repeat: expected %d, got %d", prefix, exp.Repeat, act.Repeat)
	}
	if exp.LoopDirection != act.LoopDirection {
		t.Errorf("%s LoopDirection: expected %d, got %d", prefix, exp.LoopDirection, act.LoopDirection)
	}

	compareUserData(t, prefix, &exp.UserData, &act.UserData)
}

func compareLayer(t *testing.T, index int, exp, act *Layer) {
	t.Helper()
	prefix := fmt.Sprintf("Layer[%d]('%s')", index, exp.Name)

	if exp.Name != act.Name {
		t.Errorf("%s Name: expected '%s', got '%s'", prefix, exp.Name, act.Name)
	}
	if exp.Type != act.Type {
		t.Errorf("%s Type: expected %d, got %d", prefix, exp.Type, act.Type)
	}
	if exp.Opacity != act.Opacity {
		t.Errorf("%s Opacity: expected %d, got %d", prefix, exp.Opacity, act.Opacity)
	}
	if exp.BlendMode != act.BlendMode {
		t.Errorf("%s BlendMode: expected %d, got %d", prefix, exp.BlendMode, act.BlendMode)
	}
	if exp.ChildLevel != act.ChildLevel {
		t.Errorf("%s ChildLevel: expected %d, got %d", prefix, exp.ChildLevel, act.ChildLevel)
	}
	if exp.TilesetIndex != act.TilesetIndex {
		t.Errorf("%s TilesetIndex: expected %d, got %d", prefix, exp.TilesetIndex, act.TilesetIndex)
	}

	// Layer flags
	if exp.Flags != act.Flags {
		t.Errorf("%s LayerFlags: expected %+v, got %+v", prefix, exp.Flags, act.Flags)
	}

	if exp.IsVisible() != (exp.Flags&1 != 0) {
		t.Errorf("exp.IsVisible() wrong return value")
	}
	if exp.IsLocked() != (exp.Flags&2 == 0) {
		t.Errorf("exp.IsLocked() wrong return value")
	}

	if exp.IsBackgroundLayer() != (exp.Flags&8 != 0) {
		t.Errorf("exp.IsBackgroundLayer() wrong return value")
	}

	if exp.PreferLinkedCels() != (exp.Flags&16 != 0) {
		t.Errorf("exp.PreferLinkedCels() wrong return value")
	}

	if exp.IsGroupCollapsed() != (exp.Flags&32 != 0) {
		t.Errorf("exp.IsGroupCollapsed() wrong return value")
	}
	if exp.IsReferenceLayer() != (exp.Flags&64 != 0) {
		t.Errorf("exp.IsReferenceLayer() wrong return value")
	}

	compareUserData(t, prefix, &exp.UserData, &act.UserData)

	// Cels
	if len(exp.Cels) != len(act.Cels) {
		t.Errorf("%s Cels count: expected %d, got %d", prefix, len(exp.Cels), len(act.Cels))
	} else {
		for i := range exp.Cels {
			compareCel(t, fmt.Sprintf("%s.Cel[%d]", prefix, i), exp.Cels[i], act.Cels[i])
		}
	}
}

func compareCel(t *testing.T, prefix string, exp, act *Cel) {
	t.Helper()

	if (exp == nil) != (act == nil) {
		t.Errorf("%s: expected nil=%v, got nil=%v", prefix, exp == nil, act == nil)
		return
	}

	if exp == nil {
		return
	}

	if exp.LayerIndex != act.LayerIndex {
		t.Errorf("%s LayerIndex: expected %d, got %d", prefix, exp.LayerIndex, act.LayerIndex)
	}
	if exp.Pos != act.Pos {
		t.Errorf("%s Pos: expected %v, got %v", prefix, exp.Pos, act.Pos)
	}
	if exp.Size != act.Size {
		t.Errorf("%s Size: expected %v, got %v", prefix, exp.Size, act.Size)
	}
	if exp.ZIndex != act.ZIndex {
		t.Errorf("%s ZIndex: expected %d, got %d", prefix, exp.ZIndex, act.ZIndex)
	}
	if exp.Opacity != act.Opacity {
		t.Errorf("%s Opacity: expected %d, got %d", prefix, exp.Opacity, act.Opacity)
	}
	if exp.Type != act.Type {
		t.Errorf("%s Type: expected %d, got %d", prefix, exp.Type, act.Type)
	}

	compareUserData(t, prefix, &exp.UserData, &act.UserData)

	// Tiles
	if len(exp.Tiles) != len(act.Tiles) {
		t.Errorf("%s Tiles count: expected %d, got %d", prefix, len(exp.Tiles), len(act.Tiles))
	} else {
		for i := range exp.Tiles {
			if exp.Tiles[i] != act.Tiles[i] {
				t.Errorf("%s Tile[%d]: expected %+v, got %+v", prefix, i, exp.Tiles[i], act.Tiles[i])
			}
		}
	}

	// Image existence check (not comparing content)
	if (exp.Image == nil) != (act.Image == nil) {
		t.Errorf("%s Image: expected nil=%v, got nil=%v", prefix, exp.Image == nil, act.Image == nil)
	}
}

func compareSlice(t *testing.T, index int, exp, act *Slice) {
	t.Helper()
	prefix := fmt.Sprintf("Slice[%d]('%s')", index, exp.Name)

	if exp.Name != act.Name {
		t.Errorf("%s Name: expected '%s', got '%s'", prefix, exp.Name, act.Name)
	}

	compareUserData(t, prefix, &exp.UserData, &act.UserData)

	// Frames
	if len(exp.Frames) != len(act.Frames) {
		t.Errorf("%s Frames count: expected %d, got %d", prefix, len(exp.Frames), len(act.Frames))
	} else {
		for i := range exp.Frames {
			if exp.Frames[i] != act.Frames[i] {
				t.Errorf("%s Frame[%d]: expected %+v, got %+v", prefix, i, exp.Frames[i], act.Frames[i])
			}
		}
	}
}

// Basic Sprite properties
func compareAseProp(t *testing.T, exp, act *Ase) {
	t.Helper()

	if !slices.Equal(exp.Palette, act.Palette) {
		t.Errorf("Palette: expected %d, got %d", exp.Palette, act.Palette)
	}

	if exp.Size != act.Size {
		t.Errorf("Size: expected %v, got %v", exp.Size, act.Size)
	}

	if exp.ColorDepth != act.ColorDepth {
		t.Errorf("ColorDepth: expected %d, got %d", exp.ColorDepth, act.ColorDepth)
	}

	if exp.PixelRatio != act.PixelRatio {
		t.Errorf("PixelRatio: expected %v, got %v", exp.PixelRatio, act.PixelRatio)
	}
	if exp.HasLayersUUID != act.HasLayersUUID {
		t.Errorf("HasUUID: expected %v, got %v", exp.HasLayersUUID, act.HasLayersUUID)
	}

	layer := act.GetLayerByUUID(uuid.UUID{82, 121, 255, 18, 51, 254, 69, 190, 162, 78, 113, 67, 50, 237, 190, 236})
	layerByName := act.GetLayerByName(layer.Name)
	if layer != layerByName {
		t.Errorf("Layer mismatch: GetLayerByUUID and GetLayerByName(%q) returned different instances. UUID layer: %+v, Name layer: %+v",
			layer.Name, layer, layerByName)
	}

	if layer := act.GetLayerByUUID(uuid.UUID{}); layer != nil {
		t.Errorf("GetLayerByUUID should return nil for empty UUID, got %+v", layer)
	}

	if layer := act.GetLayerByName("FakeLayerName999"); layer != nil {
		t.Errorf("GetLayerByName should return nil for non-existent name, got %+v", layer)
	}

	if exp.Layers[0].UUID != layer.UUID {
		t.Errorf("Size: expected %v, got %v", exp.Layers[0].UUID, act.Layers[0].UUID)
	}

	if exp.Layers[0].UserData != act.Layers[0].UserData {
		t.Errorf("Size: expected %v, got %v", exp.Layers[0].UserData, act.Layers[0].UserData)
	}

}
