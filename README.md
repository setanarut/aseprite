[![GoDoc](https://godoc.org/github.com/setanarut/aseprite?status.svg)](https://pkg.go.dev/github.com/setanarut/aseprite)

# aseprite

Aseprite file parser/decoder in Go

```shell
go get github.com/setanarut/aseprite@latest
```

Layers are not flattened, but this can be done with external Go image processing packages. The opacity and blending mode information of each layer and Cel is available.

## Chunk Types

Available chunks.

- [x] Layer Chunk (0x2004)
- [x] Cel Chunk (0x2005)
- [x] Tileset Chunk (0x2023)
- [x] Tags Chunk (0x2018)
- [x] User Data Chunk (0x2020)
- [x] Slice Chunk (0x2022)
- [x] Old palette chunk (0x0004)
- [x] Old palette chunk (0x0011)
- [x] Palette Chunk (0x2019)

The following chunks are not currently supported in this package.

- [ ] Cel Extra Chunk (0x2006)
- [ ] Color Profile Chunk (0x2007)
- [ ] External Files Chunk (0x2008)
- [ ] Mask Chunk (0x2016) DEPRECATED
- [ ] Path Chunk (0x2017)