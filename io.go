package aseprite

import (
	"io/fs"
	"os"
)

// Read opens and parses an Aseprite file from the given filepath,
// optionally filtering to visible layers only
func Read(filepath string, onlyVisibleLayers bool) (a Ase, err error) {
	file, err := os.Open(filepath)
	if err != nil {
		return a, err
	}
	defer file.Close()

	_, err = a.parse(file, onlyVisibleLayers)

	if err != nil {
		return a, err
	}

	return a, nil
}

// ReadFs opens and parses an Aseprite file from a virtual filesystem,
// optionally filtering to visible layers only
func ReadFs(f fs.FS, filepath string, onlyVisibleLayers bool) (a Ase, err error) {
	file, err := f.Open(filepath)
	if err != nil {
		return a, err
	}
	defer file.Close()

	_, err = a.parse(file, onlyVisibleLayers)

	if err != nil {
		return a, err
	}

	return a, nil
}
