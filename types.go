package aseprite

//go:generate stringer -type=LayerType,BlendMode,ColorDepth,LoopDirection,CelType,FlipBitMask -output=type_string.go

import (
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/google/uuid"
)

const (
	//Bitmask for X flip
	FlipX FlipBitMask = 1 << iota
	// Bitmask for Y flip
	FlipY
	// Bitmask for diagonal flip (swap X/Y axis)
	FlipD
)

const (
	NRGBA ColorDepth = 32
	// Grayscale mode is parsed as image.NRGBA images.
	Grayscale ColorDepth = 16
	Indexed   ColorDepth = 8
)

const (
	Forward LoopDirection = iota
	Reverse
	PingPong
	PingPongReverse
)

const (
	Image LayerType = iota
	Group
	Tilemap
)

const (
	RawUncompressed CelType = iota
	LinkedCel
	CompressedImage
	CompressedTilemap
)

const (
	Normal BlendMode = iota
	Multiply
	Screen
	Overlay
	Darken
	Lighten
	ColorDodge
	ColorBurn
	HardLight
	SoftLight
	Difference
	Exclusion
	Hue
	Saturation
	Color
	Luminosity
	Addition
	Subtract
	Divide
)

type userDataReceiver interface {
	setUserData(text string, c color.NRGBA)
}

type BlendMode uint16
type LayerType uint16
type ColorDepth uint16
type CelType uint16
type LoopDirection uint8
type FlipBitMask uint8

// is Y flip
func (f FlipBitMask) IsFlipX() bool {
	return f&FlipX != 0
}

// is X flip
func (f FlipBitMask) IsFlipY() bool {
	return f&FlipY != 0
}

// is diagonal flip (swap X/Y axis)
func (f FlipBitMask) IsFlipD() bool {
	return f&FlipD != 0
}

// type LayerFlags struct {

// 	// Visible: Layer visibility state.
// 	Visible bool

// 	// Locked: Layer lock state (inverse of Aseprite 'Editable' flag).
// 	Locked bool

// 	// Background: Whether the layer is a background layer.
// 	Background bool

// 	// PreferLinkedCels: Whether linked cels are preferred.
// 	PreferLinkedCels bool

// 	// GroupCollapsed: Whether the group layer is collapsed.
// 	GroupCollapsed bool

//		// Reference: Whether the layer is a reference layer.
//		Reference bool
//	}

// func (l LayerFlags) String() string {
// 	return fmt.Sprintf("Visible: %v\n"+"Locked: %v\n"+
// 		"Background: %v\n"+"PreferLinkedCels: %v\n"+"Collapsed: %v\n"+"Reference: %v",
// 		l.Visible, l.Locked, l.Background,
// 		l.PreferLinkedCels, l.GroupCollapsed, l.Reference)
// }

type Tile struct {
	ID  uint32      // Tile index
	XYD FlipBitMask // XYD bitmask flags
}

// Tileset represents a set of tiles defined in the .ase file (Chunk 0x2023)
type Tileset struct {

	// Tileset name
	Name string

	// Tileset ID
	ID uint32

	// Tile width/height
	TileSize image.Point

	NumTiles int

	// Tileset image containing all tiles vertically (Present if Flags & 2 is set)
	Image image.Image

	// Tileset flags
	//
	// 1 - Include link to external file
	//
	// 2 - Include tiles inside this file
	//
	// 4 - Tilemaps using this tileset use tile ID=0 as empty tile
	//     (this is the new format). In rare cases this bit is off,
	//     and the empty tile will be equal to 0xffffffff (used in
	//     internal versions of Aseprite)
	//
	// 8 - Aseprite will try to match modified tiles with their X
	//     flipped version automatically in Auto mode when using
	//     this tileset.
	//
	// 16 - Same for Y flips
	//
	// 32 - Same for D(iagonal) flips
	Flags uint32

	// External file data (Present if Flags & 1 is set)
	ExternalFileID    uint32
	ExternalTilesetID uint32

	// Index to show in the UI (usually 1)
	BaseIndex int
}

// TileImage returns single tile sub-image
func (ts *Tileset) TileImage(tileID uint32) image.Image {
	if ts.Image == nil || tileID == 0 {
		return nil
	}
	y0 := int(tileID) * ts.TileSize.Y
	y1 := y0 + ts.TileSize.Y

	rect := image.Rect(0, y0, ts.TileSize.X, y1)
	if img, ok := ts.Image.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return img.SubImage(rect)
	}
	return nil
}

type Frame struct {

	// Duration of this frame in the animation
	Duration time.Duration
}

type SliceFrame struct {

	// The bounds of the slice in the canvas
	Bounds image.Rectangle

	// 9-slices internal rectangle (relative to slice bounds)
	Rect9Slices image.Rectangle

	// A pivot to specify the central/base location. (relative to slice bounds)
	Pivot image.Point
}

type Slice struct {
	UserData
	Name   string
	Frames []SliceFrame
}

// A cel is an image in a specific xy-coordinate, and a specific layer/frame combination.
type Cel struct {

	// LayerIndex is the raw index of the layer this cel belongs to.
	LayerIndex int

	// ZIndex defines the display order override.
	// 0 = default, +N = show N layers forward, -N = show N layers back.
	ZIndex int

	// Cel position
	Pos image.Point

	// Cel Width and Cel Height. Pixel unit for Image layers.
	//
	// Number of tiles for Tilemap layers.
	Size image.Point

	// Cel image
	Image image.Image

	// Cel opacity. (0-255)
	Opacity uint8

	// Tiles contains the ID and flip flags for each tile in a tilemap layer.
	// This is nil for regular image layers.
	Tiles []Tile

	// Type specifies if this cel is a Raw Image, Linked Cel, Compressed Image, or Tilemap.
	Type CelType

	// UserData contains extra information like text and color associated with the cel.
	UserData

	// layer is a reference to the layer this cel belongs to,
	layer *Layer
}

// BuildTilemapImage rasterizes the tilemap cel into a composite image.NRGBA.
// It resolves each tile index against the associated tileset, applies bitmask
// transformations for flips (X/Y/D), and performs a draw-call to assemble
// the final pixel buffer into the Cel's Image field.
func (c *Cel) BuildTilemapImage() {
	if c.layer.Type != Tilemap || c.Type != CompressedTilemap || c.layer.tileset == nil {
		return
	}

	tileset := c.layer.tileset
	fullWidth := c.Size.X * tileset.TileSize.X
	fullHeight := c.Size.Y * tileset.TileSize.Y
	rect := image.Rect(0, 0, fullWidth, fullHeight)

	var res draw.Image

	switch ts := tileset.Image.(type) {
	case *image.Paletted:
		res = image.NewPaletted(rect, ts.Palette)
	case *image.NRGBA:
		res = image.NewNRGBA(rect)
	default:
		res = image.NewNRGBA(rect)
	}

	for i, tile := range c.Tiles {
		if tile.ID == 0 {
			continue
		}
		tileImg := tileset.TileImage(tile.ID)
		if tileImg == nil {
			continue
		}

		posX := (i % c.Size.X) * tileset.TileSize.X
		posY := (i / c.Size.X) * tileset.TileSize.Y

		drawTile(res, tileImg, posX, posY, tile.XYD)
	}
	c.Image = res
}

func (c *Cel) setUserData(text string, col color.NRGBA) {
	c.Text = text
	c.Color = col
}

// GetLayer returns layer this cel belongs to
func (c *Cel) GetLayer() *Layer {
	return c.layer
}

type Layer struct {

	// Layer name.
	Name string

	// Layer's universally unique identifier.
	//
	// This field is uuid.Nil (all zeros) if Ase.HasLayersUUID is false.
	UUID uuid.UUID

	// Layer type
	Type LayerType

	// Layer flags
	Flags uint16

	// Blending mode of this layer.
	BlendMode BlendMode

	// Layer opacity. (0-255)
	Opacity uint8

	// Index matches the frame number
	Cels []*Cel

	// The child level is used to show the relationship of this layer with the last one read.
	//
	// https://github.com/aseprite/aseprite/blob/main/docs/ase-file-specs.md#note1
	ChildLevel uint16

	// Index for Ase.Tilesets[]
	TilesetIndex uint32

	UserData

	// tileset is a private reference to Tileset.
	tileset *Tileset

	// parent is a private reference to the main Ase object.
	parent *Ase

	// rawIndex is the original index of the layer in the Aseprite file.
	rawIndex int
}

func (l *Layer) IsVisible() bool {
	return l.Flags&1 != 0
}
func (l *Layer) IsLocked() bool {
	return l.Flags&2 == 0
}
func (l *Layer) IsBackgroundLayer() bool {
	return l.Flags&8 != 0
}
func (l *Layer) PreferLinkedCels() bool {
	return l.Flags&16 != 0
}
func (l *Layer) IsGroupCollapsed() bool {
	return l.Flags&32 != 0
}
func (l *Layer) IsReferenceLayer() bool {
	return l.Flags&64 != 0
}

// GetTileset returns the Tileset used by Tilemap layers.
// It returns nil if layer is not a tilemap.
func (l *Layer) GetTileset() *Tileset {
	return l.tileset
}

// IsCelEmpty checks if the frame has no content.
func (l *Layer) IsCelEmpty(frame int) bool {
	return l.Cels[frame] == nil
}

// Cel returns the cel at the specified frame.
func (l *Layer) Cel(frame int) *Cel {
	return l.Cels[frame]
}

func (l *Layer) setUserData(text string, col color.NRGBA) {
	l.Text = text
	l.Color = col
}

// User-defined data
type UserData struct {
	Color color.NRGBA
	Text  string
}

func (u *UserData) setUserData(text string, c color.NRGBA) {
	u.Text = text
	u.Color = c
}

// Tag Represents a tag in the timeline.
type Tag struct {
	Name string

	// Frame index where this tag starts.
	Start int

	// Frame index where this tag ends.
	End int

	// Play this animation section N times, 0 means infinite.
	Repeat uint16

	LoopDirection LoopDirection

	UserData
}

type ColorProfile struct {
	Type  uint16  // 0: No color profile, 1: Use sRGB, 2: Use embedded ICC profile
	Flags uint16  // Bit 1: Use special fixed gamma
	Gamma float64 // Fixed gamma value converted from 16.16 fixed-point
	ICC   []byte  // Embedded ICC profile data (only populated if Type == 2)
}
