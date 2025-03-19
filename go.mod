module example

go 1.23.0

toolchain go1.23.5

replace github.com/aatomu/aatomlib/disgord => ../aatomlib/disgord

replace github.com/aatomu/aatomlib/utils => ../aatomlib/utils

require (
	github.com/aatomu/aatomlib/utils v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.19.0
)
