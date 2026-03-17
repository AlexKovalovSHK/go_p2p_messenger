go run ./cmd/aether - start

GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -o aether.exe ./cmd/aether - build .exe for windows 

