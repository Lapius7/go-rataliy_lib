module go-ratelimit-example

go 1.26.4

require github.com/Lapius7/go-ratelimit v0.0.0

// This points at the parent directory so the example runs against your
// local checkout. Once you depend on a published version of go-ratelimit
// in your own project, remove this line and run `go get github.com/Lapius7/go-ratelimit`.
replace github.com/Lapius7/go-ratelimit => ../
