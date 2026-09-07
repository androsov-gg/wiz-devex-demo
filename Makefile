.PHONY: help deploy destroy doctor status build-image build-cli test vuln-check clean

help:
	@echo "Wiz Developer Experience Demo (Google Cloud Argolis) Deployer"
	@echo ""
	@echo "Available commands:"
	@echo "  make deploy       - Interactively deploy a demo scenario"
	@echo "  make destroy      - Interactively teardown a demo scenario"
	@echo "  make doctor       - Check prerequisites and GCP environment"
	@echo "  make status       - Show active deployment status and URLs"
	@echo "  make build-image  - Build and push container image via Cloud Build"
	@echo "  make build-cli    - Compile deployer binary to bin/deployer"
	@echo "  make test         - Run CLI unit tests"
	@echo "  make vuln-check   - Run Go vulnerability check (govulncheck)"
	@echo "  make clean        - Clean up compiled binaries"
	@echo ""

deploy:
	cd deployer-cli && go run . deploy

destroy:
	cd deployer-cli && go run . destroy

doctor:
	cd deployer-cli && go run . doctor

status:
	cd deployer-cli && go run . status

build-image:
	cd deployer-cli && go run . build

build-cli:
	mkdir -p bin
	cd deployer-cli && go build -o ../bin/deployer .

test:
	cd deployer-cli && go test -v ./...

vuln-check:
	cd deployer-cli && go run golang.org/x/vuln/cmd/govulncheck@latest ./...

clean:
	rm -rf bin/
