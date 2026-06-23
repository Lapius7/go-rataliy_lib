module go-rataliy_lib-example

go 1.26.4

require github.com/Lapius7/go-rataliy_lib v0.0.0

// This points at the parent directory so the example runs against your
// local checkout. Once you depend on a published version of go-rataliy_lib
// in your own project, remove this line and run `go get github.com/Lapius7/go-rataliy_lib`.
replace github.com/Lapius7/go-rataliy_lib => ../
