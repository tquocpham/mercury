SERVICES := gateway notifier publisher query worker
BIN_DIR  := bin
MODULES  := pkg $(addprefix cmd/,$(SERVICES))

.PHONY: all $(SERVICES) tidy clean

all: $(SERVICES)

$(SERVICES):
	@mkdir -p $(BIN_DIR)
	cd cmd/$@ && go build -o ../../$(BIN_DIR)/$@ .

tidy:
	$(foreach mod,$(MODULES),cd $(CURDIR)/$(mod) && go mod tidy;)

clean:
	rm -rf $(BIN_DIR)
