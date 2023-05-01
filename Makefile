BUILD_DIR=./build
all:
	GOOS=js GOARCH=wasm go build -ldflags "-s" -o ${BUILD_DIR}/boxlin.wasm
	GOOS=windows GOARCH=386 CGO_ENABLED=1 CXX=i686-w64-mingw32-g++ CC=i686-w64-mingw32-gcc go build -ldflags "-s -H windowsgui" -v -o ${BUILD_DIR}/boxlin.exe
	go build -ldflags "-s" -v -o ${BUILD_DIR}/boxlin.bin
