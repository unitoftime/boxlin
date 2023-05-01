BUILD_DIR=./build
wasm:
	GOOS=js GOARCH=wasm go build -o ${BUILD_DIR}/boxlin.wasm ${CLIENT_DIR}
