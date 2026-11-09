module github.com/npillmayer/khipu

go 1.25.6

replace github.com/npillmayer/cords => ../cords

replace github.com/npillmayer/uax => ../uax

require (
	github.com/npillmayer/cords v0.1.2-alpha.3
	github.com/npillmayer/schuko v0.2.0-alpha.3.0.20211209143531-2d524c4964ff
	github.com/npillmayer/uax v0.3.0-alpha1
	github.com/thatisuday/commando v1.0.4
	golang.org/x/text v0.32.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jolestar/go-commons-pool v2.0.0+incompatible // indirect
	github.com/thatisuday/clapper v1.0.10 // indirect
)
