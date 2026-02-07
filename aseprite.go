package aseprite

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
	"math"
	"time"
)

type userDataReceiver interface {
	setUserData(text string, c color.NRGBA)
}

type Aseprite struct {
	Width                   int // Canvas width
	Height                  int // Canvas height
	Flags                   uint16
	Bpp                     uint16
	TransparentPaletteIndex uint8
	Palette                 color.Palette
	Frames                  []Frame
	Layers                  []Layer
	Slices                  []Slice
	Tags                    []Tag
	makeCelFunc             func(f *Aseprite, bounds image.Rectangle, opacity byte, pix []byte) Cel
}

func (a *Aseprite) ReadFrom(r io.Reader) (filesize int64, err error) {
	var hdr [128]byte
	if n, err := io.ReadFull(r, hdr[:]); err != nil {
		return int64(n), err
	}
	if magic := binary.LittleEndian.Uint16(hdr[4:]); magic != 0xA5E0 {
		return 128, errors.New("invalid magic number")
	}
	if pixw, pixh := hdr[34], hdr[35]; pixw != pixh {
		return 128, errors.New("unsupported pixel ratio")
	}
	fileSize := int64(binary.LittleEndian.Uint32(hdr[:]))
	totalFrames := int(binary.LittleEndian.Uint16(hdr[6:]))
	a.Width = int(binary.LittleEndian.Uint16(hdr[8:]))
	a.Height = int(binary.LittleEndian.Uint16(hdr[10:]))
	a.Bpp = binary.LittleEndian.Uint16(hdr[12:])
	a.Flags = binary.LittleEndian.Uint16(hdr[14:])
	a.TransparentPaletteIndex = hdr[28]
	paletteSize := binary.LittleEndian.Uint16(hdr[32:])
	a.Palette = make(color.Palette, paletteSize)
	a.Frames = make([]Frame, 0, totalFrames)
	switch a.Bpp {
	case 8:
		a.makeCelFunc = makeCelImage8
	case 16:
		a.makeCelFunc = makeCelImage16
	case 32:
		a.makeCelFunc = makeCelImage32
	default:
		return 128, errors.New("invalid color depth")
	}
	for i := range a.Palette {
		a.Palette[i] = color.Black
	}
	a.Palette[a.TransparentPaletteIndex] = color.Transparent
	chunkHeaderBuf := make([]byte, 6)
	frameHeaderBuf := make([]byte, 16)
	currentOffset := int64(128)
	for range totalFrames {
		if _, err := io.ReadFull(r, frameHeaderBuf); err != nil {
			return currentOffset, err
		}
		currentOffset += 16
		if magic := binary.LittleEndian.Uint16(frameHeaderBuf[4:]); magic != 0xF1FA {
			return currentOffset, errors.New("invalid frame magic number")
		}
		oldChunks := binary.LittleEndian.Uint16(frameHeaderBuf[6:])
		durationMS := binary.LittleEndian.Uint16(frameHeaderBuf[8:])
		newChunks := binary.LittleEndian.Uint32(frameHeaderBuf[12:])
		nchunks := int(newChunks)
		if nchunks == 0 {
			nchunks = int(oldChunks)
		}
		currentFrame := Frame{
			Dur: time.Millisecond * time.Duration(durationMS),
		}
		if len(a.Layers) > 0 {
			currentFrame.Cels = make([]Cel, len(a.Layers))
		}
		var lastUserDataTarget userDataReceiver
		for j := 0; j < nchunks; j++ {
			if _, err := io.ReadFull(r, chunkHeaderBuf); err != nil {
				return currentOffset, err
			}
			currentOffset += 6
			chunkSize := binary.LittleEndian.Uint32(chunkHeaderBuf[0:])
			chunkType := binary.LittleEndian.Uint16(chunkHeaderBuf[4:])
			dataLen := int(chunkSize) - 6
			if dataLen < 0 {
				return currentOffset, errors.New("invalid chunk size")
			}
			chunkData := make([]byte, dataLen)
			if _, err := io.ReadFull(r, chunkData); err != nil {
				return currentOffset, err
			}
			currentOffset += int64(dataLen)
			switch chunkType {
			case 0x2004:
				var l Layer
				if err := l.Parse(chunkData); err != nil {
					return currentOffset, err
				}
				a.Layers = append(a.Layers, l)
				lastUserDataTarget = &a.Layers[len(a.Layers)-1]
				if len(currentFrame.Cels) < len(a.Layers) {
					newCels := make([]Cel, len(a.Layers))
					copy(newCels, currentFrame.Cels)
					currentFrame.Cels = newCels
				}
			case 0x2005:
				layerIdx := int(binary.LittleEndian.Uint16(chunkData))
				if layerIdx >= len(currentFrame.Cels) {
					newSize := max(len(a.Layers), layerIdx+1)
					newCels := make([]Cel, newSize)
					copy(newCels, currentFrame.Cels)
					currentFrame.Cels = newCels
				}
				cel, err := a.parseCelChunk0x2005(chunkData, layerIdx)
				if err != nil {
					return currentOffset, err
				}
				if cel != nil {
					currentFrame.Cels[layerIdx] = *cel
					lastUserDataTarget = &currentFrame.Cels[layerIdx]
				}
			case 0x2019:
				a.parsePaletteChunk0x2019(chunkData)
				lastUserDataTarget = nil
			case 0x0004:
				a.parseOldPaletteChunk0x0004(chunkData)
				lastUserDataTarget = nil
			case 0x0011:
				a.parseOldPaletteChunk0x0011(chunkData)
				lastUserDataTarget = nil
			case 0x2018:
				newTags := a.parseTagsChunk0x2018(chunkData)
				startIdx := len(a.Tags)
				a.Tags = append(a.Tags, newTags...)
				if len(a.Tags) > startIdx {
					lastUserDataTarget = &a.Tags[len(a.Tags)-1]
				}
			case 0x2022:
				slice := a.parseSliceChunk0x2022(chunkData, totalFrames)
				a.Slices = append(a.Slices, slice)
				lastUserDataTarget = &a.Slices[len(a.Slices)-1]
			case 0x2020:
				if lastUserDataTarget != nil {
					data, col := a.parseUserDataChunk0x2020(chunkData)
					lastUserDataTarget.setUserData(string(data), col)
				}
			default:
				lastUserDataTarget = nil
			}
		}
		if len(currentFrame.Cels) < len(a.Layers) {
			newCels := make([]Cel, len(a.Layers))
			copy(newCels, currentFrame.Cels)
			currentFrame.Cels = newCels
		}
		a.Frames = append(a.Frames, currentFrame)
	}
	return fileSize, nil
}
func (a *Aseprite) parseSliceChunk0x2022(raw []byte, totalFrames int) Slice {
	var s Slice
	nKeysForSlice := int(binary.LittleEndian.Uint32(raw))
	flags := binary.LittleEndian.Uint32(raw[4:])
	name := parseString(raw[12:])
	raw = raw[14+len(name):]
	s.Name = name
	frameIndices := make([]int, 0, nKeysForSlice)
	for i := 0; len(raw) > 0 && i < nKeysForSlice; i++ {
		frameIdx := int(binary.LittleEndian.Uint32(raw))
		frameIndices = append(frameIndices, frameIdx)
		var key SliceFrame
		x := int32(binary.LittleEndian.Uint32(raw[4:]))
		y := int32(binary.LittleEndian.Uint32(raw[8:]))
		w := binary.LittleEndian.Uint32(raw[12:])
		h := binary.LittleEndian.Uint32(raw[16:])
		raw = raw[20:]
		key.Bounds = image.Rect(int(x), int(y), int(x)+int(w), int(y)+int(h))
		if flags&1 != 0 {
			cx := int32(binary.LittleEndian.Uint32(raw))
			cy := int32(binary.LittleEndian.Uint32(raw[4:]))
			cw := binary.LittleEndian.Uint32(raw[8:])
			ch := binary.LittleEndian.Uint32(raw[12:])
			raw = raw[16:]
			key.Center = image.Rect(int(cx), int(cy), int(cx)+int(cw), int(cy)+int(ch))
		}
		if flags&2 != 0 {
			px := int32(binary.LittleEndian.Uint32(raw))
			py := int32(binary.LittleEndian.Uint32(raw[4:]))
			raw = raw[8:]
			key.Pivot = image.Pt(int(px), int(py))
		}
		s.Frames = append(s.Frames, key)
	}
	expandSliceKey(&s, totalFrames, frameIndices)
	return s
}
func (a *Aseprite) parseTagsChunk0x2018(raw []byte) []Tag {
	ntags := int(binary.LittleEndian.Uint16(raw))
	tags := make([]Tag, ntags)
	ptr := raw[10:]
	for i := range ntags {
		t := &tags[i]
		t.Lo = binary.LittleEndian.Uint16(ptr)
		t.Hi = binary.LittleEndian.Uint16(ptr[2:])
		t.LoopDirection = uint8(ptr[4])
		t.Repeat = binary.LittleEndian.Uint16(ptr[5:])
		nameLen := binary.LittleEndian.Uint16(ptr[17:])
		t.Name = string(ptr[19 : 19+nameLen])
		ptr = ptr[19+nameLen:]
	}
	return tags
}
func (a *Aseprite) parseUserDataChunk0x2020(raw []byte) (data []byte, col color.NRGBA) {
	flags := binary.LittleEndian.Uint32(raw)
	raw = raw[4:]
	if flags&1 != 0 {
		n := binary.LittleEndian.Uint16(raw)
		data, raw = raw[2:2+n], raw[2+n:]
	}
	if flags&2 != 0 {
		col = parseColor(raw)
	}
	return data, col
}
func (a *Aseprite) parsePaletteChunk0x2019(raw []byte) {
	entries := binary.LittleEndian.Uint32(raw[0:])
	lo := binary.LittleEndian.Uint32(raw[4:])
	raw = raw[20:]
	for i := 0; i < int(entries); i++ {
		if len(raw) < 2 {
			break
		}
		flags := binary.LittleEndian.Uint16(raw)
		if len(raw) < 6 {
			break
		}
		idx := int(lo) + i
		if idx < len(a.Palette) {
			a.Palette[idx] = parseColor(raw[2:])
		}
		raw = raw[6:]
		if flags&1 != 0 {
			raw = skipString(raw)
		}
	}
}
func (a *Aseprite) parseOldPaletteChunk0x0011(raw []byte) {
	packets := binary.LittleEndian.Uint16(raw)
	raw = raw[2:]
	currentIndex := 0
	for i := 0; i < int(packets); i++ {
		skip := int(raw[0])
		currentIndex += skip
		n := int(raw[1])
		if n == 0 {
			n = 256
		}
		raw = raw[2:]
		for j := 0; j < n && currentIndex < len(a.Palette); j++ {
			a.Palette[currentIndex] = color.NRGBA{
				R: raw[0] * 4,
				G: raw[1] * 4,
				B: raw[2] * 4,
				A: 255,
			}
			raw = raw[3:]
			currentIndex++
		}
	}
}
func (a *Aseprite) parseCelChunk0x2005(raw []byte, layerIdx int) (*Cel, error) {
	xpos := int(int16(binary.LittleEndian.Uint16(raw[2:])))
	ypos := int(int16(binary.LittleEndian.Uint16(raw[4:])))
	opacity := raw[6]
	celtype := binary.LittleEndian.Uint16(raw[7:])
	if layerIdx >= len(a.Layers) {
		return nil, nil
	}
	layer := &a.Layers[layerIdx]
	if layer.Flags&1 == 0 || layer.Flags&64 != 0 {
		return nil, nil
	}
	raw = raw[16:]
	finalOpacity := byte((int(opacity) * int(layer.Opacity)) / 255)
	var pix []byte
	switch celtype {
	case 0:
		pix = raw[4:]
	case 1:
		srcFrame := int(binary.LittleEndian.Uint16(raw))
		if srcFrame < len(a.Frames) && layerIdx < len(a.Frames[srcFrame].Cels) {
			c := a.Frames[srcFrame].Cels[layerIdx]
			return &c, nil
		}
		return nil, nil
	case 2:
		zr, err := zlib.NewReader(bytes.NewReader(raw[4:]))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		data, err := io.ReadAll(zr)
		if err != nil {
			return nil, err
		}
		pix = data
	default:
		return nil, errors.New("unsupported cel type")
	}
	width := int(binary.LittleEndian.Uint16(raw))
	height := int(binary.LittleEndian.Uint16(raw[2:]))
	bounds := image.Rect(xpos, ypos, xpos+width, ypos+height)
	c := a.makeCelFunc(a, bounds, finalOpacity, pix)
	return &c, nil
}
func (a *Aseprite) parseOldPaletteChunk0x0004(raw []byte) {
	packets := binary.LittleEndian.Uint16(raw)
	raw = raw[2:]
	currentIndex := 0
	for i := 0; i < int(packets); i++ {
		skip := int(raw[0])
		currentIndex += skip
		n := int(raw[1])
		if n == 0 {
			n = 256
		}
		raw = raw[2:]
		for j := 0; j < n && currentIndex < len(a.Palette); j++ {
			a.Palette[currentIndex] = color.NRGBA{
				R: raw[0],
				G: raw[1],
				B: raw[2],
				A: 255,
			}
			raw = raw[3:]
			currentIndex++
		}
	}
}
func (a *Aseprite) buildUserDataText() []byte {
	n := 0
	for _, l := range a.Layers {
		if l.Flags&1 != 0 {
			n += len(l.Text)
		}
	}
	for _, fr := range a.Frames {
		for _, c := range fr.Cels {
			n += len(c.Text)
		}
	}
	return make([]byte, 0, n)
}
func (a *Aseprite) buildLayerUserDataText() [][]byte {
	userdataText := a.buildUserDataText()
	ld := make([][]byte, 0, len(a.Layers))
	for _, l := range a.Layers {
		if l.Flags&1 != 0 && len(l.Text) > 0 {
			ofs := len(userdataText)
			userdataText = append(userdataText, l.Text...)
			ld = append(ld, userdataText[ofs:])
		}
	}
	return ld
}

type Frame struct {
	Dur  time.Duration
	Cels []Cel
}

type SliceFrame struct {
	Bounds image.Rectangle
	Center image.Rectangle
	Pivot  image.Point
}

type Slice struct {
	Name   string
	Frames []SliceFrame
	UserData
}

type Cel struct {
	UserData
	Image image.Image
}

func (c *Cel) setUserData(text string, col color.NRGBA) {
	c.Text = text
	c.Color = col
}

type Layer struct {
	Name      string
	Flags     uint16
	BlendMode uint16
	Opacity   byte
	UserData
}

func (l *Layer) setUserData(text string, col color.NRGBA) {
	l.Text = text
	l.Color = col
}

func (l *Layer) Parse(raw []byte) error {
	if typ := binary.LittleEndian.Uint16(raw[2:]); typ == 2 {
		return errors.New("tilemap layers not supported")
	}
	l.Flags = binary.LittleEndian.Uint16(raw)
	l.BlendMode = binary.LittleEndian.Uint16(raw[10:])
	l.Opacity = raw[12]
	l.Name = string(raw[16:])
	return nil
}

type UserData struct {
	Color color.NRGBA
	Text  string
}

func (u *UserData) setUserData(text string, c color.NRGBA) {
	u.Text = text
	u.Color = c
}

type Tag struct {
	Name          string
	Lo            uint16
	Hi            uint16
	Repeat        uint16
	LoopDirection uint8
	UserData
}

func makeCelImage8(f *Aseprite, bounds image.Rectangle, opacity byte, pix []byte) Cel {
	img := image.Paletted{
		Pix:     pix,
		Stride:  bounds.Dx(),
		Rect:    bounds,
		Palette: f.Palette,
	}
	return Cel{Image: &img}
}
func makeCelImage16(f *Aseprite, bounds image.Rectangle, opacity byte, pix []byte) Cel {
	img := image.NewNRGBA(bounds)
	stride := bounds.Dx() * 2
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			i := (y-bounds.Min.Y)*stride + (x-bounds.Min.X)*2
			grayValue := pix[i]
			alphaValue := pix[i+1]
			finalAlpha := uint16(alphaValue) * uint16(opacity) / 255
			img.SetNRGBA(x, y, color.NRGBA{
				R: grayValue,
				G: grayValue,
				B: grayValue,
				A: byte(finalAlpha),
			})
		}
	}
	return Cel{Image: img}
}
func makeCelImage32(f *Aseprite, bounds image.Rectangle, opacity byte, pix []byte) Cel {
	img := image.NRGBA{
		Pix:    pix,
		Stride: bounds.Dx() * 4,
		Rect:   bounds,
	}
	return Cel{Image: &img}
}

func expandSliceKey(slice *Slice, lenFrames int, frameIndices []int) {
	if len(slice.Frames) == lenFrames {
		return
	}
	expandedKeys := make([]SliceFrame, lenFrames)
	keyIdx := 0
	var current SliceFrame
	if len(slice.Frames) > 0 {
		current = slice.Frames[0]
	}
	for frameIdx := range expandedKeys {
		if keyIdx < len(slice.Frames) && frameIndices[keyIdx] == frameIdx {
			current = slice.Frames[keyIdx]
			keyIdx++
		}
		expandedKeys[frameIdx] = current
	}
	slice.Frames = expandedKeys
}
func skipString(raw []byte) []byte {
	n := binary.LittleEndian.Uint16(raw)
	return raw[2+n:]
}
func parseString(raw []byte) string {
	n := binary.LittleEndian.Uint16(raw)
	return string(raw[2 : 2+n])
}
func parseColor(raw []byte) color.NRGBA {
	return color.NRGBA{
		R: raw[0],
		G: raw[1],
		B: raw[2],
		A: raw[3],
	}
}
func factorPowerOfTwo(n int) (a, b int) {
	x := int(math.Ceil(math.Log2(float64(n))))
	a = 1 << (x - x/2)
	b = 1 << (x / 2)
	return
}
