SERVICES := auth tradecourier entitlements gateway gatewaypriv messages mmservice mmsolver publisher subscriber trade wallet msgworker inventory portal
BIN_DIR  := bin
MODULES  := pkg $(addprefix cmd/,$(SERVICES))

.PHONY: all $(SERVICES) tidy clean docker-build integrationtests proto

all: $(SERVICES)

$(SERVICES):
	@mkdir -p $(BIN_DIR)
	cd cmd/$@ && go build -o ../../$(BIN_DIR)/$@ .

tidy:
	$(foreach mod,$(MODULES),cd $(CURDIR)/$(mod) && go mod tidy;)

clean:
	rm -rf $(BIN_DIR)
	docker system prune -f
	docker image prune -a -f

docker-build:
	docker compose build --parallel 2

integrationtests:
	cd tests/integration && pip install -r requirements.txt -q && INTEGRATION_CONFIG=$(CONFIG) pytest $(ARGS) -v

proto:
	$(eval export PATH=$(PATH):$(shell go env GOPATH)/bin)
	# Go
	cd cmd/portal && protoc --go_out=pb/ --go_opt=paths=source_relative portal.proto
	# Python (interactive tests)
	cd cmd/portal && protoc --python_out=../../tests/interactive portal.proto
	# C# SDK
	cd cmd/portal && protoc --csharp_out=../../sdk/csharp portal.proto
	# GDScript SDK (requires: go install github.com/Sandmax-1/protoc-gen-gdscript@latest)
	cd cmd/portal && protoc --gdscript_out=../../sdk/gd portal.proto || echo "skipping gdscript: protoc-gen-gdscript not installed"
	# Python (integration tests)
	cd cmd/portal && protoc --python_out=../../tests/integration portal.proto
