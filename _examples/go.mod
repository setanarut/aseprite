module github.com/setanarut/aseprite/examples

go 1.25

replace github.com/setanarut/aseprite => ../

require (
	github.com/anthonynsimon/bild v0.14.0
	github.com/setanarut/aseprite v0.0.0
)

require golang.org/x/image v0.18.0 // indirect
