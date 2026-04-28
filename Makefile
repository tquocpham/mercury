SERVICES := auth courier entitlements gateway gatewaypriv messages mmservice mmsolver publisher subscriber trade wallet msgworker
BIN_DIR  := bin
MODULES  := pkg $(addprefix cmd/,$(SERVICES))

.PHONY: all $(SERVICES) tidy clean integrationtests

all: $(SERVICES)

$(SERVICES):
	@mkdir -p $(BIN_DIR)
	cd cmd/$@ && go build -o ../../$(BIN_DIR)/$@ .

tidy:
	$(foreach mod,$(MODULES),cd $(CURDIR)/$(mod) && go mod tidy;)

clean:
	rm -rf $(BIN_DIR)

integrationtests:
	cd tests/integration && pip install -r requirements.txt -q && INTEGRATION_CONFIG=$(CONFIG) pytest $(ARGS) -v
