LATEST_VERSION=0.13.0
LATEST_BUILD_VERSION=9
echo "LATEST_VERSION=$LATEST_VERSION"
echo "LATEST_BUILD_VERSION=$LATEST_BUILD_VERSION"
echo building darwin-amd64...
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ../dist/mac/emc/edge-matrix -ldflags="-s -w -X 'github.com/emc-protocol/edge-matrix/versioning.Version=$(echo $LATEST_VERSION)' -X 'github.com/emc-protocol/edge-matrix/versioning.Build=$(echo $LATEST_BUILD_VERSION)'"
echo building darwin-arm64...
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o ../dist/mac_arm64/emc/edge-matrix -ldflags="-s -w -X 'github.com/emc-protocol/edge-matrix/versioning.Version=$(echo $LATEST_VERSION)' -X 'github.com/emc-protocol/edge-matrix/versioning.Build=$(echo $LATEST_BUILD_VERSION)'"
echo building linux-amd64...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../dist/linux/emc/edge-matrix -ldflags="-s -w -X 'github.com/emc-protocol/edge-matrix/versioning.Version=$(echo $LATEST_VERSION)' -X 'github.com/emc-protocol/edge-matrix/versioning.Build=$(echo $LATEST_BUILD_VERSION)'"
echo building windows-amd64...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../dist/windows/emc/edge-matrix.exe -ldflags="-s -w -X 'github.com/emc-protocol/edge-matrix/versioning.Version=$(echo $LATEST_VERSION)' -X 'github.com/emc-protocol/edge-matrix/versioning.Build=$(echo $LATEST_BUILD_VERSION)'"
echo completed.
