# Protocol Buffers

## Generate code

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
cd <bbgo>/pkg/protobuf
protoc -I=. --go_out=. bbgo.proto
```
