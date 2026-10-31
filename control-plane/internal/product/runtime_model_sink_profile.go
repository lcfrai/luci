package product

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
)

//go:embed runtime_assets/model_sink_main.go.txt
var runtimeModelSinkSource string

func runtimeModelSinkProfile() string {
	sum := sha256.Sum256([]byte(runtimeModelSinkSource))
	return hex.EncodeToString(sum[:])
}
func runtimeModelSinkDockerfile(builder string) (string, error) {
	if !kubeImmutableImage.MatchString(builder) {
		return "", ErrRuntimePreparation
	}
	// The operator reviews an immutable existing Go 1.25+ builder image. Its
	// advertised version is not assumed proof: an unsupported build fails once.
	return fmt.Sprintf(`FROM %s AS compile
WORKDIR /src
COPY main.go /src/main.go
ENV GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=256MiB
RUN go build -p=1 -trimpath -buildvcs=false -ldflags="-s -w" -o /out/luci-model-sink /src/main.go
FROM scratch
COPY --from=compile /out/luci-model-sink /luci-model-sink
USER 1000:1000
ENTRYPOINT ["/luci-model-sink"]
`, builder), nil
}
