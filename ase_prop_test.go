package aseprite

import (
	"image"
	"image/color"
	"testing"

	"github.com/google/uuid"
)

func TestAseProp(t *testing.T) {

	expected := Ase{
		ColorDepth:    8,
		Size:          image.Point{X: 16, Y: 16},
		PixelRatio:    image.Point{1, 2},
		HasLayersUUID: true,

		Layers: []*Layer{
			{
				Name: "Layer 1",
				Type: Image,
				UUID: uuid.UUID{82, 121, 255, 18, 51, 254, 69, 190, 162, 78, 113, 67, 50, 237, 190, 236},
				LayerFlags: LayerFlags{
					Visible: true,
				},
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

// Basic Sprite properties
func compareAseProp(t *testing.T, exp, act *Ase) {
	t.Helper()

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

	if exp.Layers[0].UUID != layer.UUID {
		t.Errorf("Size: expected %v, got %v", exp.Layers[0].UUID, act.Layers[0].UUID)
	}

	if exp.Layers[0].UserData != act.Layers[0].UserData {
		t.Errorf("Size: expected %v, got %v", exp.Layers[0].UserData, act.Layers[0].UserData)
	}

}
